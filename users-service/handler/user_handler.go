package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"users-service/domain"
	"users-service/repository"
	"users-service/service"

	"go.mongodb.org/mongo-driver/mongo"
)

const maxBodyBytes = 64 << 10

type UserHandler struct {
	repo              *repository.UserRepository
	smtpHost          string
	smtpPort          string
	smtpFrom          string
	defaultQuotaBytes int64
}

func NewUserHandler(repo *repository.UserRepository, smtpHost, smtpPort, smtpFrom string, quotaBytes int64) *UserHandler {
	return &UserHandler{
		repo:              repo,
		smtpHost:          smtpHost,
		smtpPort:          smtpPort,
		smtpFrom:          smtpFrom,
		defaultQuotaBytes: quotaBytes,
	}
}

// #region GET
// GET /api/users/verify?token=xyz
func (h *UserHandler) Verify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token parameter", http.StatusBadRequest)
		return
	}

	// Ako zahtev dolazi direktno iz brauzera (klik na stari API link), preusmeri na frontend UI
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "http://localhost:3000/?token="+url.QueryEscape(token), http.StatusFound)
		return
	}

	res, err := h.repo.VerifyToken(r.Context(), token)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	if res.ModifiedCount == 0 {
		http.Error(w, "Invalid or already used verification token", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Account successfully activated. You can now log in.",
	})
}

// GET /api/users/reset-password?token=xyz
func (h *UserHandler) getResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeResetLinkError(w)
		return
	}

	user, err := h.repo.FindByPasswordResetTokenHash(r.Context(), service.HashResetToken(token))
	if err != nil || !isResetTokenUsable(user) {
		writeResetLinkError(w)
		return
	}

	ok, retryAt := service.PasswordChangeAllowed(user.PasswordLastChanged)
	if !ok {
		writePasswordTooYoung(w, retryAt)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Reset link is valid. Submit a new password.",
	})
}

// #endregion

// GET/POST /api/users/reset-password
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getResetPassword(w, r)
	case http.MethodPost:
		h.postResetPassword(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// #region POST
// POST /api/users/register
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := service.ValidateRegistration(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	passwordHash, err := service.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	token, err := service.GenerateSecureToken()
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	newUser := &domain.User{
		FirstName:                req.FirstName,
		LastName:                 req.LastName,
		Email:                    req.Email,
		PasswordHash:             passwordHash,
		Role:                     domain.RoleRegular,
		IsVerified:               false,
		VerificationToken:        token,
		VerificationTokenExpires: now.Add(service.Cfg.VerificationTokenTTL),
		StorageQuotaBytes:        h.defaultQuotaBytes,
		UsedStorageBytes:         0,
		PasswordLastChanged:      now,
		CreatedAt:                now,
	}

	err = h.repo.Create(r.Context(), newUser)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeRegisterSuccess(w)
			return
		}
		http.Error(w, "Error creating account in database", http.StatusInternalServerError)
		return
	}

	h.sendMailAsync("verification", req.Email, func() error {
		return service.SendVerificationEmail(h.smtpHost, h.smtpPort, h.smtpFrom, req.Email, token)
	})

	writeRegisterSuccess(w)
}

