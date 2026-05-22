package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	userpb "github.com/Nalatka/GoMovieService/proto"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	port := getenv("PORT", "8081")
	upstream := getenv("USER_SERVICE_ADDR", "user-service:50051")
	addr := ":" + port

	conn, err := grpc.NewClient(upstream, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	handler := &apiHandler{client: userpb.NewUserServiceClient(conn)}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.health(upstream))
	mux.HandleFunc("/users/register", handler.register)
	mux.HandleFunc("/users/login", handler.login)
	mux.HandleFunc("/users/logout", handler.logout)
	mux.HandleFunc("/users/", handler.users)

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

type apiHandler struct {
	client userpb.UserServiceClient
}

func (h *apiHandler) health(upstream string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"gateway": "user-gateway", "status": "ok", "upstream": upstream})
	}
}

func (h *apiHandler) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req userpb.RegisterUserRequest
	if !decode(w, r, &req) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.RegisterUser(ctx, &req)
	writeGRPC(w, http.StatusCreated, resp, err)
}

func (h *apiHandler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req userpb.LoginUserRequest
	if !decode(w, r, &req) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.LoginUser(ctx, &req)
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req userpb.LogoutUserRequest
	if !decode(w, r, &req) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.LogoutUser(ctx, &req)
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) users(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	if len(parts) < 2 || parts[0] != "users" || parts[1] == "" {
		notFound(w)
		return
	}
	userID := parts[1]
	switch {
	case len(parts) == 2:
		h.userProfile(w, r, userID)
	case len(parts) == 3 && parts[2] == "watchlist":
		h.watchlist(w, r, userID)
	case len(parts) == 4 && parts[2] == "watchlist":
		h.watchlistItem(w, r, userID, parts[3])
	case len(parts) == 3 && parts[2] == "history":
		h.history(w, r, userID)
	case len(parts) == 3 && parts[2] == "recommendations":
		h.recommendations(w, r, userID)
	default:
		notFound(w)
	}
}

func (h *apiHandler) userProfile(w http.ResponseWriter, r *http.Request, userID string) {
	ctx, cancel := requestContext(r)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		if !h.requireSelfOrAdmin(w, r, userID) {
			return
		}
		resp, err := h.client.GetUser(ctx, &userpb.GetUserRequest{UserId: userID})
		writeGRPC(w, http.StatusOK, resp, err)
	case http.MethodPatch:
		if !h.requireSelfOrAdmin(w, r, userID) {
			return
		}
		var body struct {
			Username string `json:"username"`
			Email    string `json:"email"`
		}
		if !decode(w, r, &body) {
			return
		}
		resp, err := h.client.UpdateUser(ctx, &userpb.UpdateUserRequest{UserId: userID, Username: body.Username, Email: body.Email})
		writeGRPC(w, http.StatusOK, resp, err)
	case http.MethodDelete:
		if !h.requireAdmin(w, r) {
			return
		}
		resp, err := h.client.DeleteUser(ctx, &userpb.DeleteUserRequest{UserId: userID})
		writeGRPC(w, http.StatusOK, resp, err)
	default:
		methodNotAllowed(w)
	}
}

func (h *apiHandler) watchlist(w http.ResponseWriter, r *http.Request, userID string) {
	if !h.requireSelfOrAdmin(w, r, userID) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		resp, err := h.client.GetWatchlist(ctx, &userpb.GetWatchlistRequest{UserId: userID})
		writeGRPC(w, http.StatusOK, resp, err)
	case http.MethodPost:
		var body struct {
			MovieID string `json:"movie_id"`
		}
		if !decode(w, r, &body) {
			return
		}
		resp, err := h.client.AddToWatchlist(ctx, &userpb.AddToWatchlistRequest{UserId: userID, MovieId: body.MovieID})
		writeGRPC(w, http.StatusCreated, resp, err)
	default:
		methodNotAllowed(w)
	}
}

func (h *apiHandler) watchlistItem(w http.ResponseWriter, r *http.Request, userID string, movieID string) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	if !h.requireSelfOrAdmin(w, r, userID) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.RemoveFromWatchlist(ctx, &userpb.RemoveFromWatchlistRequest{UserId: userID, MovieId: movieID})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) history(w http.ResponseWriter, r *http.Request, userID string) {
	if !h.requireSelfOrAdmin(w, r, userID) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		resp, err := h.client.GetHistory(ctx, &userpb.GetHistoryRequest{UserId: userID, Limit: int32(queryInt(r, "limit"))})
		writeGRPC(w, http.StatusOK, resp, err)
	case http.MethodPost:
		var body struct {
			MovieID        string `json:"movie_id"`
			WatchedSeconds int32  `json:"watched_seconds"`
		}
		if !decode(w, r, &body) {
			return
		}
		resp, err := h.client.AddToHistory(ctx, &userpb.AddToHistoryRequest{UserId: userID, MovieId: body.MovieID, WatchedSeconds: body.WatchedSeconds})
		writeGRPC(w, http.StatusCreated, resp, err)
	default:
		methodNotAllowed(w)
	}
}

func (h *apiHandler) recommendations(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !h.requireSelfOrAdmin(w, r, userID) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetRecommendations(ctx, &userpb.GetRecommendationsRequest{UserId: userID, Limit: int32(queryInt(r, "limit"))})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims, ok := parseClaims(w, r)
	if !ok {
		return false
	}
	if claims["role"] != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
		return false
	}
	return true
}

func (h *apiHandler) requireSelfOrAdmin(w http.ResponseWriter, r *http.Request, userID string) bool {
	claims, ok := parseClaims(w, r)
	if !ok {
		return false
	}
	if claims["role"] == "admin" || claims["sub"] == userID {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
	return false
}

func parseClaims(w http.ResponseWriter, r *http.Request) (jwt.MapClaims, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return nil, false
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return []byte(getenv("JWT_SECRET", "dev-secret")), nil
	})
	if err != nil || !parsed.Valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return nil, false
	}
	return claims, true
}

func requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func queryInt(r *http.Request, key string) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return false
	}
	return true
}

func writeGRPC(w http.ResponseWriter, successStatus int, payload any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, successStatus, payload)
}

func writeError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	code := http.StatusInternalServerError
	switch st.Code().String() {
	case "InvalidArgument":
		code = http.StatusBadRequest
	case "Unauthenticated":
		code = http.StatusUnauthorized
	case "NotFound":
		code = http.StatusNotFound
	case "AlreadyExists":
		code = http.StatusConflict
	}
	writeJSON(w, code, map[string]string{"error": st.Message()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
