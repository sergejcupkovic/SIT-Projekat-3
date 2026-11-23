package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"storage-service/handler"
	"storage-service/middleware"
	"storage-service/repository"
	"storage-service/service"
	"strconv"
	"syscall"
	"time"

	storagepb "stashr/proto/storage"

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

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "storage_db"
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
	objBucket := envOr("OBJECT_STORE_BUCKET", "stashr-files")
	objUseSSL := os.Getenv("OBJECT_STORE_USE_SSL") == "true"

	usersAddr := envOr("USERS_GRPC_ADDR", "users-service:50051")

	maxUploadBytes, err := strconv.ParseInt(envOr("MAX_UPLOAD_BYTES", "209715200"), 10, 64)
	if err != nil || maxUploadBytes <= 0 {
		log.Fatal("MAX_UPLOAD_BYTES must be a positive integer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	objectStore, err := service.NewObjectStore(objEndpoint, objAccessKey, objSecretKey, objBucket, objUseSSL)
	if err != nil {
		log.Fatalf("Failed to initialize the object store: %v", err)
	}
	log.Printf("Storage service connected to object store %s (bucket %q)", objEndpoint, objBucket)

	// Client of users-service's internal QuotaService. grpc.NewClient connects
	// lazily and reconnects on its own, so start-up order does not matter.
	usersConn, err := grpc.NewClient(usersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create users-service gRPC client (%s): %v", usersAddr, err)
	}
	defer usersConn.Close()

	db := client.Database(dbName)
	folderRepo := repository.NewFolderRepository(db)
	fileRepo := repository.NewFileRepository(db)

	folderHandler := handler.NewFolderHandler(folderRepo, fileRepo)
	fileHandler := handler.NewFileHandler(fileRepo, folderRepo, objectStore, service.NewQuotaClient(usersConn), maxUploadBytes)

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
