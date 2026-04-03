package proxy

import (
	"net"
	"net/http"
	"strings"
)

func ClientIP(r *http.Request) string {
	if s := strings.TrimSpace(r.Header.Get("X-Real-IP")); s != "" {
		return hostOnly(s)
	}
	if s := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); s != "" {
		for part := range strings.SplitSeq(s, ",") {
			ip := strings.TrimSpace(part)
			if ip != "" {
				return hostOnly(ip)
			}
		}
	}
	if s := strings.TrimSpace(r.Header.Get("Forwarded")); s != "" {
		if ip := forwardedForValue(s); ip != "" {
			return ip
		}
	}
	return remoteTCPHost(r.RemoteAddr)
}

func remoteTCPHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func hostOnly(s string) string {
	s = strings.Trim(s, `"`)
	if strings.HasPrefix(s, "[") {
		if host, _, err := net.SplitHostPort(s); err == nil {
			return host
		}
		if i := strings.IndexByte(s, ']'); i > 0 {
			return strings.TrimPrefix(s[:i], "[")
		}
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		return host
	}
	return s
}

func forwardedForValue(s string) string {
	for p := range strings.SplitSeq(s, ",") {
		p = strings.TrimSpace(p)
		for kv := range strings.SplitSeq(p, ";") {
			kv = strings.TrimSpace(kv)
			low := strings.ToLower(kv)
			if !strings.HasPrefix(low, "for=") {
				continue
			}
			v := strings.TrimSpace(kv[len("for="):])
			v = strings.Trim(v, `"`)
			return hostOnly(v)
		}
	}
	return ""
}
