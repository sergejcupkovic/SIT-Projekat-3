package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"users-service/domain"
	"users-service/repository"
	"users-service/service"

	userspb "stashr/proto/users"

	"stashr/shared/appconfig"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	mb = int64(1) << 20
	gb = int64(1) << 30
)

// fakeRequests mimics the unique "one pending request per user" index and the
// atomic pending -> resolved transition.
type fakeRequests struct {
	items map[primitive.ObjectID]*domain.QuotaRequest
}

func (f *fakeRequests) Create(_ context.Context, r *domain.QuotaRequest) error {
	for _, other := range f.items {
		if other.UserID == r.UserID && other.Status == domain.QuotaRequestPending {
			return repository.ErrPendingQuotaRequestExists
		}
	}
	r.ID = primitive.NewObjectID()
	cp := *r
	f.items[r.ID] = &cp
	return nil
}

func (f *fakeRequests) FindByID(_ context.Context, id primitive.ObjectID) (*domain.QuotaRequest, error) {
	r, ok := f.items[id]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	cp := *r
	return &cp, nil
}

func (f *fakeRequests) FindByUser(_ context.Context, userID primitive.ObjectID) ([]domain.QuotaRequest, error) {
	var out []domain.QuotaRequest
	for _, r := range f.items {
		if r.UserID == userID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeRequests) FindAll(_ context.Context, st domain.QuotaRequestStatus) ([]domain.QuotaRequest, error) {
	var out []domain.QuotaRequest
	for _, r := range f.items {
		if st == "" || r.Status == st {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeRequests) Resolve(_ context.Context, id primitive.ObjectID, st domain.QuotaRequestStatus, adminID string, at time.Time) (*domain.QuotaRequest, error) {
	r, ok := f.items[id]
	if !ok || r.Status != domain.QuotaRequestPending {
		return nil, mongo.ErrNoDocuments
	}
	r.Status, r.ResolvedBy, r.ResolvedAt = st, adminID, at
	cp := *r
	return &cp, nil
}

func (f *fakeRequests) Reopen(_ context.Context, id primitive.ObjectID) error {
	r := f.items[id]
	r.Status, r.ResolvedBy, r.ResolvedAt = domain.QuotaRequestPending, "", time.Time{}
	return nil
}

type fakeUsers struct {
	*fakeStore
	raiseErr error
}

func (f fakeUsers) FindByIDs(_ context.Context, ids []primitive.ObjectID) ([]domain.User, error) {
	var out []domain.User
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (f fakeUsers) RaiseStorageQuota(_ context.Context, id primitive.ObjectID, quota int64) (*domain.User, bool, error) {
	if f.raiseErr != nil {
		return nil, false, f.raiseErr
	}
	u, ok := f.users[id]
	if !ok || u.StorageQuotaBytes >= quota {
		return nil, false, nil
	}
	u.StorageQuotaBytes = quota
	cp := *u
	return &cp, true, nil
}

type requestWorld struct {
	h        *QuotaRequestHandler
	requests *fakeRequests
	store    *fakeStore
	userID   primitive.ObjectID
	user     context.Context
	admin    context.Context
}

// newRequestWorld: one regular user with a 100 MB quota, 40 MB used.
func newRequestWorld(t *testing.T) *requestWorld {
	t.Helper()
	_, store, id, userCtx := setup(t, 100*mb, 40*mb)
	requests := &fakeRequests{items: map[primitive.ObjectID]*domain.QuotaRequest{}}
	return &requestWorld{
		h:        NewQuotaRequestHandler(requests, fakeUsers{fakeStore: store}, appconfig.DefaultLimits()),
		requests: requests,
		store:    store,
		userID:   id,
		user:     userCtx,
		admin:    ctxFor(t, primitive.NewObjectID().Hex(), "A"),
	}
}

func (w *requestWorld) submit(t *testing.T, bytes int64) *userspb.QuotaRequestInfo {
	t.Helper()
	info, err := w.h.CreateQuotaRequest(w.user, &userspb.CreateQuotaRequestRequest{RequestedQuotaBytes: bytes})
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestCreateQuotaRequest_Success(t *testing.T) {
	w := newRequestWorld(t)
	info := w.submit(t, gb)

	if info.Status != "pending" || info.RequestedQuotaBytes != gb || info.QuotaAtRequestBytes != 100*mb {
		t.Fatalf("unexpected reply: %+v", info)
	}
	if info.ResolvedAt != nil {
		t.Fatal("a pending request has no resolution time")
	}
	if w.store.users[w.userID].StorageQuotaBytes != 100*mb {
		t.Fatal("submitting must not change the quota")
	}
}

func TestCreateQuotaRequest_Bounds(t *testing.T) {
	limits := appconfig.DefaultLimits()
	for name, bytes := range map[string]int64{
		"negative":         -1,
		"zero":             0,
		"below minimum":    limits.MinQuotaBytes - 1,
		"equal to current": 100 * mb,
		"above maximum":    limits.MaxQuotaBytes + 1,
	} {
		w := newRequestWorld(t)
		_, err := w.h.CreateQuotaRequest(w.user, &userspb.CreateQuotaRequestRequest{RequestedQuotaBytes: bytes})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: expected InvalidArgument, got %v", name, err)
		}
		if len(w.requests.items) != 0 {
			t.Errorf("%s: nothing may be stored", name)
		}
	}

	w := newRequestWorld(t)
	w.submit(t, limits.MaxQuotaBytes) // the maximum itself is allowed
}

func TestCreateQuotaRequest_OnlyOnePending(t *testing.T) {
	w := newRequestWorld(t)
	first := w.submit(t, gb)

	_, err := w.h.CreateQuotaRequest(w.user, &userspb.CreateQuotaRequestRequest{RequestedQuotaBytes: 2 * gb})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}

	// Once it is resolved a new one may be submitted.
	if _, err := w.h.RejectQuotaRequest(w.admin, &userspb.ResolveQuotaRequestRequest{RequestId: first.Id}); err != nil {
		t.Fatal(err)
	}
	w.submit(t, 2*gb)
}

func TestApproveQuotaRequest(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, gb)

	info, err := w.h.ApproveQuotaRequest(w.admin, &userspb.ResolveQuotaRequestRequest{RequestId: req.Id})
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != "approved" || info.CurrentQuotaBytes != gb || info.ResolvedAt == nil {
		t.Fatalf("unexpected reply: %+v", info)
	}
	if got := w.store.users[w.userID].StorageQuotaBytes; got != gb {
		t.Fatalf("quota = %d, want %d", got, gb)
	}
}

func TestRejectQuotaRequestLeavesQuotaAlone(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, gb)

	info, err := w.h.RejectQuotaRequest(w.admin, &userspb.ResolveQuotaRequestRequest{RequestId: req.Id})
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != "rejected" || w.store.users[w.userID].StorageQuotaBytes != 100*mb {
		t.Fatalf("unexpected result: %+v", info)
	}
}

func TestResolvedRequestCannotBeDecidedAgain(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, gb)
	id := &userspb.ResolveQuotaRequestRequest{RequestId: req.Id}
	if _, err := w.h.RejectQuotaRequest(w.admin, id); err != nil {
		t.Fatal(err)
	}

	if _, err := w.h.ApproveQuotaRequest(w.admin, id); status.Code(err) != codes.Aborted {
		t.Fatalf("approve after reject: expected Aborted, got %v", err)
	}
	if _, err := w.h.RejectQuotaRequest(w.admin, id); status.Code(err) != codes.Aborted {
		t.Fatalf("reject twice: expected Aborted, got %v", err)
	}
	if w.store.users[w.userID].StorageQuotaBytes != 100*mb {
		t.Fatal("quota must be unchanged")
	}
}

