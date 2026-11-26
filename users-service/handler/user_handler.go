// Package handler is the transport layer of users-service.
//
// UserHandler implements userspb.UsersServiceServer, the interface generated
// from proto/users/users_service.proto by protoc-gen-go-grpc. Each method
// receives an already-decoded protobuf request (the api-gateway turned the
// REST/JSON call into gRPC) and returns either a protobuf reply or a gRPC
// status error. The gateway maps status codes back to HTTP:
//
//	codes.InvalidArgument   -> 400
//	codes.Unauthenticated   -> 401
//	codes.PermissionDenied  -> 403
//	codes.NotFound          -> 404
//	codes.AlreadyExists     -> 409
//	codes.ResourceExhausted -> 429
//	codes.Internal          -> 500
package handler

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"users-service/domain"
	"users-service/repository"
	"users-service/service"

	userspb "stashr/proto/users"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type UserHandler struct {
	// Embedding the generated Unimplemented server keeps this type compiling
	// (returning codes.Unimplemented) if new rpcs are added to the .proto.
	userspb.UnimplementedUsersServiceServer

	repo              *repository.UserRepository
	smtpHost          string
	smtpPort          string
	smtpFrom          string
	defaultQuotaBytes int64
}

// Compile-time check that UserHandler satisfies the generated interface.
var _ userspb.UsersServiceServer = (*UserHandler)(nil)

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

// VerifyEmail  <-  GET /api/users/verify?token=xyz
// (The old "Accept: text/html -> redirect to frontend" behaviour is an HTTP
// concern and now lives in the api-gateway.)
func (h *UserHandler) VerifyEmail(ctx context.Context, req *userspb.VerifyEmailRequest) (*userspb.MessageResponse, error) {
	token := req.GetToken()
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "Missing token parameter")
	}
	if err := service.ValidateToken(token); err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid or already used verification token")
	}

	res, err := h.repo.VerifyToken(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Internal, "Server error")
	}

	if res.ModifiedCount == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid or already used verification token")
	}

	return &userspb.MessageResponse{
		Message: "Account successfully activated. You can now log in.",
	}, nil
}

// ValidateResetToken  <-  GET /api/users/reset-password?token=xyz
func (h *UserHandler) ValidateResetToken(ctx context.Context, req *userspb.ValidateResetTokenRequest) (*userspb.MessageResponse, error) {
	token := req.GetToken()
	if service.ValidateToken(token) != nil {
		return nil, resetLinkError()
	}

	user, err := h.repo.FindByPasswordResetTokenHash(ctx, service.HashResetToken(token))
	if err != nil || !isResetTokenUsable(user) {
		return nil, resetLinkError()
	}

	ok, retryAt := service.PasswordChangeAllowed(user.PasswordLastChanged)
	if !ok {
		return nil, passwordTooYoungError(retryAt)
	}

	return &userspb.MessageResponse{Message: "Reset link is valid. Submit a new password."}, nil
}

// #endregion

// #region POST

// Register  <-  POST /api/users/register  (HTTP 201)
func (h *UserHandler) Register(ctx context.Context, in *userspb.RegisterRequest) (*userspb.MessageResponse, error) {
	req := service.RegisterRequest{
		FirstName:       in.GetFirstName(),
		LastName:        in.GetLastName(),
		Email:           in.GetEmail(),
		Password:        in.GetPassword(),
		ConfirmPassword: in.GetConfirmPassword(),
	}

	if err := service.ValidateRegistration(&req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// A taken address is reported right away: no account is created and no
	// verification email is sent.
	switch _, err := h.repo.FindByEmail(ctx, req.Email); {
	case err == nil:
		return nil, emailTakenError()
	case !errors.Is(err, mongo.ErrNoDocuments):
		log.Printf("register: email lookup: %v", err)
		return nil, status.Error(codes.Internal, "Error creating account in database")
	}

	passwordHash, err := service.HashPassword(req.Password)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error processing password")
	}

	token, err := service.GenerateSecureToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating token")
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

	if err := h.repo.Create(ctx, newUser); err != nil {
		// Another registration of the same address won the race since the lookup.
		if mongo.IsDuplicateKeyError(err) {
			return nil, emailTakenError()
		}
		return nil, status.Error(codes.Internal, "Error creating account in database")
	}

	h.sendMailAsync("verification", req.Email, func() error {
		return service.SendVerificationEmail(h.smtpHost, h.smtpPort, h.smtpFrom, req.Email, token)
	})

	return registerSuccess(ctx), nil
}

