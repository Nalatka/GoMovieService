package main

import (
	"log"
	"net/http"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Nalatka/GoMovieService/proto/content"
	"gomovieservice/gateway/content-gateway/internal/handler"
)

func main() {
	grpcAddr := getEnv("CONTENT_SERVICE_ADDR", getEnv("GRPC_ADDR", "localhost:50052"))
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("grpc dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewContentServiceClient(conn)
	h := handler.NewHandler(client)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /movies", h.CreateMovie)
	mux.HandleFunc("GET /movies/{id}", h.GetMovie)
	mux.HandleFunc("PUT /movies/{id}", h.UpdateMovie)
	mux.HandleFunc("DELETE /movies/{id}", h.DeleteMovie)
	mux.HandleFunc("GET /movies", h.ListMovies)

	mux.HandleFunc("GET /movies/search", h.SearchMovies)
	mux.HandleFunc("GET /genres", h.GetGenres)
	mux.HandleFunc("GET /genres/{genre_id}/movies", h.GetMoviesByGenre)

	mux.HandleFunc("POST /movies/{id}/rate", h.RateMovie)
	mux.HandleFunc("GET /movies/{id}/rating", h.GetMovieRating)

	mux.HandleFunc("GET /movies/top", h.GetTopMovies)
	mux.HandleFunc("GET /movies/{id}/similar", h.GetSimilarMovies)

	addr := ":" + getEnv("GATEWAY_PORT", "8082")
	log.Printf("Content Gateway HTTP listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
