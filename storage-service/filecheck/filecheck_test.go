package filecheck

import (
	"errors"
	"testing"
)

func TestValidateUpload(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	exe := []byte("MZ\x90\x00\x03\x00\x00\x00")

	tests := []struct {
		name string
		file string
		head []byte
		want error
	}{
		{"text file", "notes.txt", []byte("hello world"), nil},
		{"png image", "slika.png", png, nil},
		{"executable", "setup.exe", exe, ErrTypeNotAllowed},
		{"executable renamed to png", "photo.png", exe, ErrContentMismatch},
		{"path traversal", "../../passwd.txt", []byte("x"), ErrBadName},
		{"empty name", "", nil, ErrBadName},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateUpload(tc.file, tc.head); !errors.Is(err, tc.want) {
				t.Errorf("ValidateUpload(%q) = %v, want %v", tc.file, err, tc.want)
			}
		})
	}
}