// POST /api/users/login (T-1.2.1 & T-1.2.2 & T-1.2.3)
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = service.NormalizeEmail(req.Email)

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		service.DummyPasswordCheck(req.Password)
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !service.CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !user.IsVerified {
		http.Error(w, "Account is not verified. Check your email.", http.StatusForbidden)
		return
	}

	if !user.AccountLockedUntil.IsZero() {
		if time.Now().Before(user.AccountLockedUntil) {
			http.Error(w, "Account is locked until "+user.AccountLockedUntil.Format(time.RFC3339), http.StatusForbidden)
			return
		}

		token, err := service.GenerateRestrictedJWTToken(user.ID.Hex(), user.Email, string(user.Role))
		if err != nil {
			http.Error(w, "Error generating session token", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":    "Your ban period has expired. You must change your password.",
			"token":      token,
			"restricted": true,
		})
		return
	}

	if time.Since(user.PasswordLastChanged) > service.Cfg.PasswordMaxAge {
		until := time.Now().Add(service.Cfg.AccountLockDuration)
		_ = h.repo.LockAccount(r.Context(), user.Email, until)
		http.Error(w, "Password expired. Account locked until "+until.Format(time.RFC3339),
			http.StatusForbidden)
		return
	}

	otpCode, err := service.GenerateOTPCode()
	if err != nil {
		http.Error(w, "Error generating security code", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(service.Cfg.OTPTTL)
	if err := h.repo.SaveOTP(r.Context(), user.Email, otpCode, expiresAt, true); err != nil {
		http.Error(w, "Error saving security code", http.StatusInternalServerError)
		return
	}

	h.sendMailAsync("otp", user.Email, func() error {
		return service.SendOTPEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, otpCode)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "First step authentication successful. One-Time Password sent to your email.",
	})
}

// POST /api/users/verify-otp (T-1.2.4 & T-1.2.5 & T-1.2.8)
func (h *UserHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.VerifyOTPRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = service.NormalizeEmail(req.Email)

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		service.DummyPasswordCheck(req.Password)
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !service.CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !user.AccountLockedUntil.IsZero() && time.Now().Before(user.AccountLockedUntil) {
		http.Error(w, "Account is locked until "+user.AccountLockedUntil.Format(time.RFC3339), http.StatusForbidden)
		return
	}

	if user.OTPAttempts >= service.Cfg.OTPMaxAttempts {
		_ = h.repo.ClearOTP(r.Context(), user.Email)
		http.Error(w, "Too many failed attempts. Please log in again to receive a new code.", http.StatusTooManyRequests)
		return
	}

	if time.Now().After(user.OTPExpires) || user.OTP == "" {
		_ = h.repo.ClearOTP(r.Context(), user.Email)
		http.Error(w, "OTP code has expired or is invalid", http.StatusBadRequest)
		return
	}

	if user.OTP != req.OTP {
		_ = h.repo.IncrementOTPAttempts(r.Context(), user.Email)
		http.Error(w, "Invalid OTP code", http.StatusUnauthorized)
		return
	}

	_ = h.repo.ClearOTP(r.Context(), user.Email)

	if !user.AccountLockedUntil.IsZero() {
		token, err := service.GenerateRestrictedJWTToken(user.ID.Hex(), user.Email, string(user.Role))
		if err != nil {
			http.Error(w, "Error generating session token", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"message":    "Your ban period has expired. You must change your password.",
			"token":      token,
			"restricted": true,
		})
		return
	}

	if time.Since(user.PasswordLastChanged) > service.Cfg.PasswordMaxAge {
		until := time.Now().Add(service.Cfg.AccountLockDuration)
		_ = h.repo.LockAccount(r.Context(), user.Email, until)
		http.Error(w, "Password expired. Account locked until "+until.Format(time.RFC3339), http.StatusForbidden)
		return
	}

	token, err := service.GenerateJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		http.Error(w, "Error generating session token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "Login successful",
		"token":   token,
		"user": map[string]string{
			"id":        user.ID.Hex(),
			"email":     user.Email,
			"firstName": user.FirstName,
			"lastName":  user.LastName,
			"role":      string(user.Role),
		},
	})
}

// POST /api/users/logout (T-1.2.7)
func (h *UserHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Logout successful",
	})
}

