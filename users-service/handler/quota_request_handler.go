package handler

import (
	"context"
	"errors"
	"log"
	"time"

	"users-service/domain"
	"users-service/repository"

	userspb "stashr/proto/users"

	"stashr/shared/appconfig"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// quotaRequestStore is the part of repository.QuotaRequestRepository used here.
type quotaRequestStore interface {
	Create(ctx context.Context, request *domain.QuotaRequest) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.QuotaRequest, error)
	FindByUser(ctx context.Context, userID primitive.ObjectID) ([]domain.QuotaRequest, error)
	FindAll(ctx context.Context, status domain.QuotaRequestStatus) ([]domain.QuotaRequest, error)
	Resolve(ctx context.Context, id primitive.ObjectID, status domain.QuotaRequestStatus, adminID string, at time.Time) (*domain.QuotaRequest, error)
	Reopen(ctx context.Context, id primitive.ObjectID) error
}

// quotaRequestUsers is the part of repository.UserRepository used here.
type quotaRequestUsers interface {
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.User, error)
	FindByIDs(ctx context.Context, ids []primitive.ObjectID) ([]domain.User, error)
	RaiseStorageQuota(ctx context.Context, userID primitive.ObjectID, newQuotaBytes int64) (*domain.User, bool, error)
}

// QuotaRequestHandler implements userspb.QuotaRequestServiceServer: regular
// users ask for more storage, administrators approve or reject.
type QuotaRequestHandler struct {
	userspb.UnimplementedQuotaRequestServiceServer

	requests quotaRequestStore
	users    quotaRequestUsers
	limits   appconfig.Limits
	// quota_request_rollback_timeout
	rollbackTimeout time.Duration
}

var _ userspb.QuotaRequestServiceServer = (*QuotaRequestHandler)(nil)

func NewQuotaRequestHandler(requests quotaRequestStore, users quotaRequestUsers, limits appconfig.Limits) *QuotaRequestHandler {
	return &QuotaRequestHandler{requests: requests, users: users, limits: limits, rollbackTimeout: 5 * time.Second}
}

// SetRollbackTimeout applies quota_request_rollback_timeout.
func (h *QuotaRequestHandler) SetRollbackTimeout(d time.Duration) { h.rollbackTimeout = d }

const pendingExistsMsg = "You already have a pending quota request. Wait for an administrator to decide on it."

// CreateQuotaRequest  <-  POST /api/users/quota-requests  (HTTP 201)
// The user comes from the session token, never from the request.
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
	if !h.limits.QuotaInRange(requested) {
		return nil, status.Error(codes.InvalidArgument, h.limits.QuotaRangeMessage())
	}
	if requested <= user.StorageQuotaBytes {
		return nil, status.Error(codes.InvalidArgument,
			"Requested quota must be greater than your current quota of "+appconfig.FormatBytes(user.StorageQuotaBytes))
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
			return nil, status.Error(codes.AlreadyExists, pendingExistsMsg)
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

	resp := &userspb.ListQuotaRequestsResponse{Requests: make([]*userspb.QuotaRequestInfo, 0, len(requests))}
	for i := range requests {
		resp.Requests = append(resp.Requests, toQuotaRequestInfo(&requests[i], user))
	}
	return resp, nil
}

// ListQuotaRequests  <-  GET /api/admin/quota-requests[?status=...]
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

	ids := make([]primitive.ObjectID, 0, len(requests))
	seen := make(map[primitive.ObjectID]bool, len(requests))
	for _, r := range requests {
		if !seen[r.UserID] {
			seen[r.UserID] = true
			ids = append(ids, r.UserID)
		}
	}
	users, err := h.users.FindByIDs(ctx, ids)
	if err != nil {
		log.Printf("load users of quota requests: %v", err)
		return nil, status.Error(codes.Internal, "Error loading quota requests")
	}
	byID := make(map[primitive.ObjectID]*domain.User, len(users))
	for i := range users {
		byID[users[i].ID] = &users[i]
	}

	resp := &userspb.ListQuotaRequestsResponse{Requests: make([]*userspb.QuotaRequestInfo, 0, len(requests))}
	for i := range requests {
		resp.Requests = append(resp.Requests, toQuotaRequestInfo(&requests[i], byID[requests[i].UserID]))
	}
	return resp, nil
}

// ApproveQuotaRequest  <-  POST /api/admin/quota-requests/{request_id}/approve
//
// The request is first claimed (pending -> approved) and only then is the quota
// raised, so two administrators cannot decide it twice. If the quota cannot be
// raised the request goes back to pending.
func (h *QuotaRequestHandler) ApproveQuotaRequest(ctx context.Context, in *userspb.ResolveQuotaRequestRequest) (*userspb.QuotaRequestInfo, error) {
	adminID, request, err := h.pendingRequestForAdmin(ctx, in.GetRequestId())
	if err != nil {
		return nil, err
	}
	// The bounds may have been changed in config.yaml since the request was made.
	if !h.limits.QuotaInRange(request.RequestedQuotaBytes) {
		return nil, status.Error(codes.InvalidArgument, h.limits.QuotaRangeMessage())
	}

	resolved, err := h.requests.Resolve(ctx, request.ID, domain.QuotaRequestApproved, adminID, time.Now().UTC())
	if err != nil {
		return nil, resolveError(err)
	}

	user, raised, err := h.users.RaiseStorageQuota(ctx, resolved.UserID, resolved.RequestedQuotaBytes)
	if err != nil {
		log.Printf("approve quota request %s: %v", resolved.ID.Hex(), err)
		h.reopen(ctx, resolved.ID)
		return nil, status.Error(codes.Internal, "Error updating storage quota")
	}
	if !raised {
		h.reopen(ctx, resolved.ID)
		current, err := h.users.FindByID(ctx, resolved.UserID)
		if err != nil {
			return nil, userLookupError(err)
		}
		return nil, status.Error(codes.Aborted,
			"The user's quota is already "+appconfig.FormatBytes(current.StorageQuotaBytes)+
				", which is not less than the requested size. Reject this request instead.")
	}

	return toQuotaRequestInfo(resolved, user), nil
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
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		// The decision is saved; only the user details of the reply are missing.
		log.Printf("load user of rejected quota request %s: %v", resolved.ID.Hex(), err)
	}
	return toQuotaRequestInfo(resolved, user), nil
}

// pendingRequestForAdmin checks that the caller is an administrator and that
// requestID names a request that still awaits a decision.
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

// reopen must also work when the caller has given up on the request.
func (h *QuotaRequestHandler) reopen(ctx context.Context, id primitive.ObjectID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.rollbackTimeout)
	defer cancel()
	if err := h.requests.Reopen(ctx, id); err != nil {
		log.Printf("WARNING: quota request %s is marked approved but the quota was not changed: %v", id.Hex(), err)
	}
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

// toQuotaRequestInfo builds the reply; user is nil if the account no longer exists.
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
