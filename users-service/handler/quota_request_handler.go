package handler

import (
	"context"
	"errors"
	"log"
	"time"

	"users-service/domain"
	"users-service/repository"
	"users-service/service"

	userspb "stashr/proto/users"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type QuotaRequestHandler struct {
	userspb.UnimplementedQuotaRequestServiceServer

	requests *repository.QuotaRequestRepository
	users    *repository.UserRepository
}

func NewQuotaRequestHandler(requests *repository.QuotaRequestRepository, users *repository.UserRepository) *QuotaRequestHandler {
	return &QuotaRequestHandler{requests: requests, users: users}
}

// CreateQuotaRequest  <-  POST /api/users/quota-requests
func (h *QuotaRequestHandler) CreateQuotaRequest(ctx context.Context, in *userspb.CreateQuotaRequestRequest) (*userspb.QuotaRequestInfo, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return nil, userLookupError(err)
	}

	requested := in.GetRequestedQuotaBytes()
	if err := service.ValidateQuotaRequest(requested, user.StorageQuotaBytes); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	request := &domain.QuotaRequest{
		UserID:              user.ID,
		QuotaAtRequestBytes: user.StorageQuotaBytes,
		RequestedQuotaBytes: requested,
		Status:              domain.QuotaRequestPending,
		CreatedAt:           time.Now().UTC(),
	}
	if err := h.requests.Create(ctx, request); err != nil {
		if errors.Is(err, repository.ErrPendingQuotaRequestExists) {
			return nil, status.Error(codes.AlreadyExists, "You already have a pending quota request. Wait for an administrator to decide on it.")
		}
		log.Printf("create quota request for %s: %v", user.ID.Hex(), err)
		return nil, status.Error(codes.Internal, "Error saving quota request")
	}

	setHTTPStatus(ctx, 201)
	return toQuotaRequestInfo(request, user), nil
}

// ListMyQuotaRequests  <-  GET /api/users/quota-requests
func (h *QuotaRequestHandler) ListMyQuotaRequests(ctx context.Context, _ *userspb.ListMyQuotaRequestsRequest) (*userspb.ListQuotaRequestsResponse, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return nil, userLookupError(err)
	}
	requests, err := h.requests.FindByUser(ctx, userID)
	if err != nil {
		log.Printf("list quota requests of %s: %v", userID.Hex(), err)
		return nil, status.Error(codes.Internal, "Error loading quota requests")
	}

	resp := &userspb.ListQuotaRequestsResponse{}
	for i := range requests {
		resp.Requests = append(resp.Requests, toQuotaRequestInfo(&requests[i], user))
	}
	return resp, nil
}

// ListQuotaRequests  <-  GET /api/admin/quota-requests
func (h *QuotaRequestHandler) ListQuotaRequests(ctx context.Context, in *userspb.ListQuotaRequestsRequest) (*userspb.ListQuotaRequestsResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	filter := domain.QuotaRequestStatus(in.GetStatus())
	switch filter {
	case "", domain.QuotaRequestPending, domain.QuotaRequestApproved, domain.QuotaRequestRejected:
	default:
		return nil, status.Error(codes.InvalidArgument, "status must be pending, approved or rejected")
	}

	requests, err := h.requests.FindAll(ctx, filter)
	if err != nil {
		log.Printf("list quota requests: %v", err)
		return nil, status.Error(codes.Internal, "Error loading quota requests")
	}

	resp := &userspb.ListQuotaRequestsResponse{}
	for i := range requests {
		user, err := h.users.FindByID(ctx, requests[i].UserID)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, userLookupError(err)
		}
		resp.Requests = append(resp.Requests, toQuotaRequestInfo(&requests[i], user))
	}
	return resp, nil
}

