package handler

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"notifications-service/domain"
	"notifications-service/middleware"

	notificationspb "stashr/proto/notifications"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

var testSecret = []byte("test-secret-test-secret-test-secret-test")

type fakeRepo struct {
	items []domain.Notification
	err   error
}

func (f *fakeRepo) ListForAllUsers(_ context.Context) ([]domain.Notification, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func startTestClient(t *testing.T, repo notificationRepository) notificationspb.NotificationServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(grpc.UnaryInterceptor(middleware.AuthUnaryInterceptor(testSecret)))
	notificationspb.RegisterNotificationServiceServer(srv, NewNotificationHandler(repo))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return notificationspb.NewNotificationServiceClient(conn)
}

func rkCtx(t *testing.T) context.Context {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-1",
		"role": "RK",
		"exp":  time.Now().Add(time.Hour).Unix(),
	}).SignedString(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
}

func TestGetNotifications_ReturnsSeededItems(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeRepo{
		items: []domain.Notification{
			{ID: "1", Title: "Welcome", Message: "Hello", Type: "success", CreatedAt: now},
			{ID: "2", Title: "Security", Message: "2FA active", Type: "info", CreatedAt: now.Add(-time.Minute)},
		},
	}
	client := startTestClient(t, repo)

	resp, err := client.GetNotifications(rkCtx(t), &notificationspb.GetNotificationsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetNotifications()) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(resp.GetNotifications()))
	}
	if resp.GetNotifications()[0].GetTitle() != "Welcome" {
		t.Fatalf("unexpected first notification: %+v", resp.GetNotifications()[0])
	}
}

func TestGetNotifications_RejectsUnauthenticated(t *testing.T) {
	client := startTestClient(t, &fakeRepo{})
	_, err := client.GetNotifications(context.Background(), &notificationspb.GetNotificationsRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
	}
}

func TestGetNotifications_RepoFailureReturnsInternal(t *testing.T) {
	client := startTestClient(t, &fakeRepo{err: errors.New("cassandra timeout")})
	_, err := client.GetNotifications(rkCtx(t), &notificationspb.GetNotificationsRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", status.Code(err))
	}
}