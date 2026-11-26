package service

import (
	"errors"
	"regexp"
)

var (
	otpRegex   = regexp.MustCompile(`^[0-9]{6}$`)
	tokenRegex = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// ValidateEmail expects an already normalized (trimmed, lower case) address.
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > maxEmailLen {
		return errors.New("email is too long")
	}
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email address format")
	}
	return nil
}

func ValidateCredentials(email, password string) error {
	if err := ValidateEmail(email); err != nil {
		return err
	}
	if password == "" {
		return errors.New("password is required")
	}
	if len(password) > 72 {
		return errors.New("password is too long")
	}
	return nil
}

func ValidateOTP(otp string) error {
	if !otpRegex.MatchString(otp) {
		return errors.New("one-time password must be exactly 6 digits")
	}
	return nil
}

func ValidateToken(token string) error {
	if !tokenRegex.MatchString(token) {
		return errors.New("malformed token")
	}
	return nil
}
