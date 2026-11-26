package main

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"notifications-service/repository"

	"stashr/shared/appconfig"

	"github.com/gocql/gocql"
)

// notificationsConfig holds the config.yaml keys of notifications-service.
type notificationsConfig struct {
	Keyspace string `yaml:"cassandra_keyspace"`
	// Cassandra takes a while to accept connections after it starts.
	ConnectAttempts   int           `yaml:"cassandra_connect_attempts"`
	ConnectRetryDelay time.Duration `yaml:"cassandra_connect_retry_delay"`
	Timeout           time.Duration `yaml:"cassandra_timeout"`
	// Only used when the keyspace is first created.
	ReplicationFactor int    `yaml:"cassandra_replication_factor"`
	SchemaConsistency string `yaml:"cassandra_schema_consistency"`
	Consistency       string `yaml:"cassandra_consistency"`
}

func defaultNotificationsConfig() notificationsConfig {
	return notificationsConfig{
		Keyspace:          "stashr_notifications",
		ConnectAttempts:   20,
		ConnectRetryDelay: 3 * time.Second,
		Timeout:           10 * time.Second,
		ReplicationFactor: 1,
		SchemaConsistency: "quorum",
		Consistency:       "one",
	}
}

// The keyspace name is written into CQL statements, so only an identifier will do.
var keyspaceRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,47}$`)

// loadConfig reads and validates config.yaml, and returns the settings together
// with the repository options they describe.
func loadConfig(path string) (notificationsConfig, repository.Options, error) {
	cfg := defaultNotificationsConfig()
	if err := appconfig.Decode(path, &cfg); err != nil {
		return cfg, repository.Options{}, err
	}
	options, err := cfg.options()
	return cfg, options, err
}

func (c notificationsConfig) options() (repository.Options, error) {
	var none repository.Options
	if !keyspaceRegex.MatchString(c.Keyspace) {
		return none, errors.New("cassandra_keyspace must start with a letter and use only letters, digits and _ (max 48)")
	}
	if c.ConnectAttempts < 1 || c.ReplicationFactor < 1 {
		return none, errors.New("cassandra_connect_attempts and cassandra_replication_factor must be positive integers")
	}
	if err := appconfig.PositiveDurations(map[string]time.Duration{
		"cassandra_connect_retry_delay": c.ConnectRetryDelay,
		"cassandra_timeout":             c.Timeout,
	}); err != nil {
		return none, err
	}
	schema, err := gocql.ParseConsistencyWrapper(c.SchemaConsistency)
	if err != nil {
		return none, fmt.Errorf("cassandra_schema_consistency: %w", err)
	}
	consistency, err := gocql.ParseConsistencyWrapper(c.Consistency)
	if err != nil {
		return none, fmt.Errorf("cassandra_consistency: %w", err)
	}
	return repository.Options{
		Timeout:           c.Timeout,
		ReplicationFactor: c.ReplicationFactor,
		SchemaConsistency: schema,
		Consistency:       consistency,
	}, nil
}
