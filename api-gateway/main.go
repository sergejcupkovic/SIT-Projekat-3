// The API gateway takes HTTPS REST requests and calls the services over gRPC.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	storagepb "stashr/proto/storage"
	userspb "stashr/proto/users"
	notificationspb "stashr/proto/notifications"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	usersAddr := env("USERS_GRPC_ADDR", "users-service:50051")
	storageAddr := env("STORAGE_GRPC_ADDR", "storage-service:50052")
	notificationsAddr := env("NOTIFICATIONS_GRPC_ADDR", "notifications-service:50053")
	port := env("GATEWAY_PORT", "8443")
	certFile := env("TLS_CERT_FILE", "/certs/server.crt")
	keyFile := env("TLS_KEY_FILE", "/certs/server.key")

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 chars")
	}
	cfg := loadConfig()

	usersConn, err := grpc.NewClient(usersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create users-service gRPC client (%s): %v", usersAddr, err)
	}
	defer usersConn.Close()

	storageConn, err := grpc.NewClient(storageAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create storage-service gRPC client (%s): %v", storageAddr, err)
	}
	defer storageConn.Close()

	notificationsConn, err := grpc.NewClient(notificationsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create notifications-service gRPC client (%s): %v", notificationsAddr, err)
	}
	defer notificationsConn.Close()

	runtime.DefaultContextTimeout = 15 * time.Second

	gw := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		// Lets services choose the HTTP success code (201 Created) - see response.go.
		runtime.WithForwardResponseOption(httpStatusFromMetadata),
	)

	if err := userspb.RegisterUsersServiceHandler(ctx, gw, usersConn); err != nil {
		log.Fatalf("Failed to register UsersService routes: %v", err)
	}
	if err := userspb.RegisterQuotaRequestServiceHandler(ctx, gw, usersConn); err != nil {
		log.Fatalf("Failed to register QuotaRequestService routes: %v", err)
	}
	if err := storagepb.RegisterStorageServiceHandler(ctx, gw, storageConn); err != nil {
		log.Fatalf("Failed to register StorageService routes: %v", err)
	}
	if err := notificationspb.RegisterNotificationServiceHandler(ctx, gw, notificationsConn); err != nil {
		log.Fatalf("Failed to register NotificationService routes: %v", err)
	}

	// POST /api/files/upload and POST /api/files/{file_id}/version: hand-written routes (multipart -> gRPC stream).
	up := newUploadHandler(gw, storagepb.NewStorageServiceClient(storageConn), []byte(jwtSecret), cfg.MaxUploadBytes)
	if err := gw.HandlePath(http.MethodPost, "/api/files/upload", up.handle); err != nil {
		log.Fatalf("Failed to register the upload route: %v", err)
	}
	if err := gw.HandlePath(http.MethodPost, "/api/files/{file_id}/version", up.handleVersion); err != nil {
		log.Fatalf("Failed to register the file version upload route: %v", err)
	}

	limiter := newRateLimiter(rate.Limit(cfg.RateLimitPerSecond), cfg.RateLimitBurst, []byte(jwtSecret))
	limiter.startCleanup()

	handler := securityHeaders(corsMiddleware(limiter.middleware(gw)))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down API Gateway...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("API Gateway (HTTPS) listening on :%s -> users=%s, storage=%s", port, usersAddr, storageAddr)
	// ListenAndServeTLS serves HTTPS only; plain http:// requests to this port fail the TLS handshake.
	if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("API Gateway error: %v", err)
	}
}
