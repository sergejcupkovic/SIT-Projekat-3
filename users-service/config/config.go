package config

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"stashr/shared/appconfig"
)

type Config struct {
	// Limits shared with the other services (quota bounds, file and password
	// rules). Loaded by appconfig, so yaml.v3 must skip it here.
	appconfig.Limits `yaml:"-"`

	// env only: secrets and deployment wiring
	Port          string `yaml:"-"`
	SMTPPort      string `yaml:"-"`
	FrontendPort  string `yaml:"-"`
	MongoURI      string `yaml:"-"`
	JWTSecret     string `yaml:"-"`
	AdminEmail    string `yaml:"-"`
	AdminPassword string `yaml:"-"`

	// yaml
	PublicHost           string        `yaml:"public_host"`
	MongoDBName          string        `yaml:"mongo_db_name"`
	SMTPHost             string        `yaml:"smtp_host"`
	SMTPFrom             string        `yaml:"smtp_from"`
	SMTPTimeout          time.Duration `yaml:"smtp_timeout"`
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

	// Connecting to MongoDB and creating the indexes at start-up.
	MongoConnectTimeout time.Duration `yaml:"mongo_connect_timeout"`
	// Putting a quota request back to pending after its approval failed.
	QuotaRequestRollbackTimeout time.Duration `yaml:"quota_request_rollback_timeout"`

	UI `yaml:",inline"`
}

// UI holds the frontend's settings; GET /api/config/limits hands them over.
// Everything here is public, so nothing secret belongs in it.
type UI struct {
	// A storage bar turns red from this share of the quota.
	StorageWarningPercent int `yaml:"storage_warning_percent"`
	// How many of their own past quota requests a user sees.
	QuotaRequestHistoryItems int `yaml:"quota_request_history_items"`
	// The request form suggests the current quota times this.
	QuotaRequestSuggestionFactor float64 `yaml:"quota_request_suggestion_factor"`
	// The "quick adjust" buttons of the admin quota editor (negative = less).
	AdminQuotaAdjustStepsBytes []int64 `yaml:"admin_quota_adjust_steps_bytes"`
	// Web inbox of the local mail catcher, linked from the pages; empty hides the links.
	MailUIURL string `yaml:"mail_ui_url"`
}

func DefaultUI() UI {
	return UI{
		StorageWarningPercent:        90,
		QuotaRequestHistoryItems:     5,
		QuotaRequestSuggestionFactor: 2,
		AdminQuotaAdjustStepsBytes:   []int64{-1 << 30, -100 << 20, 100 << 20, 1 << 30},
		MailUIURL:                    "http://localhost:8025",
	}
}

// Validate rejects UI settings the frontend could not work with.
func (u UI) Validate() error {
	if u.StorageWarningPercent < 1 || u.StorageWarningPercent > 100 {
		return errors.New("storage_warning_percent must be 1..100")
	}
	if u.QuotaRequestHistoryItems < 0 {
		return errors.New("quota_request_history_items must not be negative")
	}
	if !(u.QuotaRequestSuggestionFactor > 1) {
		return errors.New("quota_request_suggestion_factor must be greater than 1")
	}
	for _, step := range u.AdminQuotaAdjustStepsBytes {
		if step == 0 {
			return errors.New("admin_quota_adjust_steps_bytes must not contain 0")
		}
	}
	// It becomes a link, so no javascript: or other schemes.
	if u.MailUIURL != "" && !strings.HasPrefix(u.MailUIURL, "http://") && !strings.HasPrefix(u.MailUIURL, "https://") {
		return errors.New("mail_ui_url must start with http:// or https://, or be empty")
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func LoadConfig() *Config {
	cfg := &Config{
		Limits: appconfig.DefaultLimits(),

		Port:         env("USERS_GRPC_PORT", "50051"),
		SMTPPort:     env("SMTP_PORT", "1025"),
		FrontendPort: env("FRONTEND_PORT", "3000"),
		MongoURI:     env("MONGO_URI", "mongodb://mongodb:27017"),
		JWTSecret:    os.Getenv("JWT_SECRET"),

		AdminEmail:    env("ADMIN_EMAIL", "admin@stashr.com"),
		AdminPassword: env("ADMIN_PASSWORD", ""),

		PublicHost:           "localhost",
		MongoDBName:          "users_db",
		SMTPHost:             "mailpit",
		SMTPFrom:             "noreply@stashr.com",
		SMTPTimeout:          10 * time.Second,
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

		MongoConnectTimeout:         10 * time.Second,
		QuotaRequestRollbackTimeout: 5 * time.Second,
		UI:                          DefaultUI(),
	}

	if err := appconfig.Load(appconfig.Path(), &cfg.Limits, cfg); err != nil {
		log.Fatal(err)
	}
	if len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 chars")
	}
	if cfg.BcryptCost < 4 || cfg.BcryptCost > 31 {
		log.Fatal("bcrypt_cost must be 4..31")
	}
	if err := appconfig.PositiveDurations(map[string]time.Duration{
		"smtp_timeout":                   cfg.SMTPTimeout,
		"mongo_connect_timeout":          cfg.MongoConnectTimeout,
		"quota_request_rollback_timeout": cfg.QuotaRequestRollbackTimeout,
	}); err != nil {
		log.Fatal(err)
	}
	if err := cfg.UI.Validate(); err != nil {
		log.Fatal(err)
	}
	return cfg
}
