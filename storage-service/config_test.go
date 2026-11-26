package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoryConfigLoads(t *testing.T) {
	_, cfg, err := loadConfig(filepath.Join("..", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The shipped file must not rename the existing database or bucket.
	if cfg.MongoDBName != "storage_db" || cfg.ObjectStoreBucket != "stashr-files" {
		t.Fatalf("unexpected names: %+v", cfg)
	}
}

func TestStorageConfigValues(t *testing.T) {
	load := func(content string) (storageConfig, error) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		_, cfg, err := loadConfig(path)
		return cfg, err
	}

	cfg, err := load("object_store_bucket: other\nobject_store_connect_attempts: 3\nupload_cleanup_timeout: 1m\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ObjectStoreBucket != "other" || cfg.ObjectStoreConnectAttempts != 3 || cfg.UploadCleanupTimeout != time.Minute {
		t.Fatalf("file values not applied: %+v", cfg)
	}

	for _, bad := range []string{
		"object_store_bucket: \"\"\n", "storage_mongo_db_name: \"\"\n",
		"object_store_connect_attempts: 0\n", "upload_cleanup_timeout: 0s\n",
	} {
		if _, err := load(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}
