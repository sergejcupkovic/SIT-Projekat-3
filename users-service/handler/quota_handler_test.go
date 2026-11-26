package handler

import (
	"context"
	"testing"
	"time"

	"users-service/config"

	"stashr/shared/appconfig"
	"users-service/domain"
	"users-service/service"

	userspb "stashr/proto/users"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeStore mimics the atomic behaviour of the MongoDB queries.
type fakeStore struct {
	users map[primitive.ObjectID]*domain.User
}

func (f *fakeStore) FindByID(_ context.Context, id primitive.ObjectID) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	cp := *u
	return &cp, nil
}

func (f *fakeStore) ReserveStorageQuota(_ context.Context, id primitive.ObjectID, size int64) (*domain.User, bool, error) {
	u, ok := f.users[id]
	if !ok || u.UsedStorageBytes+size > u.StorageQuotaBytes {
		return nil, false, nil
	}
	u.UsedStorageBytes += size
	cp := *u
	return &cp, true, nil
}

func (f *fakeStore) ReleaseStorageQuota(_ context.Context, id primitive.ObjectID, size int64) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	u.UsedStorageBytes -= size
	if u.UsedStorageBytes < 0 {
		u.UsedStorageBytes = 0
	}
	cp := *u
	return &cp, nil
}

func setup(t *testing.T, quota, used int64) (*QuotaHandler, *fakeStore, primitive.ObjectID, context.Context) {
	t.Helper()
	service.Init(&config.Config{
		JWTSecret:  "test-secret-test-secret-test-secret-test",
		Limits:     appconfig.DefaultLimits(),
		BcryptCost: bcrypt.MinCost,
		JWTTTL:     time.Hour,
	})

	id := primitive.NewObjectID()
	store := &fakeStore{users: map[primitive.ObjectID]*domain.User{
		id: {ID: id, Role: domain.RoleRegular, StorageQuotaBytes: quota, UsedStorageBytes: used},
	}}
	return NewQuotaHandler(store), store, id, ctxFor(t, id.Hex(), "RK")
}

func ctxFor(t *testing.T, userID, role string) context.Context {
	t.Helper()
	tok, err := service.GenerateJWTToken(userID, "u@example.com", role)
	if err != nil {
		t.Fatal(err)
	}
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
}

func TestReserveQuota_AllowedWhenFileFits(t *testing.T) {
	h, store, id, ctx := setup(t, 100, 40)

	resp, err := h.ReserveQuota(ctx, &userspb.ReserveQuotaRequest{SizeBytes: 60})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Allowed || resp.UsedStorageBytes != 100 || resp.RemainingBytes != 0 {
		t.Fatalf("unexpected reply: %+v", resp)
	}
	if store.users[id].UsedStorageBytes != 100 {
		t.Fatalf("usage not persisted: %d", store.users[id].UsedStorageBytes)
	}
}

func TestReserveQuota_DeniedWhenFileDoesNotFit(t *testing.T) {
	h, store, id, ctx := setup(t, 100, 40)

	resp, err := h.ReserveQuota(ctx, &userspb.ReserveQuotaRequest{SizeBytes: 61})
	if err != nil {
		t.Fatalf("a quota violation is a normal reply, not an error: %v", err)
	}
	if resp.Allowed {
		t.Fatal("61 bytes must not fit into 60 free bytes")
	}
	if resp.StorageQuotaBytes != 100 || resp.UsedStorageBytes != 40 || resp.RemainingBytes != 60 {
		t.Fatalf("reply must carry the current numbers: %+v", resp)
	}
	if store.users[id].UsedStorageBytes != 40 {
		t.Fatal("a denied reservation must not change usage")
	}
}

func TestReserveQuota_RejectsNonPositiveSize(t *testing.T) {
	h, _, _, ctx := setup(t, 100, 0)
	for _, size := range []int64{0, -1, -100} {
		if _, err := h.ReserveQuota(ctx, &userspb.ReserveQuotaRequest{SizeBytes: size}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("size %d: expected InvalidArgument, got %v", size, err)
		}
	}
}

func TestReleaseQuota_ReturnsSpaceAndNeverGoesNegative(t *testing.T) {
	h, store, id, ctx := setup(t, 100, 30)

	if _, err := h.ReleaseQuota(ctx, &userspb.ReleaseQuotaRequest{SizeBytes: 10}); err != nil {
		t.Fatal(err)
	}
	if got := store.users[id].UsedStorageBytes; got != 20 {
		t.Fatalf("expected 20, got %d", got)
	}

	resp, err := h.ReleaseQuota(ctx, &userspb.ReleaseQuotaRequest{SizeBytes: 999})
	if err != nil {
		t.Fatal(err)
	}
	if resp.UsedStorageBytes != 0 {
		t.Fatalf("usage must clamp at 0, got %d", resp.UsedStorageBytes)
	}
}

func TestGetQuota(t *testing.T) {
	h, _, _, ctx := setup(t, 100, 25)

	resp, err := h.GetQuota(ctx, &userspb.GetQuotaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StorageQuotaBytes != 100 || resp.UsedStorageBytes != 25 || resp.RemainingBytes != 75 {
		t.Fatalf("unexpected reply: %+v", resp)
	}
}

func TestQuotaRpcsRequireAValidSessionToken(t *testing.T) {
	h, _, id, _ := setup(t, 100, 0)

	restricted, _ := service.GenerateRestrictedJWTToken(id.Hex(), "u@example.com", "RK")
	cases := map[string]context.Context{
		"no metadata":      context.Background(),
		"no bearer prefix": metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "garbage")),
		"garbage token":    metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer garbage")),
		"restricted token": metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+restricted)),
		"admin role":       ctxFor(t, id.Hex(), "A"),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := h.GetQuota(ctx, &userspb.GetQuotaRequest{}); status.Code(err) != codes.Unauthenticated {
				t.Errorf("GetQuota: expected Unauthenticated, got %v", err)
			}
			if _, err := h.ReserveQuota(ctx, &userspb.ReserveQuotaRequest{SizeBytes: 1}); status.Code(err) != codes.Unauthenticated {
				t.Errorf("ReserveQuota: expected Unauthenticated, got %v", err)
			}
			if _, err := h.ReleaseQuota(ctx, &userspb.ReleaseQuotaRequest{SizeBytes: 1}); status.Code(err) != codes.Unauthenticated {
				t.Errorf("ReleaseQuota: expected Unauthenticated, got %v", err)
			}
		})
	}
}

// A caller can only touch its own quota: the user comes from the JWT, never
// from the request, so another user's usage is unaffected.
func TestQuotaRpcsOnlyTouchTheCallersOwnQuota(t *testing.T) {
	h, store, mine, ctx := setup(t, 100, 0)
	otherID := primitive.NewObjectID()
	store.users[otherID] = &domain.User{ID: otherID, Role: domain.RoleRegular, StorageQuotaBytes: 100, UsedStorageBytes: 50}

	if _, err := h.ReserveQuota(ctx, &userspb.ReserveQuotaRequest{SizeBytes: 10}); err != nil {
		t.Fatal(err)
	}
	if store.users[mine].UsedStorageBytes != 10 || store.users[otherID].UsedStorageBytes != 50 {
		t.Fatalf("mine=%d other=%d", store.users[mine].UsedStorageBytes, store.users[otherID].UsedStorageBytes)
	}
}

func TestGetQuota_UnknownUser(t *testing.T) {
	h, _, _, _ := setup(t, 100, 0)
	ctx := ctxFor(t, primitive.NewObjectID().Hex(), "RK")
	if _, err := h.GetQuota(ctx, &userspb.GetQuotaRequest{}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}
