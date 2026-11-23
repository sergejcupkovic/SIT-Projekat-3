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

		ctx, err := authenticate(ctx, jwtSecret)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// AuthStreamInterceptor does the same for streaming rpcs (the file upload is
// client-streaming). The JWT is checked once, before the handler runs and
// before a single byte of the stream is read.
func AuthStreamInterceptor(jwtSecret []byte) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !strings.HasPrefix(info.FullMethod, storageMethodPrefix) {
			return handler(srv, ss)
		}

		ctx, err := authenticate(ss.Context(), jwtSecret)
		if err != nil {
			return err
		}
		return handler(srv, &authedStream{ServerStream: ss, ctx: ctx})
	}
}

// authedStream hands the handler the context that carries the user identity.
type authedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authedStream) Context() context.Context { return s.ctx }

// authenticate validates the session JWT in ctx's metadata and returns ctx
// extended with the user's ID, e-mail and role.
func authenticate(ctx context.Context, jwtSecret []byte) (context.Context, error) {
	tokenStr, ok := bearerToken(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Missing or invalid Authorization header")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
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
	return ctx, nil
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
