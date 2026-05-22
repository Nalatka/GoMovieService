package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	userpb "github.com/Nalatka/GoMovieService/proto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	deliverygrpc "gomovieservice/services/user-service/internal/delivery/grpc"
	"gomovieservice/services/user-service/internal/repository"
	"gomovieservice/services/user-service/internal/usecase"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	port := getenv("GRPC_PORT", "50051")
	addr := ":" + port

	db, err := pgxpool.New(ctx, databaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: getenv("REDIS_HOST", "localhost:6380")})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal(err)
	}

	natsConn, err := nats.Connect(getenv("NATS_URL", nats.DefaultURL))
	if err != nil {
		log.Fatal(err)
	}
	defer natsConn.Drain()

	events := repository.NewNATSEvents(natsConn)
	mailer := repository.NewSMTPMailer(getenv("SMTP_HOST", ""), getenv("SMTP_PORT", ""), getenv("SMTP_USERNAME", ""), getenv("SMTP_PASSWORD", ""), getenv("SMTP_FROM", ""))
	service := usecase.NewService(repository.NewPostgresRepository(db), repository.NewRedisTokenStore(redisClient), events, mailer, getenv("JWT_SECRET", "dev-secret"))
	service.SetAdminEmails(getenv("ADMIN_EMAILS", ""))
	if _, err := events.SubscribeStreamCompleted(service); err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	userpb.RegisterUserServiceServer(server, deliverygrpc.NewHandler(service))

	log.Printf("user-service listening on %s", addr)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func databaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getenv("DB_USER", "user"),
		getenv("DB_PASSWORD", "userpass"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5433"),
		getenv("DB_NAME", "userdb"),
	)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