// Approving an "increase" must never shrink a quota the admin raised meanwhile.
func TestApproveNeverLowersAQuota(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, gb)
	w.store.users[w.userID].StorageQuotaBytes = 5 * gb

	_, err := w.h.ApproveQuotaRequest(w.admin, &userspb.ResolveQuotaRequestRequest{RequestId: req.Id})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("expected Aborted, got %v", err)
	}
	if w.store.users[w.userID].StorageQuotaBytes != 5*gb {
		t.Fatal("quota must be unchanged")
	}
	if got := w.requests.items[mustID(t, req.Id)].Status; got != domain.QuotaRequestPending {
		t.Fatalf("the request must stay pending, got %s", got)
	}
}

func TestApproveReopensTheRequestWhenTheUpdateFails(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, gb)
	w.h.users = fakeUsers{fakeStore: w.store, raiseErr: errors.New("database down")}

	_, err := w.h.ApproveQuotaRequest(w.admin, &userspb.ResolveQuotaRequestRequest{RequestId: req.Id})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
	if got := w.requests.items[mustID(t, req.Id)].Status; got != domain.QuotaRequestPending {
		t.Fatalf("the request must stay pending, got %s", got)
	}
}

func TestApproveRechecksTheConfiguredMaximum(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, 10*gb)
	w.h.limits.MaxQuotaBytes = gb // lowered in config.yaml after the request

	_, err := w.h.ApproveQuotaRequest(w.admin, &userspb.ResolveQuotaRequestRequest{RequestId: req.Id})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestQuotaRequestAccessControl(t *testing.T) {
	w := newRequestWorld(t)
	req := w.submit(t, gb)
	id := &userspb.ResolveQuotaRequestRequest{RequestId: req.Id}

	if _, err := w.h.ListQuotaRequests(w.user, &userspb.ListQuotaRequestsRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("list as user: %v", err)
	}
	if _, err := w.h.ApproveQuotaRequest(w.user, id); status.Code(err) != codes.PermissionDenied {
		t.Errorf("approve as user: %v", err)
	}
	if _, err := w.h.RejectQuotaRequest(w.user, id); status.Code(err) != codes.PermissionDenied {
		t.Errorf("reject as user: %v", err)
	}
	if _, err := w.h.ApproveQuotaRequest(context.Background(), id); status.Code(err) != codes.Unauthenticated {
		t.Errorf("approve without a token: %v", err)
	}
	// Administrators do not submit requests.
	if _, err := w.h.CreateQuotaRequest(w.admin, &userspb.CreateQuotaRequestRequest{RequestedQuotaBytes: gb}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("create as admin: %v", err)
	}

	// A user only ever sees their own requests.
	otherID := primitive.NewObjectID()
	w.store.users[otherID] = &domain.User{ID: otherID, Role: domain.RoleRegular, StorageQuotaBytes: 100 * mb}
	mine, err := w.h.ListMyQuotaRequests(ctxFor(t, otherID.Hex(), "RK"), &userspb.ListMyQuotaRequestsRequest{})
	if err != nil || len(mine.Requests) != 0 {
		t.Errorf("other user's list: %v %v", mine, err)
	}
}

