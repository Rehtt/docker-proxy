package config

import (
	"fmt"
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

type fileConfig struct {
	Routes []Route `yaml:"routes"`
	Cache  *Cache  `yaml:"cache"`
}

func Load(path string) ([]Route, *Cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, nil, err
	}
	if len(fc.Routes) == 0 {
		return nil, nil, fmt.Errorf("no routes defined")
	}
	for i := range fc.Routes {
		r := &fc.Routes[i]
		r.Host = strings.ToLower(strings.TrimSpace(r.Host))
		if r.Host == "" {
			return nil, nil, fmt.Errorf("route[%d]: empty host", i)
		}
		u, err := url.Parse(r.Upstream)
		if err != nil {
			return nil, nil, fmt.Errorf("route[%d] upstream: %w", i, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, nil, fmt.Errorf("route[%d]: upstream must be http or https", i)
		}
		if u.Host == "" {
			return nil, nil, fmt.Errorf("route[%d]: upstream missing host", i)
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
			return fc.Routes, nil, nil
		}
		if c.Dir == "" {
			if c.Enabled != nil && *c.Enabled {
				return nil, nil, fmt.Errorf("cache: dir is required when enabled is true")
			}
			return fc.Routes, nil, nil
		}
		if c.TTLDays <= 0 {
			c.TTLDays = 3
		}
		cache = c
	}
	return fc.Routes, cache, nil
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
