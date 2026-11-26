package main

import (
	"errors"
	"fmt"
	"time"

	"stashr/shared/appconfig"
)

// maxUploadChunkBytes keeps one stream message safely under gRPC's default
// 4 MB receive limit.
const maxUploadChunkBytes = 3 << 20

// gatewayConfig holds the config.yaml keys only the gateway uses; the limits it
// shares with the services are in appconfig.Limits.
type gatewayConfig struct {
	RateLimitPerSecond       float64       `yaml:"rate_limit_per_second"`
	RateLimitBurst           int           `yaml:"rate_limit_burst"`
	RateLimitCleanupInterval time.Duration `yaml:"rate_limit_cleanup_interval"`
	RateLimitVisitorTTL      time.Duration `yaml:"rate_limit_visitor_ttl"`

	RequestTimeout    time.Duration `yaml:"gateway_request_timeout"`
	ReadHeaderTimeout time.Duration `yaml:"gateway_read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"gateway_read_timeout"`
	WriteTimeout      time.Duration `yaml:"gateway_write_timeout"`
	IdleTimeout       time.Duration `yaml:"gateway_idle_timeout"`
	MaxJSONBodyBytes  int64         `yaml:"max_json_body_bytes"`
	ShutdownTimeout   time.Duration `yaml:"gateway_shutdown_timeout"`
	// Origin browsers may call the gateway from directly; "*" allows any.
	CORSAllowedOrigin string `yaml:"cors_allowed_origin"`

	Upload uploadSettings `yaml:",inline"`
}

type uploadSettings struct {
	// A big upload over a slow link may legitimately take a while; this replaces
	// the server-wide read/write timeouts for the upload routes.
	Timeout    time.Duration `yaml:"upload_timeout"`
	ChunkBytes int           `yaml:"upload_chunk_bytes"`
	// Parts bigger than this are spooled to temporary files instead of RAM.
	MultipartMemoryBytes int64 `yaml:"upload_multipart_memory_bytes"`
	// Room for the multipart framing and the folderId field on top of the file itself.
	MultipartOverheadBytes int64 `yaml:"upload_multipart_overhead_bytes"`
}

func defaultUploadSettings() uploadSettings {
	return uploadSettings{
		Timeout:                10 * time.Minute,
		ChunkBytes:             64 << 10,
		MultipartMemoryBytes:   8 << 20,
		MultipartOverheadBytes: 1 << 20,
	}
}

func defaultGatewayConfig() gatewayConfig {
	return gatewayConfig{
		RateLimitPerSecond:       1,
		RateLimitBurst:           20,
		RateLimitCleanupInterval: time.Minute,
		RateLimitVisitorTTL:      3 * time.Minute,
		RequestTimeout:           15 * time.Second,
		ReadHeaderTimeout:        10 * time.Second,
		ReadTimeout:              30 * time.Second,
		WriteTimeout:             30 * time.Second,
		IdleTimeout:              120 * time.Second,
		MaxJSONBodyBytes:         1 << 20,
		ShutdownTimeout:          10 * time.Second,
		CORSAllowedOrigin:        "*",
		Upload:                  defaultUploadSettings(),
	}
}

// loadConfig reads and validates config.yaml.
func loadConfig(path string) (appconfig.Limits, gatewayConfig, error) {
	limits := appconfig.DefaultLimits()
	cfg := defaultGatewayConfig()
	if err := appconfig.Load(path, &limits, &cfg); err != nil {
		return limits, cfg, err
	}
	return limits, cfg, cfg.validate()
}

func (c gatewayConfig) validate() error {
	if !(c.RateLimitPerSecond > 0) {
		return errors.New("rate_limit_per_second must be a positive number")
	}
	if c.RateLimitBurst < 1 {
		return errors.New("rate_limit_burst must be a positive integer")
	}
	if err := appconfig.PositiveDurations(map[string]time.Duration{
		"rate_limit_cleanup_interval": c.RateLimitCleanupInterval,
		"rate_limit_visitor_ttl":      c.RateLimitVisitorTTL,
		"gateway_request_timeout":     c.RequestTimeout,
		"gateway_read_header_timeout": c.ReadHeaderTimeout,
		"gateway_read_timeout":        c.ReadTimeout,
		"gateway_write_timeout":       c.WriteTimeout,
		"gateway_idle_timeout":        c.IdleTimeout,
		"gateway_shutdown_timeout":    c.ShutdownTimeout,
		"upload_timeout":              c.Upload.Timeout,
	}); err != nil {
		return err
	}
	if c.CORSAllowedOrigin == "" {
		return errors.New("cors_allowed_origin must not be empty")
	}
	if c.MaxJSONBodyBytes <= 0 {
		return errors.New("max_json_body_bytes must be a positive number")
	}
	if c.Upload.ChunkBytes <= 0 || c.Upload.ChunkBytes > maxUploadChunkBytes {
		return fmt.Errorf("upload_chunk_bytes must be between 1 and %d", maxUploadChunkBytes)
	}
	if c.Upload.MultipartMemoryBytes <= 0 || c.Upload.MultipartOverheadBytes <= 0 {
		return errors.New("upload_multipart_memory_bytes and upload_multipart_overhead_bytes must be positive")
	}
	return nil
}
