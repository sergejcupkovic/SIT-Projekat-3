package middleware

import (
	"context"
	"fmt"
	"strings"

	storagepb "stashr/proto/storage"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	UserIDKey contextKey = "userID"
	EmailKey  contextKey = "email"
	RoleKey   contextKey = "role"
)

// storageMethodPrefix is the gRPC full-method prefix of every StorageService
// rpc, e.g. "/storage.StorageService/CreateFolder". Only those require a JWT;
// infrastructure services (health checks, reflection) stay open.
var storageMethodPrefix = "/" + storagepb.StorageService_ServiceDesc.ServiceName + "/"

// AuthUnaryInterceptor is the gRPC equivalent of the old HTTP AuthMiddleware.
//
// A unary interceptor wraps every unary rpc call: it runs before the handler,
// can reject the call with a status error, or enrich ctx and call handler().
//
// The JWT arrives as gRPC metadata "authorization": grpc-gateway copies the
// HTTP "Authorization: Bearer <jwt>" header into it when translating REST to gRPC.
func AuthUnaryInterceptor(jwtSecret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !strings.HasPrefix(info.FullMethod, storageMethodPrefix) {
			return handler(ctx, req)
		}

		tokenStr, ok := bearerToken(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "Missing or invalid Authorization header")
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			return nil, status.Error(codes.Unauthenticated, "Invalid or expired session token")
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "Invalid token claims")
		}

		if restricted, ok := claims["restricted"].(bool); ok && restricted {
			return nil, status.Error(codes.PermissionDenied, "Restricted token cannot access storage. Please change your expired password first.")
		}

		userID, _ := claims["sub"].(string)
		email, _ := claims["email"].(string)
		role, _ := claims["role"].(string)

		if userID == "" {
			return nil, status.Error(codes.Unauthenticated, "Invalid user identity in token")
		}

		if role != "RK" {
			return nil, status.Error(codes.PermissionDenied, "Access restricted to regular users (RK)")
		}

		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, EmailKey, email)
		ctx = context.WithValue(ctx, RoleKey, role)

		return handler(ctx, req)
	}
}

func bearerToken(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	for _, v := range md.Get("authorization") {
		if strings.HasPrefix(v, "Bearer ") && len(v) > len("Bearer ") {
			return strings.TrimPrefix(v, "Bearer "), true
		}
	}
	return "", false
}
