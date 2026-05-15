package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type statusResponse struct {
	Gateway  string `json:"gateway"`
	Status   string `json:"status"`
	Upstream string `json:"upstream"`
}

func main() {
	port := getenv("PORT", "8081")
	upstream := getenv("USER_SERVICE_ADDR", "user-service:50051")
	addr := ":" + port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{
			Gateway:  "user-gateway",
			Status:   "ok",
			Upstream: upstream,
		})
	})

	log.Printf("user-gateway listening on %s, upstream %s", addr, upstream)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
