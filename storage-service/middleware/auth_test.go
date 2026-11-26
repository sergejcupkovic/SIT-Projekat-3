package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var secret = []byte("test-secret-test-secret-test-secret-test")

func signToken(t *testing.T, key []byte, role string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": "u1", "role": role, "exp": time.Now().Add(time.Hour).Unix()}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestAuthInterceptor(t *testing.T) {
	otherKey := []byte("some-other-secret-some-other-secret")
	tests := []struct {
		name   string
		header string
		want   codes.Code
	}{
		{"no token", "", codes.Unauthenticated},
		{"forged token", "Bearer " + signToken(t, otherKey, "RK"), codes.Unauthenticated},
		{"administrator", "Bearer " + signToken(t, secret, "A"), codes.PermissionDenied},
		{"regular user", "Bearer " + signToken(t, secret, "RK"), codes.OK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", tc.header))
			info := &grpc.UnaryServerInfo{FullMethod: "/storage.StorageService/BrowseFolder"}
			next := func(context.Context, any) (any, error) { return nil, nil }

			_, err := AuthUnaryInterceptor(secret)(ctx, nil, info, next)
			if status.Code(err) != tc.want {
				t.Errorf("got %v, want %v", status.Code(err), tc.want)
			}
		})
	}
}
