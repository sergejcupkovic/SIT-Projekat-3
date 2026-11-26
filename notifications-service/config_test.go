package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gocql/gocql"
)

func TestRepositoryConfigLoads(t *testing.T) {
	cfg, options, err := loadConfig(filepath.Join("..", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The shipped file must keep the behaviour the service had before.
	if cfg.Keyspace != "stashr_notifications" || cfg.ConnectAttempts != 20 || cfg.ConnectRetryDelay != 3*time.Second {
		t.Fatalf("unexpected values: %+v", cfg)
	}
	if options.SchemaConsistency != gocql.Quorum || options.Consistency != gocql.One ||
		options.ReplicationFactor != 1 || options.Timeout != 10*time.Second {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestInvalidCassandraSettingsAreRejected(t *testing.T) {
	for _, bad := range []string{
		"cassandra_keyspace: \"x; DROP KEYSPACE y\"\n",
		"cassandra_keyspace: \"\"\n",
		"cassandra_connect_attempts: 0\n",
		"cassandra_replication_factor: 0\n",
		"cassandra_timeout: 0s\n",
		"cassandra_consistency: most\n",
		"cassandra_schema_consistency: \"\"\n",
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadConfig(path); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}
