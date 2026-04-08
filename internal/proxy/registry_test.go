package proxy

import (
	"net/http"
	"testing"
)

func TestParseRedirectProxyPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		ok      bool
		host    string
		rewrote string
	}{
		{
			name:    "valid",
			path:    "/__docker_proxy_redirect__/pkg-containers.githubusercontent.com/ghcr1/blobs/sha256:abc",
			ok:      true,
			host:    "pkg-containers.githubusercontent.com",
			rewrote: "/ghcr1/blobs/sha256:abc",
		},
		{
			name: "missing_host",
			path: "/__docker_proxy_redirect__//ghcr1/blobs/sha256:abc",
			ok:   false,
		},
		{
			name: "invalid_host_no_dot",
			path: "/__docker_proxy_redirect__/localhost/ghcr1/blobs/sha256:abc",
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, p, ok := parseRedirectProxyPath(tt.path)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if host != tt.host {
				t.Fatalf("host = %q, want %q", host, tt.host)
			}
			if p != tt.rewrote {
				t.Fatalf("path = %q, want %q", p, tt.rewrote)
			}
		})
	}
}

func TestRewriteRedirect(t *testing.T) {
	resp := &http.Response{
		Header: make(http.Header),
		Request: &http.Request{
			Header: make(http.Header),
		},
	}
	resp.Header.Set("Location", "https://pkg-containers.githubusercontent.com/ghcr1/blobs/sha256:abc?foo=bar")
	resp.Request.Header.Set(hdrOriginalHost, "ghcr.docker.meproxy.top")
	resp.Request.Header.Set(hdrOriginalScheme, "https")

	if err := rewriteRedirect(resp); err != nil {
		t.Fatalf("rewriteRedirect() error = %v", err)
	}
	got := resp.Header.Get("Location")
	want := "https://ghcr.docker.meproxy.top/__docker_proxy_redirect__/pkg-containers.githubusercontent.com/ghcr1/blobs/sha256:abc?foo=bar"
	if got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}
