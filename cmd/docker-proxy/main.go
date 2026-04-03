package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Rehtt/docker-proxy/internal/cache"
	"github.com/Rehtt/docker-proxy/internal/config"
	"github.com/Rehtt/docker-proxy/internal/proxy"
)

func main() {
	listen := flag.String("listen", ":8080", "监听地址，例如 :8080 或 0.0.0.0:443")
	configPath := flag.String("config", "config.yaml", "路由配置文件路径")
	certFile := flag.String("cert", "", "TLS 证书（与 -key 同时设置则启用 HTTPS）")
	keyFile := flag.String("key", "", "TLS 私钥")
	cacheDir := flag.String("cache-dir", "", "镜像缓存目录（非空则启用，覆盖配置中的 cache.dir）")
	cacheTTLDays := flag.Int("cache-ttl-days", -1, "缓存保留天数，默认 3；-1 表示使用配置文件")
	logLevel := flag.String("log-level", "", "日志等级，覆盖配置：debug|info|warn|error")
	flag.Parse()

	app, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logCfg := app.Log
	if *logLevel != "" {
		logCfg = config.LogConfig{Level: *logLevel}
	}
	slvl, err := logCfg.SlogLevel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "log level: %v\n", err)
		os.Exit(1)
	}
	var lv slog.LevelVar
	lv.Set(slvl)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &lv})))

	cacheRoot := ""
	var cacheTTL time.Duration
	if app.Cache != nil {
		cacheRoot = app.Cache.Dir
		cacheTTL = app.Cache.TTL()
	}
	if *cacheDir != "" {
		cacheRoot = *cacheDir
		if *cacheTTLDays < 0 {
			cacheTTL = 3 * 24 * time.Hour
		}
	}
	if *cacheTTLDays >= 0 {
		cacheTTL = time.Duration(*cacheTTLDays) * 24 * time.Hour
	}
	if cacheRoot != "" && cacheTTL <= 0 {
		cacheTTL = 3 * 24 * time.Hour
	}

	m := make(map[string]*url.URL)
	for _, rt := range app.Routes {
		u, err := url.Parse(rt.Upstream)
		if err != nil {
			slog.Error("upstream", "upstream", rt.Upstream, "err", err)
			os.Exit(1)
		}
		if _, dup := m[rt.Host]; dup {
			slog.Error("duplicate host", "host", rt.Host)
			os.Exit(1)
		}
		m[rt.Host] = u
		slog.Info("route", "host", rt.Host, "upstream", rt.Upstream)
	}
	if cacheRoot != "" {
		if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
			slog.Error("cache dir", "err", err)
			os.Exit(1)
		}
		slog.Info("cache enabled", "dir", cacheRoot, "ttl", cacheTTL.String())
		cache.RunCleaner(context.Background(), cacheRoot, cacheTTL, time.Hour)
	}

	reg := proxy.NewRegistry(m, cacheRoot, cacheTTL)
	srv := &http.Server{
		Addr:    *listen,
		Handler: reg,
	}

	if *certFile != "" || *keyFile != "" {
		if *certFile == "" || *keyFile == "" {
			slog.Error("TLS", "msg", "-cert and -key must be set together")
			os.Exit(1)
		}
		slog.Info("HTTPS listening", "addr", *listen)
		if err := srv.ListenAndServeTLS(*certFile, *keyFile); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
		return
	}

	slog.Info("HTTP listening", "addr", *listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server", "err", err)
		os.Exit(1)
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
  cache:
    enabled: true
    dir: ./cache
    ttl_days: 3
  log:
    level: info

Docker daemon (/etc/docker/daemon.json), using HTTP mirror on port 8080:
  {
    "insecure-registries": ["hub.example.com:8080", "ghcr.example.com:8080"]
  }
Then: docker pull hub.example.com:8080/library/nginx:latest

For HTTPS on 443 with proper certs, omit insecure-registries and use host:443 as usual.
`)
	}
}
