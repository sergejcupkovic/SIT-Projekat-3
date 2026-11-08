package service

import (
	"testing"

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
