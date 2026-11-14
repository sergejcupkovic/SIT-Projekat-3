package config

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// env only
	Port         string `yaml:"-"`
	SMTPPort     string `yaml:"-"`
	GatewayPort  string `yaml:"-"`
	FrontendPort string `yaml:"-"`
	MongoURI     string `yaml:"-"`
	JWTSecret    string `yaml:"-"`

	// yaml
	PublicHost           string        `yaml:"public_host"`
	MongoDBName          string        `yaml:"mongo_db_name"`
	SMTPHost             string        `yaml:"smtp_host"`
	SMTPFrom             string        `yaml:"smtp_from"`
	DefaultQuotaBytes    int64         `yaml:"default_quota_bytes"`
	PasswordMinAge       time.Duration `yaml:"password_min_age"`
	PasswordMaxAge       time.Duration `yaml:"password_max_age"`
	ResetTokenTTL        time.Duration `yaml:"reset_token_ttl"`
	ResetRateLimitMax    int           `yaml:"reset_rate_limit_max"`
	ResetRateLimitWindow time.Duration `yaml:"reset_rate_limit_window"`
	MagicLinkTokenTTL    time.Duration `yaml:"magic_link_token_ttl"`
	AccountLockDuration  time.Duration `yaml:"account_lock_duration"`
	VerificationTokenTTL time.Duration `yaml:"verification_token_ttl"`
	OTPTTL               time.Duration `yaml:"otp_ttl"`
	OTPMaxAttempts       int           `yaml:"otp_max_attempts"`
	JWTTTL               time.Duration `yaml:"jwt_ttl"`
	RestrictedTokenTTL   time.Duration `yaml:"restricted_token_ttl"`
	BcryptCost           int           `yaml:"bcrypt_cost"`
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func LoadConfig() *Config {
	cfg := &Config{
		Port:         env("USERS_SERVICE_PORT", "8081"),
		SMTPPort:     env("SMTP_PORT", "1025"),
		GatewayPort:  env("GATEWAY_PORT", "8080"),
		FrontendPort: env("FRONTEND_PORT", "3000"),
		MongoURI:     env("MONGO_URI", "mongodb://mongodb:27017"),
		JWTSecret:    os.Getenv("JWT_SECRET"),

		PublicHost:           "localhost",
		MongoDBName:          "users_db",
		SMTPHost:             "mailpit",
		SMTPFrom:             "noreply@cloudstorage.local",
		DefaultQuotaBytes:    104857600,
		PasswordMinAge:       1 * time.Minute,
		PasswordMaxAge:       2 * time.Minute,
		ResetTokenTTL:        30 * time.Minute,
		AccountLockDuration:  1 * time.Minute,
		VerificationTokenTTL: 24 * time.Hour,
		ResetRateLimitMax:    3,
		ResetRateLimitWindow: 15 * time.Minute,
		MagicLinkTokenTTL:    15 * time.Minute,
		OTPTTL:               5 * time.Minute,
		OTPMaxAttempts:       3,
		JWTTTL:               24 * time.Hour,
		RestrictedTokenTTL:   15 * time.Minute,
		BcryptCost:           12,
	}

	path := env("CONFIG_PATH", "config.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config %s: %v", path, err)
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		log.Fatalf("parse config %s: %v", path, err)
	}

	if len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 chars")
	}
	if cfg.BcryptCost < 4 || cfg.BcryptCost > 31 {
		log.Fatal("bcrypt_cost must be 4..31")
	}
	return cfg
}