// POST /api/users/resend-otp (T-1.2.8)
func (h *UserHandler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.ResendOTPRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = service.NormalizeEmail(req.Email)

	const genericMsg = "If an account with that email exists, a new code has been sent."
	genericOK := func() {
		writeJSON(w, http.StatusOK, map[string]any{"message": genericMsg})
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		service.DummyPasswordCheck(req.Password)
		genericOK()
		return
	}

	if !service.CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !user.IsVerified {
		genericOK()
		return
	}

	if !user.AccountLockedUntil.IsZero() && time.Now().Before(user.AccountLockedUntil) {
		http.Error(w, "Account is locked until "+user.AccountLockedUntil.Format(time.RFC3339), http.StatusForbidden)
		return
	}

	now := time.Now()
	resetWindow := user.OTPResendWindowStart.IsZero() || now.Sub(user.OTPResendWindowStart) > service.Cfg.ResetRateLimitWindow
	if !resetWindow && user.OTPResendRequestCount >= service.Cfg.ResetRateLimitMax {
		genericOK()
		return
	}

	if err := h.repo.IncrementWindowedCount(r.Context(), user.Email, "otpResendRequestCount", "otpResendWindowStart", now, resetWindow); err != nil {
		http.Error(w, "Error processing request", http.StatusInternalServerError)
		return
	}

	otpCode, err := service.GenerateOTPCode()
	if err != nil {
		http.Error(w, "Error generating code", http.StatusInternalServerError)
		return
	}

	expiresAt := now.Add(service.Cfg.OTPTTL)
	if err := h.repo.SaveOTP(r.Context(), user.Email, otpCode, expiresAt, false); err != nil {
		http.Error(w, "Error saving code", http.StatusInternalServerError)
		return
	}

	h.sendMailAsync("otp", user.Email, func() error {
		return service.SendOTPEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, otpCode)
	})

	genericOK()
}

// POST /api/users/forgot-password
func (h *UserHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.ForgotPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = service.NormalizeEmail(req.Email)

	genericOK := func() {
		writeJSON(w, http.StatusOK, map[string]any{
			"message": "If an account with that email exists, a password reset link has been sent.",
		})
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		genericOK()
		return
	}

	now := time.Now()
	resetWindow := user.PasswordResetWindowStart.IsZero() || now.Sub(user.PasswordResetWindowStart) > service.Cfg.ResetRateLimitWindow
	if !resetWindow && user.PasswordResetRequestCount >= service.Cfg.ResetRateLimitMax {
		genericOK()
		return
	}

	if err := h.repo.IncrementResetRequestCount(r.Context(), user.Email, now, resetWindow); err != nil {
		http.Error(w, "Error processing request", http.StatusInternalServerError)
		return
	}

	rawToken, err := service.GenerateSecureToken()
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	tokenHash := service.HashResetToken(rawToken)
	expiresAt := now.Add(service.Cfg.ResetTokenTTL)
	if err := h.repo.SavePasswordResetToken(r.Context(), user.Email, tokenHash, expiresAt); err != nil {
		http.Error(w, "Error saving reset token", http.StatusInternalServerError)
		return
	}

	h.sendMailAsync("password-reset", user.Email, func() error {
		return service.SendPasswordResetEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, rawToken)
	})

	genericOK()
}

// POST /api/users/reset-password
func (h *UserHandler) postResetPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ResetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := service.ValidateResetPassword(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := h.repo.FindByPasswordResetTokenHash(r.Context(), service.HashResetToken(req.Token))
	if err != nil || !isResetTokenUsable(user) {
		writeResetLinkError(w)
		return
	}

	ok, retryAt := service.PasswordChangeAllowed(user.PasswordLastChanged)
	if !ok {
		_ = h.repo.ClearPasswordResetToken(r.Context(), user.Email)
		writePasswordTooYoung(w, retryAt)
		return
	}

	newHash, err := service.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	if err := h.repo.UpdatePassword(r.Context(), user.Email, newHash, time.Now()); err != nil {
		http.Error(w, "Error updating password", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Password updated. You can now log in.",
	})
}

// POST /api/users/change-expired-password
func (h *UserHandler) ChangeExpiredPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		http.Error(w, "Missing or invalid authorization token", http.StatusUnauthorized)
		return
	}
	tokenStr := authHeader[7:]

	email, err := service.VerifyRestrictedToken(tokenStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := service.ValidateChangeExpiredPassword(req.Password, req.ConfirmPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := h.repo.FindByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	if service.CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "New password must be different from your current password", http.StatusBadRequest)
		return
	}

	newHash, err := service.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	if err := h.repo.UpdatePassword(r.Context(), email, newHash, now); err != nil {
		http.Error(w, "Error updating password in database", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Password successfully updated. You can now sign in with your new password.",
	})
}

