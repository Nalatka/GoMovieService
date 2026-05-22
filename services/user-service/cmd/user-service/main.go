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

	userpb "github.com/Nalatka/GoMovieService/proto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	deliverygrpc "gomovieservice/services/user-service/internal/delivery/grpc"
	"gomovieservice/services/user-service/internal/repository"
	"gomovieservice/services/user-service/internal/usecase"
	dbmigrations "gomovieservice/services/user-service/migrations"
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
	runMigrations(context.Background(), db, dbmigrations.FS)

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