// Login  <-  POST /api/users/login (T-1.2.1 & T-1.2.2 & T-1.2.3)
func (h *UserHandler) Login(ctx context.Context, in *userspb.LoginRequest) (*userspb.AuthResponse, error) {
	email := service.NormalizeEmail(in.GetEmail())
	password := in.GetPassword()

	if err := service.ValidateCredentials(email, password); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByEmail(ctx, email)
	if err != nil {
		service.DummyPasswordCheck(password)
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password")
	}

	if !service.CheckPasswordHash(password, user.PasswordHash) {
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password")
	}

	if !user.IsVerified {
		return nil, status.Error(codes.PermissionDenied, "Account is not verified. Check your email.")
	}

	if !user.AccountLockedUntil.IsZero() {
		if time.Now().Before(user.AccountLockedUntil) {
			return nil, accountLockedError(user.AccountLockedUntil)
		}
		return h.restrictedLogin(user)
	}

	if time.Since(user.PasswordLastChanged) > service.Cfg.PasswordMaxAge {
		return nil, h.lockForExpiredPassword(ctx, user.Email)
	}

	otpCode, err := service.GenerateOTPCode()
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating security code")
	}

	expiresAt := time.Now().Add(service.Cfg.OTPTTL)
	if err := h.repo.SaveOTP(ctx, user.Email, otpCode, expiresAt, true); err != nil {
		return nil, status.Error(codes.Internal, "Error saving security code")
	}

	h.sendMailAsync("otp", user.Email, func() error {
		return service.SendOTPEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, otpCode)
	})

	return &userspb.AuthResponse{
		Message: "First step authentication successful. One-Time Password sent to your email.",
	}, nil
}

// VerifyOTP  <-  POST /api/users/verify-otp (T-1.2.4 & T-1.2.5 & T-1.2.8)
func (h *UserHandler) VerifyOTP(ctx context.Context, in *userspb.VerifyOTPRequest) (*userspb.AuthResponse, error) {
	email := service.NormalizeEmail(in.GetEmail())
	password := in.GetPassword()

	if err := service.ValidateCredentials(email, password); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := service.ValidateOTP(in.GetOtp()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByEmail(ctx, email)
	if err != nil {
		service.DummyPasswordCheck(password)
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password")
	}

	if !service.CheckPasswordHash(password, user.PasswordHash) {
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password")
	}

	if !user.AccountLockedUntil.IsZero() && time.Now().Before(user.AccountLockedUntil) {
		return nil, accountLockedError(user.AccountLockedUntil)
	}

	if user.OTPAttempts >= service.Cfg.OTPMaxAttempts {
		_ = h.repo.ClearOTP(ctx, user.Email)
		return nil, status.Error(codes.ResourceExhausted, "Too many failed attempts. Please log in again to receive a new code.")
	}

	if time.Now().After(user.OTPExpires) || user.OTP == "" {
		_ = h.repo.ClearOTP(ctx, user.Email)
		return nil, status.Error(codes.InvalidArgument, "OTP code has expired or is invalid")
	}

	if user.OTP != in.GetOtp() {
		_ = h.repo.IncrementOTPAttempts(ctx, user.Email)
		return nil, status.Error(codes.Unauthenticated, "Invalid OTP code")
	}

	_ = h.repo.ClearOTP(ctx, user.Email)

	if !user.AccountLockedUntil.IsZero() {
		return h.restrictedLogin(user)
	}

	if time.Since(user.PasswordLastChanged) > service.Cfg.PasswordMaxAge {
		return nil, h.lockForExpiredPassword(ctx, user.Email)
	}

	token, err := service.GenerateJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating session token")
	}

	return &userspb.AuthResponse{
		Message: "Login successful",
		Token:   token,
		User: &userspb.UserInfo{
			Id:        user.ID.Hex(),
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      string(user.Role),
		},
	}, nil
}

// Logout  <-  POST /api/users/logout (T-1.2.7)
// JWTs are stateless; the client discards its token.
func (h *UserHandler) Logout(ctx context.Context, _ *userspb.LogoutRequest) (*userspb.MessageResponse, error) {
	return &userspb.MessageResponse{Message: "Logout successful"}, nil
}

