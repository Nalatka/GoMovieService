package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	streampb "github.com/Nalatka/GoMovieService/proto/stream"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	deliverygrpc "gomovieservice/services/stream-service/internal/delivery/grpc"
	"gomovieservice/services/stream-service/internal/repository"
	"gomovieservice/services/stream-service/internal/usecase"
	dbmigrations "gomovieservice/services/stream-service/migrations"
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
	runMigrations(context.Background(), db, dbmigrations.FS)

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

func runMigrations(ctx context.Context, pool *pgxpool.Pool, fsys embed.FS) {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
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
		if err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", name,
		).Scan(&applied); err != nil {
			log.Fatalf("check migration %s: %v", name, err)
		}
		if applied {
			continue
		}

		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			log.Fatalf("read migration %s: %v", name, err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Fatalf("begin tx %s: %v", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("exec migration %s: %v", name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations(name) VALUES($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("record migration %s: %v", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("commit migration %s: %v", name, err)
		}
		log.Printf("applied migration: %s", name)
	}
}
