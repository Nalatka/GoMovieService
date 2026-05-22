package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	_ "github.com/lib/pq"

	grpcDelivery "gomovieservice/services/content-service/internal/delivery/grpc"
	dbmigrations "gomovieservice/services/content-service/migrations"
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
	runMigrations(db, dbmigrations.FS)

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     firstNonEmpty(getEnv("REDIS_ADDR", ""), getEnv("REDIS_HOST", ""), "localhost:6379"),
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func runMigrations(db *sql.DB, fsys embed.FS) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	if err != nil {
		log.Fatalf("create schema_migrations: %v", err)
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		log.Fatalf("read migrations dir: %v", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		if err := db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", name,
		).Scan(&applied); err != nil {
			log.Fatalf("check migration %s: %v", name, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := fs.ReadFile(fsys, name)
		if err != nil {
			log.Fatalf("read migration %s: %v", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("begin tx %s: %v", name, err)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			log.Fatalf("exec migration %s: %v", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations(name) VALUES($1)", name); err != nil {
			_ = tx.Rollback()
			log.Fatalf("record migration %s: %v", name, err)
		}
		if err := tx.Commit(); err != nil {
			log.Fatalf("commit migration %s: %v", name, err)
		}
		log.Printf("applied migration: %s", name)
	}
}