// ResendOTP  <-  POST /api/users/resend-otp (T-1.2.8)
func (h *UserHandler) ResendOTP(ctx context.Context, in *userspb.ResendOTPRequest) (*userspb.MessageResponse, error) {
	email := service.NormalizeEmail(in.GetEmail())
	password := in.GetPassword()

	genericOK := &userspb.MessageResponse{
		Message: "If an account with that email exists, a new code has been sent.",
	}

	if err := service.ValidateCredentials(email, password); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByEmail(ctx, email)
	if err != nil {
		service.DummyPasswordCheck(password)
		return genericOK, nil
	}

	if !service.CheckPasswordHash(password, user.PasswordHash) {
		return nil, status.Error(codes.Unauthenticated, "Invalid email or password")
	}

	if !user.IsVerified {
		return genericOK, nil
	}

	if !user.AccountLockedUntil.IsZero() && time.Now().Before(user.AccountLockedUntil) {
		return nil, accountLockedError(user.AccountLockedUntil)
	}

	now := time.Now()
	resetWindow := user.OTPResendWindowStart.IsZero() || now.Sub(user.OTPResendWindowStart) > service.Cfg.ResetRateLimitWindow
	if !resetWindow && user.OTPResendRequestCount >= service.Cfg.ResetRateLimitMax {
		return genericOK, nil
	}

	if err := h.repo.IncrementWindowedCount(ctx, user.Email, "otpResendRequestCount", "otpResendWindowStart", now, resetWindow); err != nil {
		return nil, status.Error(codes.Internal, "Error processing request")
	}

	otpCode, err := service.GenerateOTPCode()
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating code")
	}

	expiresAt := now.Add(service.Cfg.OTPTTL)
	if err := h.repo.SaveOTP(ctx, user.Email, otpCode, expiresAt, false); err != nil {
		return nil, status.Error(codes.Internal, "Error saving code")
	}

	h.sendMailAsync("otp", user.Email, func() error {
		return service.SendOTPEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, otpCode)
	})

	return genericOK, nil
}

// ForgotPassword  <-  POST /api/users/forgot-password
func (h *UserHandler) ForgotPassword(ctx context.Context, in *userspb.ForgotPasswordRequest) (*userspb.MessageResponse, error) {
	email := service.NormalizeEmail(in.GetEmail())

	genericOK := &userspb.MessageResponse{
		Message: "If an account with that email exists, a password reset link has been sent.",
	}

	if err := service.ValidateEmail(email); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByEmail(ctx, email)
	if err != nil {
		return genericOK, nil
	}

	now := time.Now()
	resetWindow := user.PasswordResetWindowStart.IsZero() || now.Sub(user.PasswordResetWindowStart) > service.Cfg.ResetRateLimitWindow
	if !resetWindow && user.PasswordResetRequestCount >= service.Cfg.ResetRateLimitMax {
		return genericOK, nil
	}

	if err := h.repo.IncrementResetRequestCount(ctx, user.Email, now, resetWindow); err != nil {
		return nil, status.Error(codes.Internal, "Error processing request")
	}

	rawToken, err := service.GenerateSecureToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating token")
	}

	tokenHash := service.HashResetToken(rawToken)
	expiresAt := now.Add(service.Cfg.ResetTokenTTL)
	if err := h.repo.SavePasswordResetToken(ctx, user.Email, tokenHash, expiresAt); err != nil {
		return nil, status.Error(codes.Internal, "Error saving reset token")
	}

	h.sendMailAsync("password-reset", user.Email, func() error {
		return service.SendPasswordResetEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, rawToken)
	})

	return genericOK, nil
}

// ResetPassword  <-  POST /api/users/reset-password
func (h *UserHandler) ResetPassword(ctx context.Context, in *userspb.ResetPasswordRequest) (*userspb.MessageResponse, error) {
	req := service.ResetPasswordRequest{
		Token:           in.GetToken(),
		Password:        in.GetPassword(),
		ConfirmPassword: in.GetConfirmPassword(),
	}

	if service.ValidateToken(req.Token) != nil {
		return nil, resetLinkError()
	}
	if err := service.ValidateResetPassword(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByPasswordResetTokenHash(ctx, service.HashResetToken(req.Token))
	if err != nil || !isResetTokenUsable(user) {
		return nil, resetLinkError()
	}

	ok, retryAt := service.PasswordChangeAllowed(user.PasswordLastChanged)
	if !ok {
		_ = h.repo.ClearPasswordResetToken(ctx, user.Email)
		return nil, passwordTooYoungError(retryAt)
	}

	newHash, err := service.HashPassword(req.Password)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error processing password")
	}

	if err := h.repo.UpdatePassword(ctx, user.Email, newHash, time.Now()); err != nil {
		return nil, status.Error(codes.Internal, "Error updating password")
	}

	return &userspb.MessageResponse{Message: "Password updated. You can now log in."}, nil
}

