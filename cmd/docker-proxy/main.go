package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/Rehtt/docker-proxy/internal/config"
	"github.com/Rehtt/docker-proxy/internal/proxy"
)

func main() {
	listen := flag.String("listen", ":8080", "监听地址，例如 :8080 或 0.0.0.0:443")
	configPath := flag.String("config", "config.yaml", "路由配置文件路径")
	certFile := flag.String("cert", "", "TLS 证书（与 -key 同时设置则启用 HTTPS）")
	keyFile := flag.String("key", "", "TLS 私钥")
	flag.Parse()

	routes, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	m := make(map[string]*url.URL)
	for _, rt := range routes {
		u, err := url.Parse(rt.Upstream)
		if err != nil {
			log.Fatalf("upstream %q: %v", rt.Upstream, err)
		}
		if _, dup := m[rt.Host]; dup {
			log.Fatalf("duplicate host: %s", rt.Host)
		}
		m[rt.Host] = u
		log.Printf("route %s -> %s", rt.Host, rt.Upstream)
	}

	reg := proxy.NewRegistry(m)
	srv := &http.Server{
		Addr:    *listen,
		Handler: reg,
	}

	if *certFile != "" || *keyFile != "" {
		if *certFile == "" || *keyFile == "" {
			log.Fatal("-cert and -key must be set together")
		}
		log.Printf("HTTPS listening on %s", *listen)
		if err := srv.ListenAndServeTLS(*certFile, *keyFile); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		return
	}

	log.Printf("HTTP listening on %s", *listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func init() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(w, "Registry reverse proxy for docker pull. Map Host header to upstream registry.\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(w, `
Example config.yaml:
  routes:
    - host: hub.example.com
      upstream: https://registry-1.docker.io
    - host: ghcr.example.com
      upstream: https://ghcr.io

Docker daemon (/etc/docker/daemon.json), using HTTP mirror on port 8080:
  {
    "insecure-registries": ["hub.example.com:8080", "ghcr.example.com:8080"]
  }
Then: docker pull hub.example.com:8080/library/nginx:latest

For HTTPS on 443 with proper certs, omit insecure-registries and use host:443 as usual.
`)
	}
}
