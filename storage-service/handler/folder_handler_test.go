package handler

import (
	"strings"
	"testing"
)

func TestValidateFolderName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"plain", "Documents", true},
		{"with spaces and digits", "Projekat 2026 (v2)", true},
		{"unicode letters", "Školski radovi", true},
		{"max length", strings.Repeat("a", 100), true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 101), false},
		{"multibyte counted as characters", strings.Repeat("š", 100), true},
		{"dot", ".", false},
		{"double dot", "..", false},
		{"only dots and spaces", ". .", false},
		{"path separator", "a/b", false},
		{"backslash", `a\b`, false},
		{"script tag", "<script>alert(1)</script>", false},
		{"html entity", "a&amp;b", false},
		{"quote", `a"b`, false},
		{"sql injection", "x'; DROP TABLE folders;--", false},
		{"nosql operator", `{"$ne": null}`, false},
		{"control character", "a\x00b", false},
		{"newline", "a\nb", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFolderName(tc.input)
			if tc.valid && err != nil {
				t.Errorf("expected %q to be accepted, got: %v", tc.input, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("expected %q to be rejected", tc.input)
			}
		})
	}
}
