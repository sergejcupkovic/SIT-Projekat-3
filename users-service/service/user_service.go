package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/smtp"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var JWTSecret = []byte("super-secret-key-change-in-prod")

type RegisterRequest struct {
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type VerifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func ValidateRegistration(req RegisterRequest) error {
	if req.FirstName == "" || req.LastName == "" || req.Email == "" {
		return errors.New("all fields are required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email address format")
	}

	if len(req.Password) < 8 ||
		!regexp.MustCompile(`[A-Z]`).MatchString(req.Password) ||
		!regexp.MustCompile(`[a-z]`).MatchString(req.Password) ||
		!regexp.MustCompile(`[0-9]`).MatchString(req.Password) ||
		!regexp.MustCompile(`[!@#~$%^&*()+|_.,<>?/{}\-]`).MatchString(req.Password) {
		return errors.New("password must have at least 8 characters, an uppercase and lowercase letter, a number, and a special character")
	}
	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateOTPCode() (string, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", nBig.Int64()+100000), nil
}

func GenerateJWTToken(userID string, email string, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"role":  role,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func GenerateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func SendVerificationEmail(smtpHost, smtpPort, from, to, token string) error {
	verifyLink := fmt.Sprintf("http://localhost:8080/api/users/verify?token=%s", token)
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: Registration Confirmation\r\n\r\nClick the link to activate your account: %s\r\n", to, verifyLink))
	return smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{to}, msg)
}

func SendOTPEmail(smtpHost, smtpPort, from, to, otpCode string) error {
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: Your One-Time Login Password (OTPS)\r\n\r\nYour single-use login code is: %s\r\nThis code will expire in 5 minutes.\r\n", to, otpCode))
	return smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{to}, msg)
}
