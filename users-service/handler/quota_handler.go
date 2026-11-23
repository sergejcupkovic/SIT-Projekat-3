package handler

import (
	"context"
	"errors"
	"log"

	"users-service/domain"
	"users-service/service"

	userspb "stashr/proto/users"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// quotaStore is the part of repository.UserRepository the quota rpcs need.
type quotaStore interface {
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.User, error)
	ReserveStorageQuota(ctx context.Context, userID primitive.ObjectID, sizeBytes int64) (*domain.User, bool, error)
	ReleaseStorageQuota(ctx context.Context, userID primitive.ObjectID, sizeBytes int64) (*domain.User, error)
}

// QuotaHandler implements userspb.QuotaServiceServer: the internal rpcs that
// storage-service calls synchronously before it stores an uploaded file
// (US-2.2, acceptance criterion 2.2.2).
//
// These rpcs have no REST binding, so the api-gateway never exposes them.
// Each call must still carry the caller's session JWT in the "authorization"
// metadata (storage-service forwards the one it received from the user), and
// operates only on the quota of the user that JWT was issued for. A caller
// can therefore never read, reserve or release another user's quota.
type QuotaHandler struct {
	userspb.UnimplementedQuotaServiceServer

	store quotaStore
}

var _ userspb.QuotaServiceServer = (*QuotaHandler)(nil)

func NewQuotaHandler(store quotaStore) *QuotaHandler {
	return &QuotaHandler{store: store}
}

// GetQuota returns the caller's quota and usage.
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

// ReserveQuota atomically checks that size_bytes fit into the caller's free
// space and reserves them. Not enough space is a normal reply (allowed=false),
// not an error, so storage-service can show the exact numbers.
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

// ReleaseQuota returns a reservation (e.g. the upload failed after the space
// was reserved). usedStorageBytes never drops below 0.
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

// callerID verifies the session JWT in the metadata and returns the user it
// belongs to.
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