// ApproveQuotaRequest  <-  POST /api/admin/quota-requests/{request_id}/approve
func (h *QuotaRequestHandler) ApproveQuotaRequest(ctx context.Context, in *userspb.ResolveQuotaRequestRequest) (*userspb.QuotaRequestInfo, error) {
	adminID, request, err := h.pendingRequestForAdmin(ctx, in.GetRequestId())
	if err != nil {
		return nil, err
	}

	user, err := h.users.FindByID(ctx, request.UserID)
	if err != nil {
		return nil, userLookupError(err)
	}
	if !service.QuotaInRange(request.RequestedQuotaBytes) {
		return nil, status.Error(codes.InvalidArgument, service.QuotaRangeMessage())
	}
	// Approving an increase must never lower a quota the admin raised meanwhile.
	if request.RequestedQuotaBytes <= user.StorageQuotaBytes {
		return nil, status.Error(codes.Aborted, "The user's quota is already "+service.FormatBytes(user.StorageQuotaBytes)+
			", which is not less than the requested size. Reject this request instead.")
	}

	// Resolve only matches a pending request, so it cannot be decided twice.
	resolved, err := h.requests.Resolve(ctx, request.ID, domain.QuotaRequestApproved, adminID, time.Now().UTC())
	if err != nil {
		return nil, resolveError(err)
	}
	updated, err := h.users.UpdateStorageQuota(ctx, resolved.UserID, resolved.RequestedQuotaBytes)
	if err != nil {
		log.Printf("approve quota request %s: %v", resolved.ID.Hex(), err)
		return nil, status.Error(codes.Internal, "Error updating storage quota")
	}
	return toQuotaRequestInfo(resolved, updated), nil
}

// RejectQuotaRequest  <-  POST /api/admin/quota-requests/{request_id}/reject
func (h *QuotaRequestHandler) RejectQuotaRequest(ctx context.Context, in *userspb.ResolveQuotaRequestRequest) (*userspb.QuotaRequestInfo, error) {
	adminID, request, err := h.pendingRequestForAdmin(ctx, in.GetRequestId())
	if err != nil {
		return nil, err
	}

	resolved, err := h.requests.Resolve(ctx, request.ID, domain.QuotaRequestRejected, adminID, time.Now().UTC())
	if err != nil {
		return nil, resolveError(err)
	}
	user, err := h.users.FindByID(ctx, resolved.UserID)
	if err != nil {
		return nil, userLookupError(err)
	}
	return toQuotaRequestInfo(resolved, user), nil
}

func (h *QuotaRequestHandler) pendingRequestForAdmin(ctx context.Context, requestID string) (string, *domain.QuotaRequest, error) {
	adminID, err := adminCallerID(ctx)
	if err != nil {
		return "", nil, err
	}

	oid, err := primitive.ObjectIDFromHex(requestID)
	if err != nil {
		return "", nil, status.Error(codes.InvalidArgument, "Invalid request ID")
	}
	request, err := h.requests.FindByID(ctx, oid)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", nil, status.Error(codes.NotFound, "Quota request not found")
		}
		return "", nil, status.Error(codes.Internal, "Error loading quota request")
	}
	if request.Status != domain.QuotaRequestPending {
		return "", nil, alreadyResolvedError()
	}
	return adminID, request, nil
}

func alreadyResolvedError() error {
	return status.Error(codes.Aborted, "This quota request has already been resolved")
}

func resolveError(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return alreadyResolvedError()
	}
	log.Printf("resolve quota request: %v", err)
	return status.Error(codes.Internal, "Error updating quota request")
}

// user is nil if the account no longer exists.
func toQuotaRequestInfo(r *domain.QuotaRequest, user *domain.User) *userspb.QuotaRequestInfo {
	info := &userspb.QuotaRequestInfo{
		Id:                  r.ID.Hex(),
		UserId:              r.UserID.Hex(),
		QuotaAtRequestBytes: r.QuotaAtRequestBytes,
		RequestedQuotaBytes: r.RequestedQuotaBytes,
		Status:              string(r.Status),
		CreatedAt:           timestamppb.New(r.CreatedAt),
	}
	if !r.ResolvedAt.IsZero() {
		info.ResolvedAt = timestamppb.New(r.ResolvedAt)
	}
	if user != nil {
		info.Email = user.Email
		info.FirstName = user.FirstName
		info.LastName = user.LastName
		info.CurrentQuotaBytes = user.StorageQuotaBytes
		info.UsedStorageBytes = user.UsedStorageBytes
	}
	return info
}