// ChangeExpiredPassword  <-  POST /api/users/change-expired-password
// Requires the restricted JWT in the "authorization" metadata
// (forwarded by grpc-gateway from the HTTP Authorization header).
func (h *UserHandler) ChangeExpiredPassword(ctx context.Context, in *userspb.ChangeExpiredPasswordRequest) (*userspb.MessageResponse, error) {
	tokenStr, ok := bearerFromMetadata(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Missing or invalid authorization token")
	}

	email, err := service.VerifyRestrictedToken(tokenStr)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	password, confirm := in.GetPassword(), in.GetConfirmPassword()
	if err := service.ValidateChangeExpiredPassword(password, confirm); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "User not found")
	}
	if service.CheckPasswordHash(password, user.PasswordHash) {
		return nil, status.Error(codes.InvalidArgument, "New password must be different from your current password")
	}

	newHash, err := service.HashPassword(password)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error processing password")
	}

	if err := h.repo.UpdatePassword(ctx, email, newHash, time.Now()); err != nil {
		return nil, status.Error(codes.Internal, "Error updating password in database")
	}

	return &userspb.MessageResponse{
		Message: "Password successfully updated. You can now sign in with your new password.",
	}, nil
}

// ChangePassword  <-  POST /api/users/change-password
// A signed-in user (any role) replaces their password. The user comes from the
// session JWT; the current password must be supplied again.
func (h *UserHandler) ChangePassword(ctx context.Context, in *userspb.ChangePasswordRequest) (*userspb.MessageResponse, error) {
	tokenStr, ok := bearerFromMetadata(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Missing or invalid authorization token")
	}
	userID, err := service.VerifyAccountToken(tokenStr)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Invalid user identity in token")
	}

	newPassword := in.GetNewPassword()
	if err := service.ValidateChangePassword(in.GetCurrentPassword(), newPassword, in.GetConfirmPassword()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByID(ctx, oid)
	if err != nil {
		return nil, userLookupError(err)
	}
	// Changing the password clears a lock, so a session that is still open must
	// not be a way to end an active one early.
	if time.Now().Before(user.AccountLockedUntil) {
		return nil, accountLockedError(user.AccountLockedUntil)
	}
	// Not Unauthenticated: the session is fine, only this input is wrong.
	if !service.CheckPasswordHash(in.GetCurrentPassword(), user.PasswordHash) {
		return nil, status.Error(codes.InvalidArgument, "Current password is incorrect")
	}
	if ok, retryAt := service.PasswordChangeAllowed(user.PasswordLastChanged); !ok {
		return nil, passwordTooYoungError(retryAt)
	}

	newHash, err := service.HashPassword(newPassword)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error processing password")
	}
	if err := h.repo.UpdatePassword(ctx, user.Email, newHash, time.Now()); err != nil {
		return nil, status.Error(codes.Internal, "Error updating password")
	}

	return &userspb.MessageResponse{Message: "Password changed successfully."}, nil
}

// GetLimits  <-  GET /api/config/limits (public)
func (h *UserHandler) GetLimits(_ context.Context, _ *userspb.GetLimitsRequest) (*userspb.LimitsResponse, error) {
	l := service.Cfg.Limits
	return &userspb.LimitsResponse{
		MinQuotaBytes:            l.MinQuotaBytes,
		MaxQuotaBytes:            l.MaxQuotaBytes,
		DefaultQuotaBytes:        l.DefaultQuotaBytes,
		MaxUploadBytes:           l.MaxUploadBytes,
		MaxFileNameLength:        int32(l.MaxFileNameLength),
		MaxFolderNameLength:      int32(l.MaxFolderNameLength),
		MaxNameLength:            int32(l.MaxNameLength),
		PasswordMinLength:        int32(l.PasswordMinLength),
		PasswordMaxLength:        int32(l.PasswordMaxLength),
		PasswordRequireUppercase: l.PasswordRequireUppercase,
		PasswordRequireLowercase: l.PasswordRequireLowercase,
		PasswordRequireDigit:     l.PasswordRequireDigit,
		PasswordRequireSpecial:   l.PasswordRequireSpecial,
		AllowedFileExtensions:    l.AllowedExtensions(),

		StorageWarningPercent:        int32(service.Cfg.StorageWarningPercent),
		QuotaRequestHistoryItems:     int32(service.Cfg.QuotaRequestHistoryItems),
		QuotaRequestSuggestionFactor: service.Cfg.QuotaRequestSuggestionFactor,
		AdminQuotaAdjustStepsBytes:   service.Cfg.AdminQuotaAdjustStepsBytes,
		MailUiUrl:                    service.Cfg.MailUIURL,
	}, nil
}

