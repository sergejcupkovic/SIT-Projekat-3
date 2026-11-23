package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"users-service/config"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxNameLen    = 100
	maxEmailLen   = 254
	smtpTimeout   = 10 * time.Second
	dummyPassword = "dummy-password-not-a-real-user"
)

var (
	JWTSecret  []byte
	Cfg        *config.Config
	dummyHash  []byte
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	nameRegex  = regexp.MustCompile(`^\p{L}[\p{L} '\-]*$`)
)

func Init(c *config.Config) {
	Cfg = c
	JWTSecret = []byte(c.JWTSecret)
	hash, err := bcrypt.GenerateFromPassword([]byte(dummyPassword), c.BcryptCost)
	if err != nil {
		panic("failed to generate dummy password hash: " + err.Error())
	}
	dummyHash = hash
}

// #region url-ovi
// Links in emails point at the public HTTPS endpoints (gateway / frontend).
func gatewayURL() string {
	return fmt.Sprintf("https://%s:%s", Cfg.PublicHost, Cfg.GatewayPort)
}

func frontendURL() string {
	return fmt.Sprintf("https://%s:%s", Cfg.PublicHost, Cfg.FrontendPort)
}

// #endregion

func HumanDur(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%d hour(s)", int(d/time.Hour))
	case d%time.Minute == 0:
		return fmt.Sprintf("%d minute(s)", int(d/time.Minute))
	default:
		return d.String()
	}
}

// #region struct-ovi
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
	Email    string `json:"email"`
	Password string `json:"password"`
	OTP      string `json:"otp"`
}

type ResendOTPRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// #endregion

// #region validacija
func ValidateRegistration(req *RegisterRequest) error {
	if req == nil {
		return errors.New("all fields are required")
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.FirstName == "" || req.LastName == "" || req.Email == "" {
		return errors.New("all fields are required")
	}
	if utf8.RuneCountInString(req.FirstName) > maxNameLen || utf8.RuneCountInString(req.LastName) > maxNameLen {
		return errors.New("name is too long")
	}
	if !nameRegex.MatchString(req.FirstName) || !nameRegex.MatchString(req.LastName) {
		return errors.New("name may only contain letters, spaces, apostrophes and hyphens")
	}
	if len(req.Email) > maxEmailLen {
		return errors.New("email is too long")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	if !emailRegex.MatchString(req.Email) {
		return errors.New("invalid email address format")
	}

	return ValidatePassword(req.Password)
}

func ValidatePassword(p string) error {
	if len(p) < 8 || len(p) > 72 {
		return errors.New("password must be 8-72 characters long")
	}
	var up, low, digit, special bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			up = true
		case unicode.IsLower(r):
			low = true
		case r >= '0' && r <= '9':
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}
	if !(up && low && digit && special) {
		return errors.New("password must contain an uppercase letter, a lowercase letter, a digit, and a special character")
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

func ValidateResetPassword(req ResetPasswordRequest) error {
	if req.Token == "" {
		return errors.New("token is required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	return ValidatePassword(req.Password)
}

// #endregion

// #region generatori
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
		"exp":   time.Now().Add(Cfg.JWTTTL).Unix(),
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
		"exp":        time.Now().Add(Cfg.RestrictedTokenTTL).Unix(),
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

// #endregion

// #region hashovanje
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), Cfg.BcryptCost)
	return string(bytes), err
}

func HashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func DummyPasswordCheck(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// #endregion

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

func PasswordChangeAllowed(lastChanged time.Time) (ok bool, retryAt time.Time) {
	if time.Since(lastChanged) < Cfg.PasswordMinAge {
		return false, lastChanged.Add(Cfg.PasswordMinAge)
	}
	return true, time.Time{}
}

// #region slanje mejlova
func sendMail(host, port, from, to, subject, body string) error {
	msg := fmt.Appendf(nil, "To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body)
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, smtpTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(smtpTimeout)); err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func SendPasswordResetEmail(host, port, from, to, rawToken string) error {
	link := fmt.Sprintf("%s/api/users/reset-password?token=%s", gatewayURL(), rawToken)
	body := fmt.Sprintf("Click the link to reset your password: %s\r\nThis link will expire in %s and can be used only once.", link, HumanDur(Cfg.ResetTokenTTL))
	return sendMail(host, port, from, to, "Password Reset Request", body)
}

func SendMagicLinkEmail(host, port, from, to, rawToken string) error {
	link := fmt.Sprintf("%s/?magicToken=%s", frontendURL(), rawToken)
	body := fmt.Sprintf("Click the link to login into your account: %s\r\nThis link will expire in %s and can be used only once.", link, HumanDur(Cfg.MagicLinkTokenTTL))
	return sendMail(host, port, from, to, "Magic Link", body)
}

func SendVerificationEmail(smtpHost, smtpPort, from, to, token string) error {
	verifyLink := fmt.Sprintf("%s/?token=%s", frontendURL(), token)
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: Registration Confirmation\r\n\r\nClick the link to activate your account: %s\r\n", to, verifyLink))
	return smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{to}, msg)
}

func SendOTPEmail(host, port, from, to, otpCode string) error {
	body := fmt.Sprintf("Your single-use login code is: %s\r\nThis code will expire in %s.", otpCode, HumanDur(Cfg.OTPTTL))
	return sendMail(host, port, from, to, "Your One-Time Login Password (OTPS)", body)
}

// #endregion
