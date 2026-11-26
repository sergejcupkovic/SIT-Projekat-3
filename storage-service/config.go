package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	MaxUploadBytes int64 `yaml:"max_upload_bytes"`
}

func loadConfig() config {
	cfg := config{MaxUploadBytes: 200 << 20}

	path := envOr("CONFIG_PATH", "config.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config %s: %v", path, err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("parse config %s: %v", path, err)
	}
	if cfg.MaxUploadBytes <= 0 {
		log.Fatal("max_upload_bytes must be positive")
	}
	return cfg
}
