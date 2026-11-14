package service

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestValidateRegistration_Success(t *testing.T) {
	req := RegisterRequest{
		FirstName:       "Petar",
		LastName:        "Petrovic",
		Email:           "petar@example.com",
		Password:        "StrongPass123!",
		ConfirmPassword: "StrongPass123!",
	}
	if err := ValidateRegistration(req); err != nil {
		t.Errorf("Expected no error for valid input, got: %v", err)
	}
}

func TestValidateRegistration_WeakPassword(t *testing.T) {
	req := RegisterRequest{
		FirstName:       "Marko",
		LastName:        "Markovic",
		Email:           "marko@test.com",
		Password:        "slabalozinka",
		ConfirmPassword: "slabalozinka",
	}
	if err := ValidateRegistration(req); err == nil {
		t.Errorf("Expected error for weak password, got nil")
	}
}

func TestValidateRegistration_PasswordMismatch(t *testing.T) {
	req := RegisterRequest{
		FirstName:       "Marko",
		LastName:        "Markovic",
		Email:           "marko@test.com",
		Password:        "StrongPass123!",
		ConfirmPassword: "DifferentPass123!",
	}
	if err := ValidateRegistration(req); err == nil {
		t.Errorf("Expected error for mismatched passwords, got nil")
	}
}

func TestValidateRegistration_InvalidEmail(t *testing.T) {
	req := RegisterRequest{
		FirstName:       "Marko",
		LastName:        "Markovic",
		Email:           "not-an-email",
		Password:        "StrongPass123!",
		ConfirmPassword: "StrongPass123!",
	}
	if err := ValidateRegistration(req); err == nil {
		t.Errorf("Expected error for invalid email, got nil")
	}
}

func TestHashPassword(t *testing.T) {
	password := "StrongPass123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Expected no error while hashing, got: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("Hash does not match original password: %v", err)
	}
}

func TestGenerateSecureToken(t *testing.T) {
	token, err := GenerateSecureToken()
	if err != nil {
		t.Fatalf("Expected no error generating token, got: %v", err)
	}
	if len(token) != 64 { // 32 bytes hex-encoded = 64 characters
		t.Errorf("Expected token length 64, got %d", len(token))
	}
}

func TestHashResetToken_Deterministic(t *testing.T) {
	raw := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	first := HashResetToken(raw)
	second := HashResetToken(raw)
	if first != second {
		t.Errorf("Expected deterministic hash, got %s and %s", first, second)
	}
	if first == raw {
		t.Errorf("Hash must not equal the raw token")
	}
	if len(first) != 64 {
		t.Errorf("Expected SHA-256 hex length 64, got %d", len(first))
	}
}

func TestPasswordChangeAllowed_TooYoung(t *testing.T) {
	lastChanged := time.Now()
	ok, retryAt := PasswordChangeAllowed(lastChanged)
	if ok {
		t.Errorf("Expected password change to be blocked when younger than 24h")
	}
	if retryAt.Sub(lastChanged) < PasswordMinAge-time.Second {
		t.Errorf("Expected retryAt about 24h after last change, got %v", retryAt)
	}
}

func TestPasswordChangeAllowed_OldEnough(t *testing.T) {
	lastChanged := time.Now().Add(-25 * time.Hour)
	ok, _ := PasswordChangeAllowed(lastChanged)
	if !ok {
		t.Errorf("Expected password change to be allowed after 24h")
	}
}

func TestValidateResetPassword_EmptyToken(t *testing.T) {
	err := ValidateResetPassword(ResetPasswordRequest{
		Token:           "",
		Password:        "StrongPass123!",
		ConfirmPassword: "StrongPass123!",
	})
	if err == nil {
		t.Errorf("Expected error for empty token, got nil")
	}
}

func TestValidateResetPassword_Mismatch(t *testing.T) {
	err := ValidateResetPassword(ResetPasswordRequest{
		Token:           "token",
		Password:        "StrongPass123!",
		ConfirmPassword: "OtherPass123!",
	})
	if err == nil {
		t.Errorf("Expected error for mismatched passwords, got nil")
	}
}

// Password expiration and restricted account tests
func TestGenerateAndVerifyRestrictedJWTToken_Success(t *testing.T) {
	userID := "60d5ec49f1b2c82b8c8e4e01"
	email := "expired_user@example.com"
	role := "RK"

	token, err := GenerateRestrictedJWTToken(userID, email, role)
	if err != nil {
		t.Fatalf("Failed to generate restricted token: %v", err)
	}

	extractedEmail, err := VerifyRestrictedToken(token)
	if err != nil {
		t.Fatalf("Expected valid restricted token, got error: %v", err)
	}

	if extractedEmail != email {
		t.Errorf("Expected extracted email %s, got %s", email, extractedEmail)
	}
}

func TestVerifyRestrictedToken_StandardTokenRejected(t *testing.T) {

	userID := "60d5ec49f1b2c82b8c8e4e01"
	email := "regular_user@example.com"
	role := "RK"

	standardToken, err := GenerateJWTToken(userID, email, role)
	if err != nil {
		t.Fatalf("Failed to generate standard token: %v", err)
	}

	_, err = VerifyRestrictedToken(standardToken)
	if err == nil {
		t.Errorf("Expected VerifyRestrictedToken to reject standard token without restricted claim")
	}
}

func TestValidateChangeExpiredPassword(t *testing.T) {
	// Matching & valid
	if err := ValidateChangeExpiredPassword("NewValidPass123!", "NewValidPass123!"); err != nil {
		t.Errorf("Expected no error for valid matching passwords, got %v", err)
	}

	// Mismatched
	if err := ValidateChangeExpiredPassword("NewValidPass123!", "MismatchPass123!"); err == nil {
		t.Errorf("Expected error for password mismatch, got nil")
	}

	// Weak password
	if err := ValidateChangeExpiredPassword("weak", "weak"); err == nil {
		t.Errorf("Expected error for weak password, got nil")
	}
}

//Magic link tests

func TestMagicLinkToken_HashingAndDeterminism(t *testing.T) {
	rawToken, err := GenerateSecureToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	hash1 := HashResetToken(rawToken)
	hash2 := HashResetToken(rawToken)

	if hash1 != hash2 {
		t.Errorf("Hashes must match for identical raw tokens")
	}

	if len(hash1) != 64 {
		t.Errorf("Expected SHA-256 hash length 64, got %d", len(hash1))
	}
}

func TestMagicLinkToken_DifferentTokensProduceDifferentHashes(t *testing.T) {
	token1, _ := GenerateSecureToken()
	token2, _ := GenerateSecureToken()

	if HashResetToken(token1) == HashResetToken(token2) {
		t.Errorf("Different tokens must produce different hashes")
	}
}

//POWERSHELL EMAIL VERIFICATION  TEST:
//run docker compose up --build -d
// $body = @{
//     firstName       = "Petar"
//     lastName        = "Petrovic"
//     email           = "petar@example.com"
//     password        = "Password123!"
//     confirmPassword = "Password123!"
// } | ConvertTo-Json

// Invoke-RestMethod -Uri "http://localhost:8080/api/users/register" -Method Post -ContentType "application/json" -Body $body

//this sends a verification email to mailpit
