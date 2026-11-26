package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"users-service/config"
	"users-service/domain"
	"users-service/handler"
	"users-service/repository"
	"users-service/service"

	userspb "stashr/proto/users"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {

	cfg := config.LoadConfig()
	service.Init(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.MongoConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB is unreachable: %v", err)
	}
	log.Println("Successfully connected to MongoDB database!")

	db := client.Database(cfg.MongoDBName)
	userRepo, err := repository.NewUserRepository(db, cfg.MongoConnectTimeout)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	quotaRequestRepo, err := repository.NewQuotaRequestRepository(db, cfg.MongoConnectTimeout)
	if err != nil {
		log.Fatalf("Failed to initialize quota request repository: %v", err)
	}

	userHandler := handler.NewUserHandler(
		userRepo,
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPFrom,
		cfg.DefaultQuotaBytes,
	)

	if cfg.AdminPassword != "" {
		hash, err := service.HashPassword(cfg.AdminPassword)
		if err != nil {
			log.Fatalf("Failed to hash admin password: %v", err)
		}
		now := time.Now()
		admin := &domain.User{
			FirstName:           "Admin",
			LastName:            "User",
			Email:               cfg.AdminEmail,
			PasswordHash:        hash,
			Role:                domain.RoleAdmin,
			IsVerified:          true,
			StorageQuotaBytes:   cfg.DefaultQuotaBytes,
			PasswordLastChanged: now,
			CreatedAt:           now,
		}
		if err := userRepo.SeedAdmin(context.Background(), admin); err != nil {
			log.Fatalf("Failed to seed admin user: %v", err)
		}
		log.Printf("Admin user seeded/verified: %s", cfg.AdminEmail)
	}

	// gRPC server: users-service is no longer an HTTP server. The api-gateway
	// is the only HTTP(S) entry point and talks to us over gRPC (HTTP/2).
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("Failed to listen on :%s: %v", cfg.Port, err)
	}

	srv := grpc.NewServer()
	userspb.RegisterUsersServiceServer(srv, userHandler) // generated in users_service_grpc.pb.go

	quotaRequestHandler := handler.NewQuotaRequestHandler(quotaRequestRepo, userRepo, cfg.Limits)
	quotaRequestHandler.SetRollbackTimeout(cfg.QuotaRequestRollbackTimeout)
	userspb.RegisterQuotaRequestServiceServer(srv, quotaRequestHandler)

	// Internal quota rpcs for storage-service (no REST binding, see users_service.proto).
	userspb.RegisterQuotaServiceServer(srv, handler.NewQuotaHandler(userRepo))

	// Standard gRPC health service (replaces the old /ping endpoint).
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus(userspb.UsersService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus(userspb.QuotaService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus(userspb.QuotaRequestService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)

	// Server reflection lets tools like grpcurl list and call services
	// without having the .proto files locally.
	reflection.Register(srv)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Println("Shutting down Users gRPC service...")
		srv.GracefulStop()
	}()

	log.Printf("Users gRPC service listening on :%s", cfg.Port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}