func TestListQuotaRequests(t *testing.T) {
	w := newRequestWorld(t)
	w.submit(t, gb)

	resp, err := w.h.ListQuotaRequests(w.admin, &userspb.ListQuotaRequestsRequest{Status: "pending"})
	if err != nil || len(resp.Requests) != 1 {
		t.Fatalf("pending: %v %v", resp, err)
	}
	if got := resp.Requests[0]; got.CurrentQuotaBytes != 100*mb || got.UsedStorageBytes != 40*mb {
		t.Fatalf("user details missing: %+v", got)
	}
	if resp, err = w.h.ListQuotaRequests(w.admin, &userspb.ListQuotaRequestsRequest{Status: "approved"}); err != nil || len(resp.Requests) != 0 {
		t.Fatalf("approved: %v %v", resp, err)
	}
	if _, err = w.h.ListQuotaRequests(w.admin, &userspb.ListQuotaRequestsRequest{Status: "bogus"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("bogus status: %v", err)
	}
}

// The range check runs before any database access, so no repository is needed.
func TestUpdateUserQuotaRejectsValuesOutsideTheBounds(t *testing.T) {
	setup(t, 100*mb, 0)
	h := &UserHandler{}
	admin := ctxFor(t, primitive.NewObjectID().Hex(), "A")
	limits := service.Cfg.Limits

	for _, bytes := range []int64{-1, 0, limits.MinQuotaBytes - 1, limits.MaxQuotaBytes + 1} {
		_, err := h.UpdateUserQuota(admin, &userspb.UpdateUserQuotaRequest{
			UserId: primitive.NewObjectID().Hex(), StorageQuotaBytes: bytes,
		})
		st := status.Convert(err)
		if st.Code() != codes.InvalidArgument || st.Message() != "Storage quota must be between 100 MB and 2 TB" {
			t.Errorf("%d bytes: got %v %q", bytes, st.Code(), st.Message())
		}
	}
}

func mustID(t *testing.T, hex string) primitive.ObjectID {
	t.Helper()
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
