package proxy

import (
	"net/http"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "remote_only",
			req:  &http.Request{RemoteAddr: "198.51.100.1:12345"},
			want: "198.51.100.1",
		},
		{
			name: "x_real_ip",
			req: func() *http.Request {
				r := &http.Request{RemoteAddr: "10.0.0.1:8080", Header: make(http.Header)}
				r.Header.Set("X-Real-IP", "203.0.113.7")
				return r
			}(),
			want: "203.0.113.7",
		},
		{
			name: "xff_chain",
			req: func() *http.Request {
				r := &http.Request{RemoteAddr: "10.0.0.1:8080", Header: make(http.Header)}
				r.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
				return r
			}(),
			want: "203.0.113.5",
		},
		{
			name: "real_ip_precedes_xff",
			req: func() *http.Request {
				r := &http.Request{RemoteAddr: "10.0.0.1:8080", Header: make(http.Header)}
				r.Header.Set("X-Real-IP", "203.0.113.2")
				r.Header.Set("X-Forwarded-For", "198.51.100.9")
				return r
			}(),
			want: "203.0.113.2",
		},
		{
			name: "forwarded_rfc7239",
			req: func() *http.Request {
				r := &http.Request{RemoteAddr: "10.0.0.1:8080", Header: make(http.Header)}
				r.Header.Set("Forwarded", `for=192.0.2.60;proto=http;by=203.0.113.43`)
				return r
			}(),
			want: "192.0.2.60",
		},
		{
			name: "forwarded_ipv6",
			req: func() *http.Request {
				r := &http.Request{RemoteAddr: "10.0.0.1:8080", Header: make(http.Header)}
				r.Header.Set("Forwarded", `for="[2001:db8:cafe::17]:4711"`)
				return r
			}(),
			want: "2001:db8:cafe::17",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClientIP(tt.req); got != tt.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
