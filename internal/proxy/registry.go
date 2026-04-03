package proxy

import (
	"crypto/tls"
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

	proxy := r.reverseProxyFor(up)
	proxy.ServeHTTP(w, req)
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
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
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
	return nil
}
