// api-gateway is the single public entry point of the system.
//
//	client ──HTTPS (REST/JSON)──▶ api-gateway ──gRPC (protobuf, HTTP/2)──▶ users-service / storage-service
//
// It does NOT contain hand-written routes. The routes come from the
// google.api.http annotations in proto/*/*.proto: protoc-gen-grpc-gateway
// generated *.pb.gw.go files with Register<Service>Handler functions that
// add, for every rpc, an HTTP route that
//  1. decodes the JSON body / path params / query params into the protobuf request,
//  2. calls the rpc through a gRPC client connection,
//  3. encodes the protobuf reply (or gRPC status error) back to JSON + HTTP status.
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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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

	handler := securityHeaders(corsMiddleware(verifyRedirect(gw, frontendURL)))

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