// RequestMagicLink  <-  POST /api/users/magic-link-request
func (h *UserHandler) RequestMagicLink(ctx context.Context, in *userspb.MagicLinkRequest) (*userspb.MessageResponse, error) {
	email := service.NormalizeEmail(in.GetEmail())

	genericOK := &userspb.MessageResponse{
		Message: "If an account with that email exists and is verified, a magic link has been sent.",
	}

	if err := service.ValidateEmail(email); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := h.repo.FindByEmail(ctx, email)
	if err != nil || !user.IsVerified {
		return genericOK, nil
	}

	now := time.Now()
	resetWindow := user.MagicLinkWindowStart.IsZero() || now.Sub(user.MagicLinkWindowStart) > service.Cfg.ResetRateLimitWindow
	if !resetWindow && user.MagicLinkRequestCount >= service.Cfg.ResetRateLimitMax {
		return genericOK, nil
	}

	if err := h.repo.IncrementWindowedCount(ctx, user.Email, "magicLinkRequestCount", "magicLinkWindowStart", now, resetWindow); err != nil {
		return nil, status.Error(codes.Internal, "Error processing request")
	}

	rawToken, err := service.GenerateSecureToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating token")
	}

	expiresAt := now.Add(service.Cfg.MagicLinkTokenTTL)
	if err := h.repo.SaveMagicLinkToken(ctx, user.Email, service.HashResetToken(rawToken), expiresAt); err != nil {
		return nil, status.Error(codes.Internal, "Error saving token")
	}

	h.sendMailAsync("magic-link", user.Email, func() error {
		return service.SendMagicLinkEmail(h.smtpHost, h.smtpPort, h.smtpFrom, user.Email, rawToken)
	})

	return genericOK, nil
}

// MagicLinkLogin  <-  POST /api/users/magic-link-login
func (h *UserHandler) MagicLinkLogin(ctx context.Context, in *userspb.MagicLinkLoginRequest) (*userspb.AuthResponse, error) {
	if in.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "Missing magic link token")
	}
	if service.ValidateToken(in.GetToken()) != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid or expired magic link")
	}

	user, err := h.repo.ConsumeMagicLinkToken(ctx, service.HashResetToken(in.GetToken()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid or expired magic link")
	}

	if !user.AccountLockedUntil.IsZero() {
		if time.Now().Before(user.AccountLockedUntil) {
			return nil, accountLockedError(user.AccountLockedUntil)
		}
		return h.restrictedLogin(user)
	}

	jwtToken, err := service.GenerateJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating authentication token")
	}

	return &userspb.AuthResponse{
		Token: jwtToken,
		User: &userspb.UserInfo{
			Id:    user.ID.Hex(),
			Email: user.Email,
			Role:  string(user.Role),
		},
	}, nil
}

// ListUsers  <-  GET /api/admin/users
func (h *UserHandler) ListUsers(ctx context.Context, _ *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	users, err := h.repo.FindAllUsers(ctx)
	if err != nil {
		log.Printf("list users error: %v", err)
		return nil, status.Error(codes.Internal, "Error loading users")
	}

	resp := &userspb.ListUsersResponse{
		Users: make([]*userspb.AdminUserInfo, 0, len(users)),
	}
	for i := range users {
		resp.Users = append(resp.Users, toAdminUserInfo(&users[i]))
	}
	return resp, nil
}

// UpdateUserQuota  <-  PATCH /api/admin/users/{user_id}/quota
//
// The quota may be raised or lowered anywhere within the configured bounds.
// Lowering it below the current usage is allowed: nothing is deleted, but new
// uploads are refused until the usage fits again.
func (h *UserHandler) UpdateUserQuota(ctx context.Context, in *userspb.UpdateUserQuotaRequest) (*userspb.AdminUserInfo, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	oid, err := primitive.ObjectIDFromHex(in.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid user ID")
	}

	newQuota := in.GetStorageQuotaBytes()
	if !service.Cfg.QuotaInRange(newQuota) {
		return nil, status.Error(codes.InvalidArgument, service.Cfg.QuotaRangeMessage())
	}

	updated, err := h.repo.UpdateStorageQuota(ctx, oid, newQuota)
	if err != nil {
		return nil, userLookupError(err)
	}
	return toAdminUserInfo(updated), nil
}

