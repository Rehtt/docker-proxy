package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Rehtt/docker-proxy/internal/cache"
)

type Registry struct {
	mu         sync.RWMutex
	routes     map[string]*url.URL
	proxyCache map[string]*httputil.ReverseProxy
	cacheDir   string
	cacheTTL   time.Duration
}

const (
	redirectProxyPrefix      = "/__docker_proxy_redirect__/"
	hdrOriginalHost          = "X-Docker-Proxy-Original-Host"
	hdrOriginalScheme        = "X-Docker-Proxy-Original-Scheme"
	hdrOriginalForwardedProto = "X-Forwarded-Proto"
)

func NewRegistry(routes map[string]*url.URL, cacheDir string, cacheTTL time.Duration) *Registry {
	return &Registry{
		routes:     routes,
		proxyCache: make(map[string]*httputil.ReverseProxy),
		cacheDir:   cacheDir,
		cacheTTL:   cacheTTL,
	}
}

func (r *Registry) SetRoutes(routes map[string]*url.URL) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = routes
	r.proxyCache = make(map[string]*httputil.ReverseProxy)
}

func (r *Registry) SetCache(dir string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheDir = dir
	r.cacheTTL = ttl
	r.proxyCache = make(map[string]*httputil.ReverseProxy)
}

func (r *Registry) upstreamForHost(host string) (*url.URL, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.routes[host]
	return u, ok
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	up, ok := r.upstreamForHost(req.Host)
	if !ok {
		http.Error(w, "unknown registry host", http.StatusBadGateway)
		return
	}
	if proxyHost, proxyPath, ok := parseRedirectProxyPath(req.URL.Path); ok {
		up = &url.URL{
			Scheme: "https",
			Host:   proxyHost,
		}
		req.URL.Path = proxyPath
	}

	proxy := r.reverseProxyFor(up)
	start := time.Now()
	rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
	proxy.ServeHTTP(rec, req)

	if want, lvl := pullAccessLog(req.URL.Path, req.Method); want {
		ctx := req.Context()
		if slog.Default().Enabled(ctx, lvl) {
			client := ClientIP(req)
			args := []any{
				"method", req.Method,
				"path", req.URL.Path,
				"host", req.Host,
				"remote", client,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"upstream_host", up.Host,
			}
			if peer := remoteTCPHost(req.RemoteAddr); peer != client {
				args = append(args, "peer", req.RemoteAddr)
			}
			slog.Log(ctx, lvl, "registry_request", args...)
		}
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	if rr.written {
		return
	}
	rr.status = code
	rr.written = true
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if !rr.written {
		rr.status = http.StatusOK
		rr.written = true
	}
	return rr.ResponseWriter.Write(b)
}

func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rr *responseRecorder) Unwrap() http.ResponseWriter {
	return rr.ResponseWriter
}

func pullAccessLog(path, method string) (bool, slog.Level) {
	if !strings.HasPrefix(path, "/v2/") {
		return false, 0
	}
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
	default:
		return false, 0
	}
	if strings.Contains(path, "/manifests/") {
		return true, slog.LevelInfo
	}
	if strings.Contains(path, "/blobs/") {
		return true, slog.LevelDebug
	}
	return true, slog.LevelDebug
}

func (r *Registry) reverseProxyFor(target *url.URL) *httputil.ReverseProxy {
	key := target.String()
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.proxyCache[key]; ok {
		return p
	}
	p := r.newSingleHostReverseProxy(target)
	r.proxyCache[key] = p
	return p
}

func filepathJoinPerHost(root, host string) string {
	h := strings.ReplaceAll(host, ":", "_")
	return filepath.Join(root, h)
}

func (r *Registry) newSingleHostReverseProxy(target *url.URL) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		originHost := req.Host
		originScheme := originalScheme(req)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.Header.Set(hdrOriginalHost, originHost)
		req.Header.Set(hdrOriginalScheme, originScheme)
		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "")
		}
	}
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2: true,
	}
	var tr http.RoundTripper = base
	if r.cacheDir != "" && r.cacheTTL > 0 {
		tr = cache.NewTransport(base, cache.Options{
			Dir: filepathJoinPerHost(r.cacheDir, target.Host),
			TTL: r.cacheTTL,
			OnError: func(e error) {
				slog.Warn("cache", "err", e)
			},
		})
	}
	return &httputil.ReverseProxy{
		Director:       director,
		Transport:      tr,
		FlushInterval:  -1,
		ModifyResponse: rewriteRedirect,
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadGateway)
		},
	}
}

func rewriteRedirect(resp *http.Response) error {
	loc := strings.TrimSpace(resp.Header.Get("Location"))
	if loc == "" {
		return nil
	}
	u, err := url.Parse(loc)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil
	}

	originHost := strings.TrimSpace(resp.Request.Header.Get(hdrOriginalHost))
	if originHost == "" {
		return nil
	}
	originScheme := strings.TrimSpace(resp.Request.Header.Get(hdrOriginalScheme))
	if originScheme == "" {
		originScheme = "https"
	}

	escapedPath := strings.TrimPrefix(u.EscapedPath(), "/")
	rewritten := &url.URL{
		Scheme:   originScheme,
		Host:     originHost,
		Path:     redirectProxyPrefix + u.Host + "/" + escapedPath,
		RawQuery: u.RawQuery,
	}
	resp.Header.Set("Location", rewritten.String())
	return nil
}

func parseRedirectProxyPath(path string) (upstreamHost string, rewrittenPath string, ok bool) {
	if !strings.HasPrefix(path, redirectProxyPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, redirectProxyPrefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", "", false
	}
	host := rest[:slash]
	if !isValidRedirectHost(host) {
		return "", "", false
	}
	tail := rest[slash:]
	if tail == "" {
		tail = "/"
	}
	return host, tail, true
}

func isValidRedirectHost(host string) bool {
	if host == "" || strings.Contains(host, "/") || strings.Contains(host, "://") {
		return false
	}
	if h, p, err := net.SplitHostPort(host); err == nil {
		if h == "" || p == "" {
			return false
		}
		return true
	}
	return strings.Contains(host, ".")
}

func originalScheme(req *http.Request) string {
	if xfp := strings.TrimSpace(req.Header.Get(hdrOriginalForwardedProto)); xfp != "" {
		parts := strings.Split(xfp, ",")
		first := strings.TrimSpace(parts[0])
		if first != "" {
			return first
		}
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}
