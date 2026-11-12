package service

import (
	"crypto/rand"
	"crypto/sha256"
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

const (
	PasswordMinAge       = 1 * time.Minute
	PasswordMaxAge       = 2 * time.Minute
	ResetTokenTTL        = 30 * time.Minute
	ResetRateLimitMax    = 3
	ResetRateLimitWindow = 15 * time.Minute
)

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

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
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

	return ValidatePassword(req.Password)
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

func GenerateRestrictedJWTToken(userID, email, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":        userID,
		"email":      email,
		"role":       role,
		"restricted": true,
		"exp":        time.Now().Add(15 * time.Minute).Unix(), //15min to change the password
		"iat":        time.Now().Unix(),
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

func ValidatePassword(password string) error {
	if len(password) < 8 ||
		!regexp.MustCompile(`[A-Z]`).MatchString(password) ||
		!regexp.MustCompile(`[a-z]`).MatchString(password) ||
		!regexp.MustCompile(`[0-9]`).MatchString(password) ||
		!regexp.MustCompile(`[!@#~$%^&*()+|_.,<>?/{}\-]`).MatchString(password) {
		return errors.New("password must have at least 8 characters, an uppercase and lowercase letter, a number, and a special character")
	}
	return nil
}

func ValidateChangeExpiredPassword(newPassword, confirmPassword string) error {
	if newPassword == "" {
		return errors.New("password is required")
	}
	if newPassword != confirmPassword {
		return errors.New("passwords do not match")
	}
	return ValidatePassword(newPassword)
}

func VerifyRestrictedToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return JWTSecret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid or expired token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	isRestricted, ok := claims["restricted"].(bool)
	if !ok || !isRestricted {
		return "", errors.New("token is not authorized for password change")
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return "", errors.New("email not found in token")
	}

	return email, nil
}

func ValidateResetPassword(req ResetPasswordRequest) error {
	if req.Token == "" {
		return errors.New("token is required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	return ValidatePassword(req.Password)
}

func HashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func PasswordChangeAllowed(lastChanged time.Time) (ok bool, retryAt time.Time) {
	if time.Since(lastChanged) < PasswordMinAge {
		return false, lastChanged.Add(PasswordMinAge)
	}
	return true, time.Time{}
}

func SendPasswordResetEmail(smtpHost, smtpPort, from, to, rawToken string) error {
	resetLink := fmt.Sprintf("http://localhost:8080/api/users/reset-password?token=%s", rawToken)
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: Password Reset Request\r\n\r\nClick the link to reset your password: %s\r\nThis link will expire in 30 minutes and can be used only once.\r\n", to, resetLink))
	return smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{to}, msg)
}
