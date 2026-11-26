// Package appconfig loads config.yaml. It owns the limits that more than one
// service has to agree on (users-service reports them to the frontend, while
// api-gateway and storage-service enforce them), so their defaults and their
// validation exist in exactly one place. Settings that only one service needs
// live in that service's own config struct.
//
// config.yaml is the only place a setting is changed: it overrides the built-in
// default, and no environment variable overrides it. The environment carries
// only secrets and deployment wiring (ports, addresses, file paths).
package appconfig

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// BcryptMaxPasswordBytes is the longest password bcrypt can hash.
const BcryptMaxPasswordBytes = 72

// Limits are the settings shared between services.
type Limits struct {
	// Storage quota, in bytes.
	DefaultQuotaBytes int64 `yaml:"default_quota_bytes"`
	MinQuotaBytes     int64 `yaml:"min_quota_bytes"`
	MaxQuotaBytes     int64 `yaml:"max_quota_bytes"`

	// Files and folders.
	MaxUploadBytes      int64 `yaml:"max_upload_bytes"`
	MaxFileNameLength   int   `yaml:"max_file_name_length"`
	MaxFolderNameLength int   `yaml:"max_folder_name_length"`
	// extension (with the dot) -> content types the server may detect for it
	AllowedFileTypes map[string][]string `yaml:"allowed_file_types"`

	// Accounts.
	MaxNameLength            int  `yaml:"max_name_length"`
	PasswordMinLength        int  `yaml:"password_min_length"`
	PasswordMaxLength        int  `yaml:"password_max_length"`
	PasswordRequireUppercase bool `yaml:"password_require_uppercase"`
	PasswordRequireLowercase bool `yaml:"password_require_lowercase"`
	PasswordRequireDigit     bool `yaml:"password_require_digit"`
	PasswordRequireSpecial   bool `yaml:"password_require_special"`
}

// DefaultLimits returns the values used for keys that config.yaml leaves out.
//
// AllowedFileTypes is filled in here for callers that do not load a file;
// Load clears it first, because yaml merges into an existing map and an
// extension removed from config.yaml would otherwise stay allowed.
func DefaultLimits() Limits {
	return Limits{
		DefaultQuotaBytes: 100 << 20, // 100 MB
		MinQuotaBytes:     100 << 20, // 100 MB
		MaxQuotaBytes:     2 << 40,   // 2 TB

		MaxUploadBytes:      200 << 20,
		MaxFileNameLength:   255,
		MaxFolderNameLength: 100,
		AllowedFileTypes:    defaultFileTypes(),

		MaxNameLength:            100,
		PasswordMinLength:        8,
		PasswordMaxLength:        BcryptMaxPasswordBytes,
		PasswordRequireUppercase: true,
		PasswordRequireLowercase: true,
		PasswordRequireDigit:     true,
		PasswordRequireSpecial:   true,
	}
}

func defaultFileTypes() map[string][]string {
	return map[string][]string{
		".txt":  {"text/plain"},
		".md":   {"text/plain"},
		".csv":  {"text/plain"},
		".json": {"text/plain"},
		".pdf":  {"application/pdf"},
		".png":  {"image/png"},
		".jpg":  {"image/jpeg"},
		".jpeg": {"image/jpeg"},
		".gif":  {"image/gif"},
	}
}

// Path is the location of config.yaml: CONFIG_PATH, or ./config.yaml.
func Path() string {
	if v := os.Getenv("CONFIG_PATH"); v != "" {
		return v
	}
	return "config.yaml"
}

// Load reads the file at path into limits and into each of extra (pointers to a
// service's own settings), then validates the limits. All of them must already
// hold their defaults.
func Load(path string, limits *Limits, extra ...any) error {
	limits.AllowedFileTypes = nil
	if err := Decode(path, append([]any{limits}, extra...)...); err != nil {
		return err
	}
	if limits.AllowedFileTypes == nil {
		limits.AllowedFileTypes = defaultFileTypes()
	}
	return limits.Validate()
}

// Decode reads the file at path into each target, for a service that needs none
// of the shared limits. Keys a struct does not know are ignored, so one file
// serves every service.
func Decode(path string, targets ...any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	for _, target := range targets {
		if err := yaml.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("parse config %s: %w", path, err)
		}
	}
	return nil
}

// PositiveDurations returns an error naming a key whose duration is not above 0.
func PositiveDurations(values map[string]time.Duration) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if values[key] <= 0 {
			return fmt.Errorf("%s must be a positive duration", key)
		}
	}
	return nil
}

// Validate rejects settings the services could not work with, and normalizes
// the extensions to lower case.
func (l *Limits) Validate() error {
	if l.MinQuotaBytes <= 0 {
		return errors.New("min_quota_bytes must be a positive number")
	}
	if l.MaxQuotaBytes < l.MinQuotaBytes {
		return errors.New("max_quota_bytes must not be smaller than min_quota_bytes")
	}
	if l.DefaultQuotaBytes < l.MinQuotaBytes || l.DefaultQuotaBytes > l.MaxQuotaBytes {
		return errors.New("default_quota_bytes must be between min_quota_bytes and max_quota_bytes")
	}
	if l.MaxUploadBytes <= 0 {
		return errors.New("max_upload_bytes must be a positive number")
	}
	if l.MaxFileNameLength <= 0 || l.MaxFolderNameLength <= 0 || l.MaxNameLength <= 0 {
		return errors.New("max_file_name_length, max_folder_name_length and max_name_length must be positive")
	}
	if l.PasswordMinLength < 1 || l.PasswordMaxLength < l.PasswordMinLength ||
		l.PasswordMaxLength > BcryptMaxPasswordBytes {
		return fmt.Errorf("password length bounds must satisfy 1 <= password_min_length <= password_max_length <= %d", BcryptMaxPasswordBytes)
	}

	if len(l.AllowedFileTypes) == 0 {
		return errors.New("allowed_file_types must list at least one extension")
	}
	normalized := make(map[string][]string, len(l.AllowedFileTypes))
	for ext, types := range l.AllowedFileTypes {
		if len(ext) < 2 || !strings.HasPrefix(ext, ".") {
			return fmt.Errorf("allowed_file_types: extension %q must start with a dot", ext)
		}
		if len(types) == 0 {
			return fmt.Errorf("allowed_file_types: %q needs at least one content type", ext)
		}
		normalized[strings.ToLower(ext)] = types
	}
	l.AllowedFileTypes = normalized
	return nil
}

// QuotaInRange reports whether bytes is an acceptable storage quota.
func (l Limits) QuotaInRange(bytes int64) bool {
	return bytes >= l.MinQuotaBytes && bytes <= l.MaxQuotaBytes
}

// QuotaRangeMessage is the error shown for a quota outside the bounds.
func (l Limits) QuotaRangeMessage() string {
	return fmt.Sprintf("Storage quota must be between %s and %s",
		FormatBytes(l.MinQuotaBytes), FormatBytes(l.MaxQuotaBytes))
}

// FormatBytes renders a size with binary units: 104857600 -> "100 MB",
// 1610612736 -> "1.5 GB".
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	value := float64(n) / unit
	i := 0
	for value >= unit && i < len(units)-1 {
		value /= unit
		i++
	}
	return strings.TrimSuffix(strconv.FormatFloat(value, 'f', 1, 64), ".0") + " " + units[i]
}

// AllowedExtensions lists the accepted file extensions (with the dot), sorted.
func (l Limits) AllowedExtensions() []string {
	exts := make([]string, 0, len(l.AllowedFileTypes))
	for ext := range l.AllowedFileTypes {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}
