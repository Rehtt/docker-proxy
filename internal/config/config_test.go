package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_cacheEnabledFalse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	content := `
routes:
  - host: a.example.com
    upstream: https://registry-1.docker.io
cache:
  enabled: false
  dir: ./cache
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if app.Cache != nil {
		t.Fatal("expected cache disabled")
	}
}

func TestLoad_cacheImplicitEnabledWithDir(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	content := `
routes:
  - host: a.example.com
    upstream: https://registry-1.docker.io
cache:
  dir: ./data
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if app.Cache == nil || app.Cache.Dir != "./data" {
		t.Fatalf("cache: %+v", app.Cache)
	}
}

func TestLoad_invalidLogLevel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	content := `
routes:
  - host: a.example.com
    upstream: https://registry-1.docker.io
log:
  level: noisy
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
