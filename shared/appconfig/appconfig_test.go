package appconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultsApplyWhenKeysAreMissing(t *testing.T) {
	limits := DefaultLimits()
	if err := Load(write(t, "public_host: localhost\n"), &limits); err != nil {
		t.Fatal(err)
	}
	if limits.MinQuotaBytes != 104857600 || limits.MaxQuotaBytes != 2199023255552 {
		t.Fatalf("unexpected quota bounds: %d..%d", limits.MinQuotaBytes, limits.MaxQuotaBytes)
	}
	if len(limits.AllowedFileTypes) != 9 || !limits.PasswordRequireSpecial {
		t.Fatalf("defaults were lost: %+v", limits)
	}
}

func TestFileOverridesDefaultsAndFillsExtraStructs(t *testing.T) {
	path := write(t, `
min_quota_bytes: 1000
default_quota_bytes: 2000
max_quota_bytes: 3000
password_require_special: false
allowed_file_types:
  ".TXT": [text/plain]
some_timeout: 7s
`)
	extra := struct {
		SomeTimeout time.Duration `yaml:"some_timeout"`
	}{SomeTimeout: time.Second}

	limits := DefaultLimits()
	if err := Load(path, &limits, &extra); err != nil {
		t.Fatal(err)
	}
	if limits.MinQuotaBytes != 1000 || limits.MaxQuotaBytes != 3000 || limits.PasswordRequireSpecial {
		t.Fatalf("file values not applied: %+v", limits)
	}
	// The list replaces the default one; it is not merged into it.
	if len(limits.AllowedFileTypes) != 1 || limits.AllowedFileTypes[".txt"] == nil {
		t.Fatalf("unexpected file types: %v", limits.AllowedFileTypes)
	}
	if extra.SomeTimeout != 7*time.Second {
		t.Fatalf("extra struct not filled: %v", extra.SomeTimeout)
	}
}

// A stale variable left in .env must not silently win over config.yaml.
func TestEnvironmentDoesNotOverrideTheFile(t *testing.T) {
	t.Setenv("MAX_UPLOAD_BYTES", "123")
	t.Setenv("MAX_QUOTA_BYTES", "456")
	limits := DefaultLimits()
	if err := Load(write(t, "max_upload_bytes: 999\nmax_quota_bytes: 209715200\n"), &limits); err != nil {
		t.Fatal(err)
	}
	if limits.MaxUploadBytes != 999 || limits.MaxQuotaBytes != 209715200 {
		t.Fatalf("the file must be the only source: %+v", limits)
	}
}

func TestPositiveDurations(t *testing.T) {
	if err := PositiveDurations(map[string]time.Duration{"a": time.Second}); err != nil {
		t.Fatal(err)
	}
	err := PositiveDurations(map[string]time.Duration{"b_timeout": 0, "a": time.Second})
	if err == nil || err.Error() != "b_timeout must be a positive duration" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvalidSettingsAreRejected(t *testing.T) {
	for name, content := range map[string]string{
		"max below min":          "min_quota_bytes: 500\nmax_quota_bytes: 100\n",
		"default below min":      "default_quota_bytes: 1\n",
		"zero minimum":           "min_quota_bytes: 0\n",
		"zero upload":            "max_upload_bytes: 0\n",
		"password over bcrypt":   "password_max_length: 100\n",
		"password min above max": "password_min_length: 20\npassword_max_length: 10\n",
		"extension without dot":  "allowed_file_types:\n  txt: [text/plain]\n",
		"extension without type": "allowed_file_types:\n  \".txt\": []\n",
		"empty type list":        "allowed_file_types: {}\n",
	} {
		limits := DefaultLimits()
		if err := Load(write(t, content), &limits); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestQuotaRange(t *testing.T) {
	l := DefaultLimits()
	for bytes, want := range map[int64]bool{
		-1: false, 0: false, l.MinQuotaBytes - 1: false,
		l.MinQuotaBytes: true, l.MaxQuotaBytes: true, l.MaxQuotaBytes + 1: false,
	} {
		if got := l.QuotaInRange(bytes); got != want {
			t.Errorf("QuotaInRange(%d) = %v, want %v", bytes, got, want)
		}
	}
	if got, want := l.QuotaRangeMessage(), "Storage quota must be between 100 MB and 2 TB"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0 B", 1023: "1023 B", 1024: "1 KB", 1610612736: "1.5 GB", 200 << 20: "200 MB", 2 << 40: "2 TB",
	} {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

// The file shipped in the repository must load.
func TestRepositoryConfigIsValid(t *testing.T) {
	limits := DefaultLimits()
	if err := Load(filepath.Join("..", "..", "config.yaml"), &limits); err != nil {
		t.Fatal(err)
	}
}
