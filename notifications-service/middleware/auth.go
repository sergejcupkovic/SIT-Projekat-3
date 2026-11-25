package middleware

import (
	"context"
	"fmt"
	"strings"

	notificationspb "stashr/proto/notifications"

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

var notificationMethodPrefix = "/" + notificationspb.NotificationService_ServiceDesc.ServiceName + "/"

func AuthUnaryInterceptor(jwtSecret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !strings.HasPrefix(info.FullMethod, notificationMethodPrefix) {
			return handler(ctx, req)
		}

		ctx, err := authenticate(ctx, jwtSecret)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

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
		return nil, status.Error(codes.PermissionDenied, "Restricted token cannot access notifications")
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