package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	MaxUploadBytes     int64   `yaml:"max_upload_bytes"`
	RateLimitPerSecond float64 `yaml:"rate_limit_per_second"`
	RateLimitBurst     int     `yaml:"rate_limit_burst"`
}

func loadConfig() config {
	cfg := config{MaxUploadBytes: 200 << 20, RateLimitPerSecond: 1, RateLimitBurst: 20}

	path := env("CONFIG_PATH", "config.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config %s: %v", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("parse config %s: %v", path, err)
	}
	if cfg.MaxUploadBytes <= 0 || cfg.RateLimitPerSecond <= 0 || cfg.RateLimitBurst < 1 {
		log.Fatal("max_upload_bytes, rate_limit_per_second and rate_limit_burst must be positive")
	}
	return cfg
}
