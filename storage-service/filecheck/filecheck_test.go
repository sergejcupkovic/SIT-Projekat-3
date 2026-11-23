package filecheck

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

var (
	pngHead = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	pdfHead = []byte("%PDF-1.7\n")
	exeHead = []byte("MZ\x90\x00\x03\x00\x00\x00")
)

func TestValidateUpload(t *testing.T) {
	tests := []struct {
		name string
		file string
		head []byte
		want error
	}{
		{"text file", "notes.txt", []byte("hello world"), nil},
		{"png image", "slika.png", pngHead, nil},
		{"upper case extension", "SLIKA.PNG", pngHead, nil},
		{"pdf", "izvestaj.pdf", pdfHead, nil},
		{"executable", "setup.exe", exeHead, ErrTypeNotAllowed},
		{"script", "run.sh", []byte("#!/bin/sh"), ErrTypeNotAllowed},
		{"html", "index.html", []byte("<html>"), ErrTypeNotAllowed},
		{"no extension", "README", []byte("hello"), ErrTypeNotAllowed},
		{"executable renamed to png", "photo.png", exeHead, ErrContentMismatch},
		{"html disguised as txt", "a.txt", []byte("<!DOCTYPE html><html>"), ErrContentMismatch},
		{"double extension", "evil.exe.png", exeHead, ErrContentMismatch},
		{"path traversal", "../../etc/passwd.txt", []byte("x"), ErrBadName},
		{"null byte", "a.txt\x00.png", pngHead, ErrBadName},
		{"script in name", "<img src=x>.png", pngHead, ErrBadName},
		{"empty name", "", nil, ErrBadName},
		{"name too long", strings.Repeat("a", 256) + ".txt", []byte("x"), ErrBadName},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateUpload(tc.file, tc.head); !errors.Is(err, tc.want) {
				t.Errorf("ValidateUpload(%q) = %v, want %v", tc.file, err, tc.want)
			}
		})
	}
}

func TestChecksumAndVerify(t *testing.T) {
	content := []byte("sadrzaj datoteke")

	sum, n, err := Checksum(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(content)) {
		t.Errorf("size = %d, want %d", n, len(content))
	}
	if len(sum) != 64 {
		t.Errorf("expected a 64 character hex digest, got %q", sum)
	}

	if err := Verify(bytes.NewReader(content), sum); err != nil {
		t.Errorf("untouched content must verify, got: %v", err)
	}

	tampered := append([]byte(nil), content...)
	tampered[0] ^= 0x01
	if err := Verify(bytes.NewReader(tampered), sum); !errors.Is(err, ErrIntegrity) {
		t.Errorf("modified content must fail verification, got: %v", err)
	}

	if err := Verify(bytes.NewReader(content), "not-a-hash"); !errors.Is(err, ErrIntegrity) {
		t.Errorf("malformed stored hash must fail verification, got: %v", err)
	}
}

func TestDetectType(t *testing.T) {
	tests := []struct {
		name string
		head []byte
		want string
	}{
		{"png", pngHead, "image/png"},
		{"pdf", pdfHead, "application/pdf"},
		{"text carries no charset parameter", []byte("hello"), "text/plain"},
		{"executable is not claimed to be anything friendly", exeHead, "application/octet-stream"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectType(tc.head); got != tc.want {
				t.Errorf("DetectType = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAllowedExtensions(t *testing.T) {
	got := strings.Join(AllowedExtensions(), " ")
	want := ".csv .gif .jpeg .jpg .json .md .pdf .png .txt"
	if got != want {
		t.Errorf("AllowedExtensions = %q, want %q", got, want)
	}
}
