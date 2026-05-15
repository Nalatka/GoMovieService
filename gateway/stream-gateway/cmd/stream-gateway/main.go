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
	port := getenv("PORT", "8083")
	upstream := getenv("STREAM_SERVICE_ADDR", "stream-service:50053")
	addr := ":" + port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{
			Gateway:  "stream-gateway",
			Status:   "ok",
			Upstream: upstream,
		})
	})

	log.Printf("stream-gateway listening on %s, upstream %s", addr, upstream)
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
