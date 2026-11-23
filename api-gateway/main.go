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
	"strconv"
	"syscall"
	"time"

	storagepb "stashr/proto/storage"
	userspb "stashr/proto/users"

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
	port := env("GATEWAY_PORT", "8443")
	certFile := env("TLS_CERT_FILE", "/certs/server.crt")
	keyFile := env("TLS_KEY_FILE", "/certs/server.key")
	frontendURL := env("FRONTEND_URL", "https://localhost:3000")

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 chars")
	}
	ratePerSecond, err := strconv.ParseFloat(env("RATE_LIMIT_PER_SECOND", "1"), 64)
	if err != nil || ratePerSecond <= 0 {
		log.Fatal("RATE_LIMIT_PER_SECOND must be a positive number")
	}
	rateBurst, err := strconv.Atoi(env("RATE_LIMIT_BURST", "20"))
	if err != nil || rateBurst < 1 {
		log.Fatal("RATE_LIMIT_BURST must be a positive integer")
	}

	maxUploadBytes, err := strconv.ParseInt(env("MAX_UPLOAD_BYTES", "209715200"), 10, 64)
	if err != nil || maxUploadBytes <= 0 {
		log.Fatal("MAX_UPLOAD_BYTES must be a positive integer")
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

	// Upper bound for every backend call made on behalf of an HTTP request.
	runtime.DefaultContextTimeout = 15 * time.Second

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
	)

	// Generated in proto/users/users_service.pb.gw.go and proto/storage/storage_service.pb.gw.go.
	if err := userspb.RegisterUsersServiceHandler(ctx, gw, usersConn); err != nil {
		log.Fatalf("Failed to register UsersService routes: %v", err)
	}
	if err := storagepb.RegisterStorageServiceHandler(ctx, gw, storageConn); err != nil {
		log.Fatalf("Failed to register StorageService routes: %v", err)
	}

	// POST /api/files/upload: the one hand-written route (multipart -> gRPC stream).
	up := newUploadHandler(gw, storagepb.NewStorageServiceClient(storageConn), []byte(jwtSecret), maxUploadBytes)
	if err := gw.HandlePath(http.MethodPost, "/api/files/upload", up.handle); err != nil {
		log.Fatalf("Failed to register the upload route: %v", err)
	}

	limiter := newRateLimiter(rate.Limit(ratePerSecond), rateBurst, []byte(jwtSecret))
	limiter.startCleanup()

	handler := securityHeaders(corsMiddleware(limiter.middleware(verifyRedirect(gw, frontendURL))))

	// ---- HTTPS server (client -> gateway) ----
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
