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

// fakeServerStream is the minimum of grpc.ServerStream the interceptor touches.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeServerStream) Context() context.Context { return f.ctx }

func streamCallWith(t *testing.T, authHeader string) (code codes.Code, gotUserID string) {
	t.Helper()
	ctx := context.Background()
	if authHeader != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authHeader))
	}
	info := &grpc.StreamServerInfo{FullMethod: "/storage.StorageService/UploadFile", IsClientStream: true}
	handler := func(_ any, ss grpc.ServerStream) error {
		gotUserID, _ = ss.Context().Value(UserIDKey).(string)
		return nil
	}
	err := AuthStreamInterceptor(secret)(nil, fakeServerStream{ctx: ctx}, info, handler)
	return status.Code(err), gotUserID
}

func TestStreamInterceptorRejectsMissingAndForgedTokens(t *testing.T) {
	if code, _ := streamCallWith(t, ""); code != codes.Unauthenticated {
		t.Fatalf("missing token: expected Unauthenticated, got %v", code)
	}
	forged := signToken(t, []byte("some-other-secret-some-other-secret"), jwt.SigningMethodHS256,
		jwt.MapClaims{"sub": "u1", "role": "RK"})
	if code, _ := streamCallWith(t, "Bearer "+forged); code != codes.Unauthenticated {
		t.Fatalf("forged token: expected Unauthenticated, got %v", code)
	}
}

func TestStreamInterceptorForbidsRestrictedAndAdmin(t *testing.T) {
	restricted := signToken(t, secret, jwt.SigningMethodHS256,
		jwt.MapClaims{"sub": "u1", "role": "RK", "restricted": true})
	if code, _ := streamCallWith(t, "Bearer "+restricted); code != codes.PermissionDenied {
		t.Fatalf("restricted: expected PermissionDenied, got %v", code)
	}
	admin := signToken(t, secret, jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u1", "role": "A"})
	if code, _ := streamCallWith(t, "Bearer "+admin); code != codes.PermissionDenied {
		t.Fatalf("admin: expected PermissionDenied, got %v", code)
	}
}

func TestStreamInterceptorPutsUserIDIntoStreamContext(t *testing.T) {
	tok := signToken(t, secret, jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-42", "role": "RK"})
	code, uid := streamCallWith(t, "Bearer "+tok)
	if code != codes.OK || uid != "user-42" {
		t.Fatalf("expected OK with user-42, got %v / %q", code, uid)
	}
}
