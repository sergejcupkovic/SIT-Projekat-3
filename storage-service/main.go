package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"storage-service/handler"
	"storage-service/middleware"
	"storage-service/repository"
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

	db := client.Database(dbName)
	folderRepo := repository.NewFolderRepository(db)
	folderHandler := handler.NewFolderHandler(folderRepo)

	secretBytes := []byte(jwtSecret)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/folders", middleware.AuthMiddleware(secretBytes, folderHandler.HandleFolders))
	mux.HandleFunc("/api/folders/", middleware.AuthMiddleware(secretBytes, folderHandler.HandleFolderByID))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Storage Service running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}