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

	// verification mail slanje
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
