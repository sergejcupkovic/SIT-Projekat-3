package service

import (
	"strings"
	"testing"
)

func registration(first, last, email string) *RegisterRequest {
	return &RegisterRequest{
		FirstName:       first,
		LastName:        last,
		Email:           email,
		Password:        "StrongPass123!",
		ConfirmPassword: "StrongPass123!",
	}
}

func TestValidateRegistration_Names(t *testing.T) {
	tests := []struct {
		name  string
		first string
		last  string
		valid bool
	}{
		{"simple", "Petar", "Petrovic", true},
		{"diacritics", "Željko", "Đorđević", true},
		{"apostrophe and hyphen", "Mary-Ann", "O'Brien", true},
		{"two words", "Ana Marija", "Jovanovic", true},
		{"max length", strings.Repeat("a", 100), "Petrovic", true},
		{"too long", strings.Repeat("a", 101), "Petrovic", false},
		{"digits", "Petar2", "Petrovic", false},
		{"script tag", "<script>alert(1)</script>", "Petrovic", false},
		{"html in last name", "Petar", "<img src=x onerror=alert(1)>", false},
		{"sql injection", "Robert'); DROP TABLE users;--", "Petrovic", false},
		{"nosql operator", "Petar", `{"$gt": ""}`, false},
		{"leading hyphen", "-Petar", "Petrovic", false},
		{"control character", "Pe\x00tar", "Petrovic", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRegistration(registration(tc.first, tc.last, "petar@example.com"))
			if tc.valid && err != nil {
				t.Errorf("expected valid, got: %v", err)
			}
			if !tc.valid && err == nil {
				t.Errorf("expected validation error for %q %q", tc.first, tc.last)
			}
		})
	}
}

func TestValidateRegistration_Email(t *testing.T) {
	invalid := []string{
		"plainaddress",
		"@example.com",
		"user@",
		"user@example",
		"user name@example.com",
		"user@example.com\r\nBcc: victim@example.com",
		`"><script>@example.com`,
		strings.Repeat("a", 250) + "@example.com",
	}
	for _, email := range invalid {
		if err := ValidateRegistration(registration("Petar", "Petrovic", email)); err == nil {
			t.Errorf("expected email %q to be rejected", email)
		}
	}
}

func TestValidatePassword_Boundaries(t *testing.T) {
	if err := ValidatePassword("Ab1!xyz"); err == nil {
		t.Error("7 characters must be rejected")
	}
	if err := ValidatePassword("Ab1!xyzw"); err != nil {
		t.Errorf("8 characters must be accepted, got: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("Ab1!", 18)); err != nil {
		t.Errorf("72 characters must be accepted, got: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("Ab1!", 18) + "x"); err == nil {
		t.Error("73 characters must be rejected")
	}
}

func TestValidateCredentials(t *testing.T) {
	if err := ValidateCredentials("user@example.com", "whatever"); err != nil {
		t.Errorf("expected valid credentials, got: %v", err)
	}
	if err := ValidateCredentials("", "whatever"); err == nil {
		t.Error("empty email must be rejected")
	}
	if err := ValidateCredentials("user@example.com", ""); err == nil {
		t.Error("empty password must be rejected")
	}
	if err := ValidateCredentials("user@example.com", strings.Repeat("a", 73)); err == nil {
		t.Error("password above the bcrypt limit must be rejected")
	}
	if err := ValidateCredentials(`{"$ne": ""}`, "whatever"); err == nil {
		t.Error("operator-like email must be rejected")
	}
}

func TestValidateOTP(t *testing.T) {
	for _, otp := range []string{"123456", "000000"} {
		if err := ValidateOTP(otp); err != nil {
			t.Errorf("expected %q to be accepted, got: %v", otp, err)
		}
	}
	for _, otp := range []string{"", "12345", "1234567", "12345a", " 12345", "12.456", `{"$ne":1}`} {
		if err := ValidateOTP(otp); err == nil {
			t.Errorf("expected %q to be rejected", otp)
		}
	}
}

func TestValidateToken(t *testing.T) {
	generated, err := GenerateSecureToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateToken(generated); err != nil {
		t.Errorf("a generated token must be accepted, got: %v", err)
	}
	for _, token := range []string{"", "abc", strings.Repeat("g", 64), strings.Repeat("A", 64), generated + "0", `{"$ne":""}`} {
		if err := ValidateToken(token); err == nil {
			t.Errorf("expected %q to be rejected", token)
		}
	}
}

func TestValidateChangePassword(t *testing.T) {
	if err := ValidateChangePassword("OldValidPass1!", "NewValidPass2!", "NewValidPass2!"); err != nil {
		t.Errorf("expected valid input to pass, got: %v", err)
	}
	for name, in := range map[string][3]string{
		"missing current":     {"", "NewValidPass2!", "NewValidPass2!"},
		"current over bcrypt": {strings.Repeat("a", 73), "NewValidPass2!", "NewValidPass2!"},
		"missing new":         {"OldValidPass1!", "", ""},
		"mismatch":            {"OldValidPass1!", "NewValidPass2!", "NewValidPass3!"},
		"same as current":     {"OldValidPass1!", "OldValidPass1!", "OldValidPass1!"},
		"weak new password":   {"OldValidPass1!", "weak", "weak"},
	} {
		if err := ValidateChangePassword(in[0], in[1], in[2]); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidatePasswordFollowsConfiguredRules(t *testing.T) {
	saved := Cfg.Limits
	defer func() { Cfg.Limits = saved }()

	Cfg.PasswordMinLength = 12
	Cfg.PasswordRequireSpecial = false
	if err := ValidatePassword("Abcdefgh1"); err == nil || err.Error() != "password must be 12-72 characters long" {
		t.Errorf("configured length not applied: %v", err)
	}
	if err := ValidatePassword("Abcdefghijk1"); err != nil {
		t.Errorf("special character is not required any more: %v", err)
	}
	if err := ValidatePassword("abcdefghijk1"); err == nil ||
		err.Error() != "password must contain an uppercase letter, a lowercase letter, and a digit" {
		t.Errorf("unexpected message: %v", err)
	}
}
