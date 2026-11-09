package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"users-service/config"
	"users-service/handler"
	"users-service/repository"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {

	cfg := config.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	userRepo, err := repository.NewUserRepository(db)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	userHandler := handler.NewUserHandler(
		userRepo,
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPFrom,
		cfg.DefaultQuotaBytes,
	)

	http.HandleFunc("/api/users/register", userHandler.Register)
	http.HandleFunc("/api/users/verify", userHandler.Verify)
	http.HandleFunc("/api/users/login", userHandler.Login)
	http.HandleFunc("/api/users/verify-otp", userHandler.VerifyOTP)
	http.HandleFunc("/api/users/resend-otp", userHandler.ResendOTP)
	http.HandleFunc("/api/users/logout", userHandler.Logout)
	
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})

	log.Printf("Users Service running on port %s...", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}