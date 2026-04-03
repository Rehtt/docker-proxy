package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Route struct {
	Host     string `yaml:"host"`
	Upstream string `yaml:"upstream"`
}

type fileConfig struct {
	Routes []Route `yaml:"routes"`
}

func Load(path string) ([]Route, error) {
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
	return fc.Routes, nil
}
