package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"notifications-service/handler"
	"notifications-service/middleware"
	"notifications-service/repository"

	notificationspb "stashr/proto/notifications"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	hostsEnv := envOr("CASSANDRA_HOSTS", "cassandra")
	hosts := strings.Split(hostsEnv, ",")
	keyspace := envOr("CASSANDRA_KEYSPACE", "stashr_notifications")
	port := envOr("NOTIFICATIONS_GRPC_PORT", "50053")

	var repo *repository.CassandraNotificationRepository
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		repo, err = repository.NewCassandraNotificationRepository(hosts, keyspace)
		if err == nil {
			break
		}
		log.Printf("Waiting for Cassandra (%s) attempt %d/20: %v", hostsEnv, attempt, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra: %v", err)
	}
	defer repo.Close()
	log.Printf("Notifications service connected to Cassandra (%s, keyspace %q)", hostsEnv, keyspace)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on :%s: %v", port, err)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(middleware.AuthUnaryInterceptor([]byte(jwtSecret))),
	)
	notificationspb.RegisterNotificationServiceServer(srv, handler.NewNotificationHandler(repo))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus(notificationspb.NotificationService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	reflection.Register(srv)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Println("Shutting down Notifications gRPC service...")
		srv.GracefulStop()
	}()

	log.Printf("Notifications gRPC service listening on :%s", port)
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