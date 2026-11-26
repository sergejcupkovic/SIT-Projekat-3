package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"users-service/config"

	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	Init(&config.Config{
		JWTSecret:          "test-secret-test-secret-test-secret-test",
		BcryptCost:         bcrypt.MinCost,
		JWTTTL:             time.Hour,
		RestrictedTokenTTL: time.Minute,
		PasswordMinLength:  8,
		MinQuotaBytes:      100 << 20,
		MaxQuotaBytes:      2 << 40,
	})
	os.Exit(m.Run())
}

func validRegistration() RegisterRequest {
	return RegisterRequest{
		FirstName:       "Petar",
		LastName:        "Petrovic",
		Email:           "petar@example.com",
		Password:        "StrongPass123!",
		ConfirmPassword: "StrongPass123!",
	}
}

func TestValidateRegistration(t *testing.T) {
	req := validRegistration()
	if err := ValidateRegistration(&req); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	cases := map[string]func(r *RegisterRequest){
		"missing name":      func(r *RegisterRequest) { r.FirstName = "" },
		"digits in name":    func(r *RegisterRequest) { r.FirstName = "Petar2" },
		"invalid email":     func(r *RegisterRequest) { r.Email = "not-an-email" },
		"weak password":     func(r *RegisterRequest) { r.Password, r.ConfirmPassword = "weakpass", "weakpass" },
		"password mismatch": func(r *RegisterRequest) { r.ConfirmPassword = "OtherPass123!" },
	}
	for name, breakIt := range cases {
		req := validRegistration()
		breakIt(&req)
		if err := ValidateRegistration(&req); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRegistrationNormalizesEmail(t *testing.T) {
	req := validRegistration()
	req.Email = "  Petar@Example.COM "
	if err := ValidateRegistration(&req); err != nil {
		t.Fatal(err)
	}
	if req.Email != "petar@example.com" {
		t.Errorf("got %q", req.Email)
	}
}

func TestValidatePasswordLength(t *testing.T) {
	if ValidatePassword("Ab1!xyz") == nil {
		t.Error("7 characters must be rejected")
	}
	if err := ValidatePassword("Ab1!xyzw"); err != nil {
		t.Errorf("8 characters must be accepted: %v", err)
	}
	if ValidatePassword(strings.Repeat("Ab1!", 18)+"x") == nil {
		t.Error("73 characters must be rejected")
	}
}

func TestValidateChangePassword(t *testing.T) {
	const current, next = "OldValidPass1!", "NewValidPass2!"
	if err := ValidateChangePassword(current, next, next); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if ValidateChangePassword("", next, next) == nil {
		t.Error("the current password is required")
	}
	if ValidateChangePassword(current, next, "Different3!") == nil {
		t.Error("the confirmation must match")
	}
	if ValidateChangePassword(current, current, current) == nil {
		t.Error("the new password must differ from the current one")
	}
}

func TestOTPAndTokenFormat(t *testing.T) {
	if ValidateOTP("123456") != nil || ValidateOTP("12a456") == nil {
		t.Error("an OTP is exactly 6 digits")
	}
	token, err := GenerateSecureToken()
	if err != nil {
		t.Fatal(err)
	}
	if ValidateToken(token) != nil || ValidateToken(`{"$ne":""}`) == nil {
		t.Error("only 64 hex characters are a token")
	}
}

func TestSessionTokens(t *testing.T) {
	const id = "60d5ec49f1b2c82b8c8e4e01"
	user, _ := GenerateJWTToken(id, "a@example.com", "RK")
	admin, _ := GenerateJWTToken(id, "a@example.com", "A")
	restricted, _ := GenerateRestrictedJWTToken(id, "a@example.com", "RK")

	if got, err := VerifySessionToken(user); err != nil || got != id {
		t.Errorf("user token: %q, %v", got, err)
	}
	if _, err := VerifySessionToken(restricted); err == nil {
		t.Error("a restricted token is not a session")
	}
	if _, err := VerifySessionToken(user + "x"); err == nil {
		t.Error("a changed token must be rejected")
	}
	if _, err := VerifyAdminToken(admin); err != nil {
		t.Errorf("admin token rejected: %v", err)
	}
	if _, err := VerifyAdminToken(user); err == nil {
		t.Error("a regular user is not an admin")
	}
}
