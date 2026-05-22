package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	streampb "github.com/Nalatka/GoMovieService/proto/stream"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	deliverygrpc "gomovieservice/services/stream-service/internal/delivery/grpc"
	"gomovieservice/services/stream-service/internal/repository"
	"gomovieservice/services/stream-service/internal/usecase"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	port := getenv("GRPC_PORT", "50053")
	addr := ":" + port

	// Database connection
	db, err := pgxpool.New(ctx, databaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	// Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr: getenv("REDIS_HOST", "localhost:6379"),
	})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal(err)
	}

	// NATS connection
	natsConn, err := nats.Connect(getenv("NATS_URL", nats.DefaultURL))
	if err != nil {
		log.Fatal(err)
	}
	defer natsConn.Drain()

	// Initialize repositories
	postgresRepo := repository.NewPostgresRepository(db)
	redisCache := repository.NewRedisCache(redisClient)
	natsEvents := repository.NewNATSEvents(natsConn)

	// Subscribe to user.deleted event
	if err := natsEvents.SubscribeUserDeleted(ctx, func(ctx context.Context, userID string) error {
		return postgresRepo.FinishUserSessions(ctx, userID)
	}); err != nil {
		log.Fatal(err)
	}

	// Initialize service
	service := usecase.NewService(postgresRepo, postgresRepo, redisCache, natsEvents)

	// Initialize gRPC handler
	handler := deliverygrpc.NewHandler(service)

	// Start gRPC server
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	streampb.RegisterStreamServiceServer(server, handler)

	log.Printf("stream-service listening on %s", addr)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func databaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getenv("DB_USER", "stream"),
		getenv("DB_PASSWORD", "streampass"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "streamdb"),
	)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
