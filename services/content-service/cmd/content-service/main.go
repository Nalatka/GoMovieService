package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/lib/pq"

	grpcDelivery "gomovieservice/services/content-service/internal/delivery/grpc"
	natsPkg "gomovieservice/services/content-service/internal/nats"
	pgRepo "gomovieservice/services/content-service/internal/repository/postgres"
	redisRepo "gomovieservice/services/content-service/internal/repository/redis"
	"gomovieservice/services/content-service/internal/usecase"

	pb "github.com/Nalatka/GoMovieService/proto/content"
)

func main() {
	// PostgreSQL
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_NAME", "contentdb"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	defer db.Close()

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
	})

	// NATS
	nc, err := nats.Connect(getEnv("NATS_URL", "nats://localhost:4222"))
	if err != nil {
		log.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("jetstream: %v", err)
	}

	// dependencies
	movieRepo := pgRepo.NewMovieRepository(db)
	genreRepo := pgRepo.NewGenreRepository(db)
	ratingRepo := pgRepo.NewRatingRepository(db)
	cache := redisRepo.NewMovieCache(rdb)
	publisher := natsPkg.NewPublisher(js)

	uc := usecase.NewContentUsecase(movieRepo, genreRepo, ratingRepo, cache, publisher)

	// --- NATS subscribers ---
	sub := natsPkg.NewSubscriber(js, uc)
	if err := sub.Subscribe(); err != nil {
		log.Fatalf("nats subscribe: %v", err)
	}

	// --- gRPC server ---
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", getEnv("GRPC_PORT", "50052")))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterContentServiceServer(grpcServer, grpcDelivery.NewContentHandler(uc))
	reflection.Register(grpcServer)

	log.Printf("Content Service gRPC listening on %s", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
