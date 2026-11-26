// Package filecheck checks the name and content type of uploaded files.
package filecheck

import (
	"errors"
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

func AllowedExtensions() []string {
	exts := make([]string, 0, len(allowedTypes))
	for ext := range allowedTypes {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

func ValidateFileName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxFileNameLen ||
		!fileNameRegex.MatchString(name) || strings.Trim(name, ". ") == "" {
		return ErrBadName
	}
	return nil
}
