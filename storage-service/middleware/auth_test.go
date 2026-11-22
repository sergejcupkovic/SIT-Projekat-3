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

func signToken(t *testing.T, key []byte, method jwt.SigningMethod, claims jwt.MapClaims) string {
	t.Helper()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func callWith(t *testing.T, authHeader string) (code codes.Code, reached bool) {
	t.Helper()
	ctx := context.Background()
	if authHeader != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authHeader))
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/storage.StorageService/BrowseFolder"}
	handler := func(ctx context.Context, req any) (any, error) {
		reached = true
		return nil, nil
	}
	_, err := AuthUnaryInterceptor(secret)(ctx, nil, info, handler)
	return status.Code(err), reached
}

func TestInterceptorRejectsMissingToken(t *testing.T) {
	if code, reached := callWith(t, ""); code != codes.Unauthenticated || reached {
		t.Fatalf("expected Unauthenticated, got %v (handler reached: %v)", code, reached)
	}
}

func TestInterceptorRejectsBadSignature(t *testing.T) {
	tok := signToken(t, []byte("some-other-secret-some-other-secret"), jwt.SigningMethodHS256,
		jwt.MapClaims{"sub": "u1", "role": "RK"})
	if code, _ := callWith(t, "Bearer "+tok); code != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", code)
	}
}

func TestInterceptorRejectsExpiredToken(t *testing.T) {
	tok := signToken(t, secret, jwt.SigningMethodHS256,
		jwt.MapClaims{"sub": "u1", "role": "RK", "exp": time.Now().Add(-time.Minute).Unix()})
	if code, _ := callWith(t, "Bearer "+tok); code != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", code)
	}
}

func TestInterceptorRejectsOtherHMACAlgorithms(t *testing.T) {
	tok := signToken(t, secret, jwt.SigningMethodHS512, jwt.MapClaims{"sub": "u1", "role": "RK"})
	if code, _ := callWith(t, "Bearer "+tok); code != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", code)
	}
}

func TestInterceptorForbidsAdministrator(t *testing.T) {
	tok := signToken(t, secret, jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u1", "role": "A"})
	if code, reached := callWith(t, "Bearer "+tok); code != codes.PermissionDenied || reached {
		t.Fatalf("expected PermissionDenied, got %v (handler reached: %v)", code, reached)
	}
}

func TestInterceptorForbidsRestrictedToken(t *testing.T) {
	tok := signToken(t, secret, jwt.SigningMethodHS256,
		jwt.MapClaims{"sub": "u1", "role": "RK", "restricted": true})
	if code, _ := callWith(t, "Bearer "+tok); code != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", code)
	}
}

func TestInterceptorAllowsRegularUser(t *testing.T) {
	tok := signToken(t, secret, jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u1", "role": "RK"})
	if code, reached := callWith(t, "Bearer "+tok); code != codes.OK || !reached {
		t.Fatalf("expected the handler to run, got %v (reached: %v)", code, reached)
	}
}