func requireAdmin(ctx context.Context) error {
	_, err := adminCallerID(ctx)
	return err
}

// adminCallerID verifies that the caller is an administrator and returns their
// user ID.
func adminCallerID(ctx context.Context) (string, error) {
	tokenStr, ok := bearerFromMetadata(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "Missing or invalid authorization token")
	}
	id, err := service.VerifyAdminToken(tokenStr)
	if err != nil {
		if errors.Is(err, service.ErrAdminRequired) {
			return "", status.Error(codes.PermissionDenied, err.Error())
		}
		return "", status.Error(codes.Unauthenticated, err.Error())
	}
	return id, nil
}

func toAdminUserInfo(u *domain.User) *userspb.AdminUserInfo {
	return &userspb.AdminUserInfo{
		Id:                u.ID.Hex(),
		Email:             u.Email,
		FirstName:         u.FirstName,
		LastName:          u.LastName,
		Role:              string(u.Role),
		IsVerified:        u.IsVerified,
		StorageQuotaBytes: u.StorageQuotaBytes,
		UsedStorageBytes:  u.UsedStorageBytes,
	}
}

// #endregion

// #region helperi

// setHTTPStatus attaches gRPC response-header metadata "x-http-code".
// A pure gRPC client ignores it; the api-gateway reads it (see
// api-gateway/response.go) and uses it as the HTTP status code, which is how
// a gRPC call can still produce "201 Created" on the REST side.
func setHTTPStatus(ctx context.Context, code int) {
	if err := grpc.SetHeader(ctx, metadata.Pairs("x-http-code", strconv.Itoa(code))); err != nil {
		log.Printf("failed to set x-http-code header: %v", err)
	}
}

// bearerFromMetadata extracts the JWT from the "authorization" metadata,
// which grpc-gateway fills from the HTTP "Authorization: Bearer ..." header.
func bearerFromMetadata(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	for _, v := range md.Get("authorization") {
		if strings.HasPrefix(v, "Bearer ") && len(v) > len("Bearer ") {
			return strings.TrimPrefix(v, "Bearer "), true
		}
	}
	return "", false
}

func registerSuccess(ctx context.Context) *userspb.MessageResponse {
	setHTTPStatus(ctx, 201)
	return &userspb.MessageResponse{
		Message: "Registration successful. Check your email to verify your account.",
	}
}

func emailTakenError() error {
	return status.Error(codes.AlreadyExists, "An account with this email already exists.")
}

// restrictedLogin issues a short-lived token that only allows changing the
// expired password (ban period over, password must be changed).
func (h *UserHandler) restrictedLogin(user *domain.User) (*userspb.AuthResponse, error) {
	token, err := service.GenerateRestrictedJWTToken(user.ID.Hex(), user.Email, string(user.Role))
	if err != nil {
		return nil, status.Error(codes.Internal, "Error generating session token")
	}
	return &userspb.AuthResponse{
		Message:    "Your ban period has expired. You must change your password.",
		Token:      token,
		Restricted: true,
	}, nil
}

func (h *UserHandler) lockForExpiredPassword(ctx context.Context, email string) error {
	until := time.Now().Add(service.Cfg.AccountLockDuration)
	_ = h.repo.LockAccount(ctx, email, until)
	return status.Error(codes.PermissionDenied, "Password expired. Account locked until "+until.Format(time.RFC3339))
}

func accountLockedError(until time.Time) error {
	return status.Error(codes.PermissionDenied, "Account is locked until "+until.Format(time.RFC3339))
}

func (h *UserHandler) sendMailAsync(kind, to string, send func() error) {
	go func() {
		if err := send(); err != nil {
			log.Printf("failed to send %s email to %s: %v", kind, to, err)
		}
	}()
}

func resetLinkError() error {
	return status.Error(codes.InvalidArgument,
		"This password reset link is invalid or has expired. You can request a new one via forgot password.")
}

func passwordTooYoungError(retryAt time.Time) error {
	retry := retryAt.UTC().Format(time.RFC3339)
	return status.Error(codes.PermissionDenied,
		"Password can only be changed once every "+service.HumanDur(service.Cfg.PasswordMinAge)+". You can retry after "+retry+".")
}

func isResetTokenUsable(user *domain.User) bool {
	return user != nil &&
		user.PasswordResetTokenHash != "" &&
		time.Now().Before(user.PasswordResetTokenExpires)
}

// #endregion
