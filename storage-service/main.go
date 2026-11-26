package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"storage-service/filecheck"
	"storage-service/handler"
	"storage-service/middleware"
	"storage-service/repository"
	"storage-service/service"
	"syscall"

	storagepb "stashr/proto/storage"

	"stashr/shared/appconfig"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://mongodb:27017"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// Object store (S3 API) that holds the file contents. Credentials have no
	// built-in default on purpose: docker-compose passes them from .env.
	objEndpoint := envOr("OBJECT_STORE_ENDPOINT", "object-store:9000")
	objAccessKey := os.Getenv("OBJECT_STORE_ACCESS_KEY")
	objSecretKey := os.Getenv("OBJECT_STORE_SECRET_KEY")
	if objAccessKey == "" || objSecretKey == "" {
		log.Fatal("OBJECT_STORE_ACCESS_KEY and OBJECT_STORE_SECRET_KEY environment variables are required")
	}
	usersAddr := envOr("USERS_GRPC_ADDR", "users-service:50051")

	// Every tunable comes from config.yaml (CONFIG_PATH).
	limits, cfg, err := loadConfig(appconfig.Path())
	if err != nil {
		log.Fatal(err)
	}
	filecheck.Configure(limits)
	handler.SetMaxFolderNameLength(limits.MaxFolderNameLength)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.MongoConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	log.Println("Storage service connected to MongoDB")

	objectStore, err := service.NewObjectStore(objEndpoint, objAccessKey, objSecretKey, cfg.ObjectStoreBucket,
		cfg.ObjectStoreUseSSL, cfg.ObjectStoreConnectAttempts, cfg.ObjectStoreConnectRetryDelay)
	if err != nil {
		log.Fatalf("Failed to initialize the object store: %v", err)
	}
	log.Printf("Storage service connected to object store %s (bucket %q)", objEndpoint, cfg.ObjectStoreBucket)

	// Client of users-service's internal QuotaService. grpc.NewClient connects
	// lazily and reconnects on its own, so start-up order does not matter.
	usersConn, err := grpc.NewClient(usersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create users-service gRPC client (%s): %v", usersAddr, err)
	}
	defer usersConn.Close()

	db := client.Database(cfg.MongoDBName)
	folderRepo := repository.NewFolderRepository(db)
	fileRepo := repository.NewFileRepository(db)

	folderHandler := handler.NewFolderHandler(folderRepo, fileRepo)
	quotaClient := service.NewQuotaClient(usersConn, cfg.QuotaServiceTimeout)
	fileHandler := handler.NewFileHandler(fileRepo, folderRepo, objectStore, quotaClient, limits.MaxUploadBytes)
	fileHandler.SetCleanupTimeout(cfg.UploadCleanupTimeout)

	port := os.Getenv("STORAGE_GRPC_PORT")
	if port == "" {
		port = "50052"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on :%s: %v", port, err)
	}

	// Every rpc (unary, and the streaming file upload) passes through the JWT
	// interceptor before reaching the handler.
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(middleware.AuthUnaryInterceptor([]byte(jwtSecret))),
		grpc.ChainStreamInterceptor(middleware.AuthStreamInterceptor([]byte(jwtSecret))),
	)
	storagepb.RegisterStorageServiceServer(srv, handler.NewStorageServer(folderHandler, fileHandler)) // generated in storage_service_grpc.pb.go

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus(storagepb.StorageService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)

	reflection.Register(srv)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Println("Shutting down Storage gRPC service...")
		srv.GracefulStop()
	}()

	log.Printf("Storage gRPC service listening on :%s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