// POST /api/users/magic-link-request
func (h *UserHandler) RequestMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}

	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = service.NormalizeEmail(req.Email)

	const genericMsg = "If an account with that email exists and is verified, a magic link has been sent."
	genericOK := func() {
		writeJSON(w, http.StatusOK, map[string]any{"message": genericMsg})
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil || !user.IsVerified {
		genericOK()
		return
	}

	now := time.Now()
	resetWindow := user.MagicLinkWindowStart.IsZero() || now.Sub(user.MagicLinkWindowStart) > service.Cfg.ResetRateLimitWindow
	if !resetWindow && user.MagicLinkRequestCount >= service.Cfg.ResetRateLimitMax {
		genericOK()
		return
	}

	if err := h.repo.IncrementWindowedCount(r.Context(), user.Email, "magicLinkRequestCount", "magicLinkWindowStart", now, resetWindow); err != nil {
		http.Error(w, "Error processing request", http.StatusInternalServerError)
		return
	}

	rawToken, err := service.GenerateSecureToken()
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	expiresAt := now.Add(service.Cfg.MagicLinkTokenTTL)
	if err := h.repo.SaveMagicLinkToken(r.Context(), user.Email, service.HashResetToken(rawToken), expiresAt); err != nil {
		http.Error(w, "Error saving token", http.StatusInternalServerError)
		return
	}

	h.sendMailAsync("magic-link", user.Email, func() error {
		return service.SendMagicLinkEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, rawToken)
	})

	genericOK()
}

// POST /api/users/magic-link-login
func (h *UserHandler) GetMagicLinkLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Token == "" {
		http.Error(w, "Missing magic link token", http.StatusBadRequest)
		return
	}

	user, err := h.repo.ConsumeMagicLinkToken(r.Context(), service.HashResetToken(req.Token))
	if err != nil {
		http.Error(w, "Invalid or expired magic link", http.StatusBadRequest)
		return
	}

	if !user.AccountLockedUntil.IsZero() {
		if time.Now().Before(user.AccountLockedUntil) {
			http.Error(w, "Account is locked until "+user.AccountLockedUntil.Format(time.RFC3339), http.StatusForbidden)
			return
		}

		restrictedToken, err := service.GenerateRestrictedJWTToken(user.ID.Hex(), user.Email, string(user.Role))
		if err != nil {
			http.Error(w, "Error generating session token", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"message":    "Your ban period has expired. You must change your password.",
			"token":      restrictedToken,
			"restricted": true,
		})
		return
	}
	jwtToken, err := service.GenerateJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		http.Error(w, "Error generating authentication token", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": jwtToken,
		"user": map[string]any{
			"id":    user.ID.Hex(),
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

// #endregion

// #region helperi
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}

func writeRegisterSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Registration successful. Check your email to verify your account.",
	})
}

func (h *UserHandler) sendMailAsync(kind, to string, send func() error) {
	go func() {
		if err := send(); err != nil {
			log.Printf("failed to send %s email to %s: %v", kind, to, err)
		}
	}()
}

func writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeResetLinkError(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"message":            "This password reset link is invalid or has expired.",
		"canRequestNewLink":  true,
		"forgotPasswordPath": "/api/users/forgot-password",
	})
}

func writePasswordTooYoung(w http.ResponseWriter, retryAt time.Time) {
	retry := retryAt.UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusForbidden, map[string]any{
		"message":            "Password can only be changed once every " + service.HumanDur(service.Cfg.PasswordMinAge) + ". You can retry after " + retry + ".",
		"retryAt":            retry,
		"canRequestNewLink":  true,
		"forgotPasswordPath": "/api/users/forgot-password",
	})
}

func isResetTokenUsable(user *domain.User) bool {
	return user != nil &&
		user.PasswordResetTokenHash != "" &&
		time.Now().Before(user.PasswordResetTokenExpires)
}

// #endregion
