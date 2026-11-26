// api-gateway is the single public entry point of the system.
//
//	client ──HTTPS (REST/JSON)──▶ api-gateway ──gRPC (protobuf, HTTP/2)──▶ users-service / storage-service
//
// Its routes come from the google.api.http annotations in proto/*/*.proto:
// protoc-gen-grpc-gateway generated *.pb.gw.go files with Register<Service>Handler
// functions that add, for every annotated rpc, an HTTP route that
//  1. decodes the JSON body / path params / query params into the protobuf request,
//  2. calls the rpc through a gRPC client connection,
//  3. encodes the protobuf reply (or gRPC status error) back to JSON + HTTP status.
//
// The single exception is the multipart file upload (upload.go), which
// grpc-gateway cannot decode; it is registered by hand and streams into the
// StorageService.UploadFile rpc.
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

	storagepb "stashr/proto/storage"
	userspb "stashr/proto/users"
	notificationspb "stashr/proto/notifications"

	"stashr/shared/appconfig"

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
	frontendURL := env("FRONTEND_URL", "https://localhost:3000")

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 chars")
	}
	// Every tunable comes from config.yaml (CONFIG_PATH).
	limits, cfg, err := loadConfig(appconfig.Path())
	if err != nil {
		log.Fatal(err)
	}

	// ---- gRPC clients (gateway -> services) ----
	// Plaintext HTTP/2 inside the private Docker network. grpc.NewClient does not
	// connect immediately; it connects on first use and reconnects automatically,
	// so the gateway can start before the services are up.
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

	// Upper bound for every backend call made on behalf of an HTTP request.
	runtime.DefaultContextTimeout = cfg.RequestTimeout

	// ---- REST <-> gRPC translator ----
	gw := runtime.NewServeMux(
		// JSON <-> protobuf conversion (protojson):
		//  * field names in lowerCamelCase (first_name -> "firstName"),
		//  * EmitUnpopulated: always output fields, e.g. "breadcrumbs": [] and
		//    "currentFolder": null, instead of omitting them,
		//  * DiscardUnknown: ignore JSON fields that are not in the .proto.
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
		// A quota too large to decode gets the range message - see errors.go.
		runtime.WithErrorHandler(quotaErrorHandler(limits.QuotaRangeMessage())),
	)

	// Generated in proto/users/users_service.pb.gw.go and proto/storage/storage_service.pb.gw.go.
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
	up := newUploadHandler(gw, storagepb.NewStorageServiceClient(storageConn), []byte(jwtSecret), limits.MaxUploadBytes)
	up.settings = cfg.Upload
	if err := gw.HandlePath(http.MethodPost, "/api/files/upload", up.handle); err != nil {
		log.Fatalf("Failed to register the upload route: %v", err)
	}
	if err := gw.HandlePath(http.MethodPost, "/api/files/{file_id}/version", up.handleVersion); err != nil {
		log.Fatalf("Failed to register the file version upload route: %v", err)
	}

	limiter := newRateLimiter(rate.Limit(cfg.RateLimitPerSecond), cfg.RateLimitBurst, []byte(jwtSecret))
	limiter.startCleanup(cfg.RateLimitCleanupInterval, cfg.RateLimitVisitorTTL)

	routes := limitRequestBody(cfg.MaxJSONBodyBytes, verifyRedirect(gw, frontendURL))
	handler := securityHeaders(corsMiddleware(cfg.CORSAllowedOrigin, limiter.middleware(routes)))

	// ---- HTTPS server (client -> gateway) ----
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down API Gateway...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("API Gateway (HTTPS) listening on :%s -> users=%s, storage=%s", port, usersAddr, storageAddr)
	// ListenAndServeTLS serves HTTPS only; plain http:// requests to this port fail the TLS handshake.
	if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("API Gateway error: %v", err)
	}
}
