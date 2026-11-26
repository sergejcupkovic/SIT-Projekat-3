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

	"stashr/shared/appconfig"

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
	port := envOr("NOTIFICATIONS_GRPC_PORT", "50053")

	// Every tunable comes from config.yaml (CONFIG_PATH).
	cfg, options, err := loadConfig(appconfig.Path())
	if err != nil {
		log.Fatal(err)
	}

	var repo *repository.CassandraNotificationRepository
	for attempt := 1; attempt <= cfg.ConnectAttempts; attempt++ {
		repo, err = repository.NewCassandraNotificationRepository(hosts, cfg.Keyspace, options)
		if err == nil {
			break
		}
		log.Printf("Waiting for Cassandra (%s) attempt %d/%d: %v", hostsEnv, attempt, cfg.ConnectAttempts, err)
		time.Sleep(cfg.ConnectRetryDelay)
	}
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra: %v", err)
	}
	defer repo.Close()
	log.Printf("Notifications service connected to Cassandra (%s, keyspace %q)", hostsEnv, cfg.Keyspace)

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