package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Dir     string
	TTL     time.Duration
	OnError func(error)
}

type transport struct {
	inner   http.RoundTripper
	dir     string
	ttl     time.Duration
	onError func(error)
}

func NewTransport(inner http.RoundTripper, opt Options) http.RoundTripper {
	if strings.TrimSpace(opt.Dir) == "" || inner == nil {
		return inner
	}
	if opt.TTL <= 0 {
		opt.TTL = 3 * 24 * time.Hour
	}
	return &transport{
		inner:   inner,
		dir:     filepath.Clean(opt.Dir),
		ttl:     opt.TTL,
		onError: opt.OnError,
	}
}

func (t *transport) err(err error) {
	if t.onError != nil && err != nil {
		t.onError(err)
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.wantCache(req) {
		return t.inner.RoundTrip(req)
	}
	key := t.cacheKey(req)
	if key == "" {
		return t.inner.RoundTrip(req)
	}
	if resp, ok := t.readDisk(key, req); ok {
		return resp, nil
	}
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if !t.shouldStore(resp, req) {
		return resp, nil
	}

	objDir := t.objectDir(key)
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.err(err)
		return resp, nil
	}

	orig := resp.Body
	if req.Method == http.MethodHead {
		resp.Body = &headReadCloser{
			ReadCloser: orig,
			objDir:     objDir,
			t:          t,
			resp:       resp,
			req:        req,
		}
		return resp, nil
	}

	tmp, err := os.CreateTemp(objDir, "part-*")
	if err != nil {
		t.err(err)
		return resp, nil
	}
	tmpPath := tmp.Name()
	tee := io.TeeReader(orig, tmp)
	resp.Body = &teeReadCloser{
		Reader:   tee,
		upstream: orig,
		file:     tmp,
		tmpPath:  tmpPath,
		objDir:   objDir,
		t:        t,
		resp:     resp,
		req:      req,
	}
	return resp, nil
}

type headReadCloser struct {
	io.ReadCloser
	objDir string
	t      *transport
	resp   *http.Response
	req    *http.Request
}

func (h *headReadCloser) Close() error {
	if err := h.ReadCloser.Close(); err != nil {
		return err
	}
	if err := h.t.writeMeta(h.objDir, h.resp, h.req); err != nil {
		h.t.err(err)
	}
	return nil
}

type teeReadCloser struct {
	io.Reader
	upstream io.ReadCloser
	file     *os.File
	tmpPath  string
	objDir   string
	t        *transport
	resp     *http.Response
	req      *http.Request
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	return t.Reader.Read(p)
}

func (t *teeReadCloser) Close() error {
	errUp := t.upstream.Close()
	errF := t.file.Close()
	if errUp != nil {
		_ = os.Remove(t.tmpPath)
		return errUp
	}
	if errF != nil {
		_ = os.Remove(t.tmpPath)
		return errF
	}
	bodyPath := filepath.Join(t.objDir, "body")
	if err := os.Rename(t.tmpPath, bodyPath); err != nil {
		_ = os.Remove(t.tmpPath)
		return err
	}
	if err := t.t.writeMeta(t.objDir, t.resp, t.req); err != nil {
		t.t.err(err)
	}
	return nil
}

func (t *transport) writeMeta(objDir string, resp *http.Response, req *http.Request) error {
	meta := storedMeta{
		Status:     resp.StatusCode,
		Proto:      resp.Proto,
		ProtoMajor: resp.ProtoMajor,
		ProtoMinor: resp.ProtoMinor,
		Header:     resp.Header.Clone(),
		URL:        req.URL.String(),
		Method:     req.Method,
	}
	b, err := json.Marshal(&meta)
	if err != nil {
		return err
	}
	metaPath := filepath.Join(objDir, "meta")
	if err := os.WriteFile(metaPath, b, 0o644); err != nil {
		return err
	}
	now := time.Now()
	_ = os.Chtimes(metaPath, now, now)
	bodyPath := filepath.Join(objDir, "body")
	if st, err := os.Stat(bodyPath); err == nil && !st.IsDir() {
		_ = os.Chtimes(bodyPath, now, now)
	}
	return nil
}

func (t *transport) wantCache(req *http.Request) bool {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	if req.Header.Get("Range") != "" {
		return false
	}
	p := req.URL.Path
	if !strings.HasPrefix(p, "/v2/") {
		return false
	}
	return strings.Contains(p, "/blobs/") || strings.Contains(p, "/manifests/")
}

func (t *transport) cacheKey(req *http.Request) string {
	u := req.URL
	if u == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(req.Method)
	b.WriteByte(0)
	b.WriteString(req.Host)
	b.WriteByte(0)
	b.WriteString(u.Path)
	b.WriteByte(0)
	b.WriteString(canonicalQuery(u))
	b.WriteByte(0)
	if a := req.Header.Get("Authorization"); a != "" {
		sum := sha256.Sum256([]byte(a))
		b.Write(sum[:])
	}
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

func canonicalQuery(u *url.URL) string {
	return u.Query().Encode()
}

func (t *transport) shouldStore(resp *http.Response, req *http.Request) bool {
	if resp.StatusCode != http.StatusOK {
		return false
	}
	ce := resp.Header.Get("Content-Encoding")
	if ce != "" && ce != "identity" {
		return false
	}
	return t.wantCache(req)
}

type storedMeta struct {
	Status     int         `json:"status"`
	Proto      string      `json:"proto"`
	ProtoMajor int         `json:"proto_major"`
	ProtoMinor int         `json:"proto_minor"`
	Header     http.Header `json:"header"`
	URL        string      `json:"url"`
	Method     string      `json:"method"`
}

func (t *transport) readDisk(key string, req *http.Request) (*http.Response, bool) {
	dir := t.objectDir(key)
	metaPath := filepath.Join(dir, "meta")
	st, err := os.Stat(metaPath)
	if err != nil || st.IsDir() {
		return nil, false
	}
	if time.Since(st.ModTime()) > t.ttl {
		_ = os.RemoveAll(dir)
		return nil, false
	}
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, false
	}
	var meta storedMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, false
	}
	if meta.Method != req.Method {
		return nil, false
	}
	var body io.ReadCloser
	if req.Method == http.MethodHead {
		body = io.NopCloser(bytes.NewReader(nil))
	} else {
		bodyPath := filepath.Join(dir, "body")
		f, err := os.Open(bodyPath)
		if err != nil {
			return nil, false
		}
		body = f
	}
	out := &http.Response{
		StatusCode: meta.Status,
		Status:     fmt.Sprintf("%d %s", meta.Status, http.StatusText(meta.Status)),
		Proto:      meta.Proto,
		ProtoMajor: meta.ProtoMajor,
		ProtoMinor: meta.ProtoMinor,
		Header:     meta.Header.Clone(),
		Body:       body,
	}
	if req.Method == http.MethodGet {
		bodyPath := filepath.Join(dir, "body")
		if st, err := os.Stat(bodyPath); err == nil {
			out.ContentLength = st.Size()
			out.Header.Set("Content-Length", strconv.FormatInt(st.Size(), 10))
		}
	}
	return out, true
}

func (t *transport) objectDir(key string) string {
	if len(key) < 4 {
		return filepath.Join(t.dir, key)
	}
	return filepath.Join(t.dir, key[:2], key[2:])
}
