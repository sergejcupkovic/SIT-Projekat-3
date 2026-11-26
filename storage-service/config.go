package main

import (
	"errors"
	"time"

	"stashr/shared/appconfig"
)

// storageConfig holds the config.yaml keys only storage-service uses; the limits
// it shares with the other services are in appconfig.Limits.
type storageConfig struct {
	MongoDBName         string        `yaml:"storage_mongo_db_name"`
	MongoConnectTimeout time.Duration `yaml:"mongo_connect_timeout"`

	ObjectStoreBucket string `yaml:"object_store_bucket"`
	ObjectStoreUseSSL bool   `yaml:"object_store_use_ssl"`
	// The object store may still be starting when the service boots.
	ObjectStoreConnectAttempts   int           `yaml:"object_store_connect_attempts"`
	ObjectStoreConnectRetryDelay time.Duration `yaml:"object_store_connect_retry_delay"`

	QuotaServiceTimeout time.Duration `yaml:"quota_service_timeout"`
	// Removing a stored object and releasing quota after a failed upload.
	UploadCleanupTimeout time.Duration `yaml:"upload_cleanup_timeout"`
}

func defaultStorageConfig() storageConfig {
	return storageConfig{
		MongoDBName:                  "storage_db",
		MongoConnectTimeout:          10 * time.Second,
		ObjectStoreBucket:            "stashr-files",
		ObjectStoreConnectAttempts:   10,
		ObjectStoreConnectRetryDelay: 2 * time.Second,
		QuotaServiceTimeout:          5 * time.Second,
		UploadCleanupTimeout:         15 * time.Second,
	}
}

// loadConfig reads and validates config.yaml.
func loadConfig(path string) (appconfig.Limits, storageConfig, error) {
	limits := appconfig.DefaultLimits()
	cfg := defaultStorageConfig()
	if err := appconfig.Load(path, &limits, &cfg); err != nil {
		return limits, cfg, err
	}
	return limits, cfg, cfg.validate()
}

func (c storageConfig) validate() error {
	if c.MongoDBName == "" || c.ObjectStoreBucket == "" {
		return errors.New("storage_mongo_db_name and object_store_bucket must not be empty")
	}
	if c.ObjectStoreConnectAttempts < 1 {
		return errors.New("object_store_connect_attempts must be a positive integer")
	}
	return appconfig.PositiveDurations(map[string]time.Duration{
		"mongo_connect_timeout":            c.MongoConnectTimeout,
		"object_store_connect_retry_delay": c.ObjectStoreConnectRetryDelay,
		"quota_service_timeout":            c.QuotaServiceTimeout,
		"upload_cleanup_timeout":           c.UploadCleanupTimeout,
	})
}
