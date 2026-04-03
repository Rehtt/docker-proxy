package cache

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestCacheKey_differsByAuth(t *testing.T) {
	tr := &transport{ttl: 24 * time.Hour}
	u, _ := url.Parse("https://registry-1.docker.io/v2/library/nginx/blobs/sha256:abc")
	r1 := &http.Request{Method: http.MethodGet, Host: "registry-1.docker.io", URL: u}
	r2 := &http.Request{Method: http.MethodGet, Host: "registry-1.docker.io", URL: u, Header: make(http.Header)}
	r2.Header.Set("Authorization", "Bearer x")
	k1 := tr.cacheKey(r1)
	k2 := tr.cacheKey(r2)
	if k1 == k2 {
		t.Fatal("expected different keys when Authorization differs")
	}
}
