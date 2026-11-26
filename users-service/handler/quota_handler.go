package handler

import (
	"context"
	"errors"
	"log"

	"users-service/domain"
	"users-service/repository"
	"users-service/service"

	userspb "stashr/proto/users"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Internal rpcs for storage-service; they only touch the quota of the user in the token.
type QuotaHandler struct {
	userspb.UnimplementedQuotaServiceServer

	store *repository.UserRepository
}

var _ userspb.QuotaServiceServer = (*QuotaHandler)(nil)

func NewQuotaHandler(store *repository.UserRepository) *QuotaHandler {
	return &QuotaHandler{store: store}
}

func (h *QuotaHandler) GetQuota(ctx context.Context, _ *userspb.GetQuotaRequest) (*userspb.QuotaResponse, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.store.FindByID(ctx, userID)
	if err != nil {
		return nil, userLookupError(err)
	}
	return quotaResponse(user, true), nil
}

func (h *QuotaHandler) ReserveQuota(ctx context.Context, req *userspb.ReserveQuotaRequest) (*userspb.QuotaResponse, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetSizeBytes() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "size_bytes must be greater than 0")
	}

	updated, allowed, err := h.store.ReserveStorageQuota(ctx, userID, req.GetSizeBytes())
	if err != nil {
		log.Printf("reserve quota for %s: %v", userID.Hex(), err)
		return nil, status.Error(codes.Internal, "Error checking storage quota")
	}
	if allowed {
		return quotaResponse(updated, true), nil
	}

	// Nothing was reserved: either the file does not fit, or the user is gone.
	user, err := h.store.FindByID(ctx, userID)
	if err != nil {
		return nil, userLookupError(err)
	}
	return quotaResponse(user, false), nil
}

func (h *QuotaHandler) ReleaseQuota(ctx context.Context, req *userspb.ReleaseQuotaRequest) (*userspb.QuotaResponse, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetSizeBytes() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "size_bytes must be greater than 0")
	}

	updated, err := h.store.ReleaseStorageQuota(ctx, userID, req.GetSizeBytes())
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, "User not found")
		}
		log.Printf("release quota for %s: %v", userID.Hex(), err)
		return nil, status.Error(codes.Internal, "Error releasing storage quota")
	}
	return quotaResponse(updated, true), nil
}

func callerID(ctx context.Context) (primitive.ObjectID, error) {
	tokenStr, ok := bearerFromMetadata(ctx)
	if !ok {
		return primitive.NilObjectID, status.Error(codes.Unauthenticated, "Missing or invalid authorization token")
	}
	userID, err := service.VerifySessionToken(tokenStr)
	if err != nil {
		return primitive.NilObjectID, status.Error(codes.Unauthenticated, err.Error())
	}
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return primitive.NilObjectID, status.Error(codes.Unauthenticated, "Invalid user identity in token")
	}
	return oid, nil
}

func userLookupError(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return status.Error(codes.NotFound, "User not found")
	}
	return status.Error(codes.Internal, "Error loading user")
}

func quotaResponse(u *domain.User, allowed bool) *userspb.QuotaResponse {
	remaining := u.StorageQuotaBytes - u.UsedStorageBytes
	if remaining < 0 {
		remaining = 0
	}
	return &userspb.QuotaResponse{
		Allowed:           allowed,
		StorageQuotaBytes: u.StorageQuotaBytes,
		UsedStorageBytes:  u.UsedStorageBytes,
		RemainingBytes:    remaining,
	}
}
