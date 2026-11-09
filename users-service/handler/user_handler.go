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
