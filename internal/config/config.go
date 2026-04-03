package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Route struct {
	Host     string `yaml:"host"`
	Upstream string `yaml:"upstream"`
}

type Cache struct {
	Enabled *bool  `yaml:"enabled"`
	Dir     string `yaml:"dir"`
	TTLDays int    `yaml:"ttl_days"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type AppConfig struct {
	Routes []Route
	Cache  *Cache
	Log    LogConfig
}

type fileConfig struct {
	Routes []Route    `yaml:"routes"`
	Cache  *Cache     `yaml:"cache"`
	Log    *LogConfig `yaml:"log"`
}

func Load(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	if len(fc.Routes) == 0 {
		return nil, fmt.Errorf("no routes defined")
	}
	for i := range fc.Routes {
		r := &fc.Routes[i]
		r.Host = strings.ToLower(strings.TrimSpace(r.Host))
		if r.Host == "" {
			return nil, fmt.Errorf("route[%d]: empty host", i)
		}
		u, err := url.Parse(r.Upstream)
		if err != nil {
			return nil, fmt.Errorf("route[%d] upstream: %w", i, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("route[%d]: upstream must be http or https", i)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("route[%d]: upstream missing host", i)
		}
		u.Path = strings.TrimSuffix(u.Path, "/")
		u.RawQuery = ""
		u.Fragment = ""
		r.Upstream = u.String()
	}

	var cache *Cache
	if fc.Cache != nil {
		c := fc.Cache
		c.Dir = strings.TrimSpace(c.Dir)
		if c.Enabled != nil && !*c.Enabled {
			cache = nil
		} else if c.Dir == "" {
			if c.Enabled != nil && *c.Enabled {
				return nil, fmt.Errorf("cache: dir is required when enabled is true")
			}
			cache = nil
		} else {
			if c.TTLDays <= 0 {
				c.TTLDays = 3
			}
			cache = c
		}
	}

	logCfg := LogConfig{Level: "info"}
	if fc.Log != nil && strings.TrimSpace(fc.Log.Level) != "" {
		logCfg.Level = strings.TrimSpace(fc.Log.Level)
	}
	if _, err := logCfg.SlogLevel(); err != nil {
		return nil, err
	}

	return &AppConfig{
		Routes: fc.Routes,
		Cache:  cache,
		Log:    logCfg,
	}, nil
}

func (l LogConfig) SlogLevel() (slog.Level, error) {
	s := strings.ToLower(strings.TrimSpace(l.Level))
	if s == "" {
		s = "info"
	}
	if s == "warning" {
		s = "warn"
	}
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log: invalid level %q (want debug, info, warn, error)", l.Level)
	}
}

func (c *Cache) TTL() time.Duration {
	if c == nil {
		return 0
	}
	d := c.TTLDays
	if d <= 0 {
		d = 3
	}
	return time.Duration(d) * 24 * time.Hour
}
