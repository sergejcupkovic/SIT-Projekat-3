package handler

import (
	"encoding/json"
	"net/http"
	"time"
	"users-service/domain"
	"users-service/repository"
	"users-service/service"

	"go.mongodb.org/mongo-driver/mongo"
)

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

// POST /api/users/register
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
		return
	}

	// input validacija
	if err := service.ValidateRegistration(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//pass hashing
	passwordHash, err := service.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	// verification toke
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
		VerificationTokenExpires: now.Add(24 * time.Hour),
		StorageQuotaBytes:        h.defaultQuotaBytes,
		UsedStorageBytes:         0,
		PasswordLastChanged:      now,
		CreatedAt:                now,
	}

	// upsivanej u bazu
	err = h.repo.Create(r.Context(), newUser)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			http.Error(w, "A user with this email address already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Error creating account in database", http.StatusInternalServerError)
		return
	}

	go func() {
		_ = service.SendVerificationEmail(h.smtpHost, h.smtpPort, h.smtpFrom, req.Email, token)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Registration successful. Check your email to verify your account.",
	})
}

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

// POST /api/users/login (T-1.2.1 & T-1.2.2 & T-1.2.3)
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !user.IsVerified {
		http.Error(w, "Account is not verified. Check your email.", http.StatusForbidden)
		return
	}

	if !service.CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if !user.AccountLockedUntil.IsZero() {
		if time.Now().Before(user.AccountLockedUntil) {
			// Ban active
			http.Error(w, "Account is locked until "+user.AccountLockedUntil.Format(time.RFC3339), http.StatusForbidden)
			return
		}

		//ban expired so it sends restricted session token
		token, err := service.GenerateRestrictedJWTToken(user.ID.Hex(), user.Email, string(user.Role))
		if err != nil {
			http.Error(w, "Error generating session token", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message":    "Your ban period has expired. You must change your password.",
			"token":      token,
			"restricted": true,
		})
		return
	}

	if time.Since(user.PasswordLastChanged) > service.PasswordMaxAge {
		until := time.Now().Add(1 * time.Minute) //set how long the account is locked for, juist one min rn
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

	expiresAt := time.Now().Add(5 * time.Minute)
	if err := h.repo.SaveOTP(r.Context(), user.Email, otpCode, expiresAt); err != nil {
		http.Error(w, "Error saving security code", http.StatusInternalServerError)
		return
	}

	go func() {
		_ = service.SendOTPEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, otpCode)
	}()

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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.OTPAttempts >= 3 {
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

	token, err := service.GenerateJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		http.Error(w, "Error generating session token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
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

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil || !user.IsVerified {
		http.Error(w, "User not found or unverified", http.StatusBadRequest)
		return
	}

	otpCode, err := service.GenerateOTPCode()
	if err != nil {
		http.Error(w, "Error generating code", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	if err := h.repo.SaveOTP(r.Context(), user.Email, otpCode, expiresAt); err != nil {
		http.Error(w, "Error saving code", http.StatusInternalServerError)
		return
	}

	go func() {
		_ = service.SendOTPEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, otpCode)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "New OTP code sent to your email.",
	})
}

func writeJSON(w http.ResponseWriter, status int, body map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeResetLinkError(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{
		"message":            "This password reset link is invalid or has expired.",
		"canRequestNewLink":  true,
		"forgotPasswordPath": "/api/users/forgot-password",
	})
}

func writePasswordTooYoung(w http.ResponseWriter, retryAt time.Time) {
	retry := retryAt.UTC().Format(time.RFC3339)
	writeJSON(w, http.StatusForbidden, map[string]interface{}{
		"message":            "Password can only be changed once every 24 hours. You can retry after " + retry + ".",
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

const forgotPasswordGenericMessage = "If an account with that email exists, a password reset link has been sent."

// POST /api/users/forgot-password
func (h *UserHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	genericOK := func() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": forgotPasswordGenericMessage,
		})
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		genericOK()
		return
	}

	now := time.Now()
	resetWindow := user.PasswordResetWindowStart.IsZero() || now.Sub(user.PasswordResetWindowStart) > service.ResetRateLimitWindow
	if !resetWindow && user.PasswordResetRequestCount >= service.ResetRateLimitMax {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"message": "Too many password reset requests. Try again later.",
		})
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
	expiresAt := now.Add(service.ResetTokenTTL)
	if err := h.repo.SavePasswordResetToken(r.Context(), user.Email, tokenHash, expiresAt); err != nil {
		http.Error(w, "Error saving reset token", http.StatusInternalServerError)
		return
	}

	go func() {
		_ = service.SendPasswordResetEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, rawToken)
	}()

	genericOK()
}

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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Reset link is valid. Submit a new password.",
	})
}

func (h *UserHandler) postResetPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Password updated. You can now log in.",
	})
}

// POST /api/users/change-expired-password (Task 3)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := service.ValidateChangeExpiredPassword(req.Password, req.ConfirmPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	const genericMsg = "If an account with that email exists and is verified, a magic link has been sent."
	genericOK := func() {
		writeJSON(w, http.StatusOK, map[string]interface{}{"message": genericMsg})
	}

	user, err := h.repo.FindByEmail(r.Context(), req.Email)
	if err != nil || !user.IsVerified {
		genericOK()
		return
	}

	rawToken, err := service.GenerateSecureToken()
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(service.MagicLinkTokenTTL)
	if err := h.repo.SaveMagicLinkToken(r.Context(), user.Email, service.HashResetToken(rawToken), expiresAt); err != nil {
		http.Error(w, "Error saving token", http.StatusInternalServerError)
		return
	}

	go func() {
		_ = service.SendMagicLinkEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, rawToken)
	}()

	genericOK()
}

// POST /api/users/magic-link-login (or GET query fallback)
func (h *UserHandler) GetMagicLinkLogin(w http.ResponseWriter, r *http.Request) {
	var token string
	if r.Method == http.MethodPost {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			token = req.Token
		}
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	if token == "" {
		http.Error(w, "Missing magic link token", http.StatusBadRequest)
		return
	}

	tokenHash := service.HashResetToken(token)

	user, err := h.repo.FindByMagicLinkToken(r.Context(), tokenHash)
	if err != nil {

		http.Error(w, "Invalid or expired magic link", http.StatusBadRequest)
		return
	}

	if err := h.repo.ClearMagicLinkToken(r.Context(), user.Email); err != nil {
		http.Error(w, "Error consuming magic link", http.StatusInternalServerError)
		return
	}

	jwtToken, err := service.GenerateJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		http.Error(w, "Error generating authentication token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": jwtToken,
		"user": map[string]interface{}{
			"id":    user.ID.Hex(),
			"email": user.Email,
			"role":  user.Role,
		},
	})
}
