package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"storage-service/handler"
	"storage-service/middleware"
	"storage-service/repository"
	"storage-service/service"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		usersServiceURL = "http://users-service:8081"
	}

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "minio:9000"
	}
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	if minioAccessKey == "" {
		minioAccessKey = "minioadmin"
	}
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	if minioSecretKey == "" {
		minioSecretKey = "minioadmin123"
	}
	minioBucket := os.Getenv("MINIO_BUCKET")
	if minioBucket == "" {
		minioBucket = "stashr-files"
	}
	minioUseSSL := os.Getenv("MINIO_USE_SSL") == "true"

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

	objectStore, err := service.NewObjectStore(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, minioUseSSL)
	if err != nil {
		log.Fatalf("Failed to initialize Object Store (MinIO): %v", err)
	}
	log.Printf("Storage service connected to MinIO bucket: %s", minioBucket)

	db := client.Database(dbName)
	folderRepo := repository.NewFolderRepository(db)
	fileRepo := repository.NewFileRepository(db)
	quotaClient := service.NewQuotaClient(usersServiceURL)

	folderHandler := handler.NewFolderHandler(folderRepo, fileRepo)
	fileHandler := handler.NewFileHandler(fileRepo, folderRepo, objectStore, quotaClient)

	secretBytes := []byte(jwtSecret)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/folders", middleware.AuthMiddleware(secretBytes, folderHandler.HandleFolders))
	mux.HandleFunc("/api/folders/", middleware.AuthMiddleware(secretBytes, folderHandler.HandleFolderByID))
	mux.HandleFunc("/api/files/upload", middleware.AuthMiddleware(secretBytes, fileHandler.UploadFile))
	mux.HandleFunc("/api/files/quota", middleware.AuthMiddleware(secretBytes, fileHandler.GetMyQuota))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Storage Service running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}