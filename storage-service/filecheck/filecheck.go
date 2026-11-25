// Package filecheck holds the checks applied to user supplied files: name and
// type whitelisting before a file is stored, and SHA-256 integrity hashing
// when it is stored and read back.
package filecheck

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxFileNameLen = 255

var (
	ErrBadName         = errors.New("file name contains invalid characters")
	ErrTypeNotAllowed  = errors.New("file type is not allowed")
	ErrContentMismatch = errors.New("file content does not match its extension")
	ErrIntegrity       = errors.New("file integrity check failed")
)

var fileNameRegex = regexp.MustCompile(`^[\p{L}\p{N} ._()\-]+$`)

// extension -> content types http.DetectContentType may report for it
var allowedTypes = map[string][]string{
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

// ValidateUpload checks the file name against the whitelist of characters and
// extensions, and that the first bytes of the content really are what the
// extension claims (a renamed executable is rejected).
func ValidateUpload(name string, head []byte) error {
	if err := ValidateFileName(name); err != nil {
		return err
	}

	want, ok := allowedTypes[strings.ToLower(filepath.Ext(name))]
	if !ok {
		return ErrTypeNotAllowed
	}

	if len(head) > 512 {
		head = head[:512]
	}
	got := http.DetectContentType(head)
	if i := strings.Index(got, ";"); i >= 0 {
		got = got[:i]
	}
	for _, w := range want {
		if got == w {
			return nil
		}
	}
	return ErrContentMismatch
}

// Checksum reads r to the end and returns its SHA-256 (hex) and size in bytes.
func Checksum(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Verify recomputes the SHA-256 of r and compares it with the hash that was
// stored at upload time.
func Verify(r io.Reader, expectedHex string) error {
	expected, err := hex.DecodeString(expectedHex)
	if err != nil || len(expected) != sha256.Size {
		return ErrIntegrity
	}

	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(h.Sum(nil), expected) != 1 {
		return ErrIntegrity
	}
	return nil
}

// DetectType returns the content type of a file as seen by the SERVER (from
// its first bytes), without parameters, e.g. "image/png" or "text/plain".
// Use it instead of the Content-Type the client claims.
func DetectType(head []byte) string {
	if len(head) > 512 {
		head = head[:512]
	}
	got := http.DetectContentType(head)
	if i := strings.Index(got, ";"); i >= 0 {
		got = got[:i]
	}
	return got
}

// AllowedExtensions lists the whitelisted file extensions (with the dot), sorted.
func AllowedExtensions() []string {
	exts := make([]string, 0, len(allowedTypes))
	for ext := range allowedTypes {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// ValidateFileName checks that name is non-empty, within maxFileNameLen runes,
// uses only whitelisted characters, and is not made solely of dots/spaces.
func ValidateFileName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxFileNameLen ||
		!fileNameRegex.MatchString(name) || strings.Trim(name, ". ") == "" {
		return ErrBadName
	}
	return nil
}
