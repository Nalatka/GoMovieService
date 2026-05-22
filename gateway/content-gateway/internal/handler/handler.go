package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	pb "github.com/Nalatka/GoMovieService/proto/content"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Handler struct {
	client pb.ContentServiceClient
}

func NewHandler(client pb.ContentServiceClient) *Handler {
	return &Handler{client: client}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func pathUUID(r *http.Request, key string) (string, error) {
	val := r.PathValue(key)
	if _, err := uuid.Parse(val); err != nil {
		return "", err
	}
	return val, nil
}

func queryInt32(r *http.Request, key string, def int32) int32 {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return def
	}
	return int32(v)
}

func (h *Handler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req pb.CreateMovieRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if _, err := uuid.Parse(req.GenreId); err != nil {
		writeError(w, http.StatusBadRequest, "invalid genre_id UUID format")
		return
	}

	resp, err := h.client.CreateMovie(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) GetMovie(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id format (expected UUID)")
		return
	}
	resp, err := h.client.GetMovie(r.Context(), &pb.GetMovieRequest{MovieId: id})
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) UpdateMovie(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id format (expected UUID)")
		return
	}
	var req pb.UpdateMovieRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.MovieId = id
	resp, err := h.client.UpdateMovie(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id format (expected UUID)")
		return
	}
	resp, err := h.client.DeleteMovie(r.Context(), &pb.DeleteMovieRequest{MovieId: id})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListMovies(w http.ResponseWriter, r *http.Request) {
	page := queryInt32(r, "page", 1)
	limit := queryInt32(r, "limit", 20)
	resp, err := h.client.ListMovies(r.Context(), &pb.ListMoviesRequest{Page: page, Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SearchMovies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page := queryInt32(r, "page", 1)
	limit := queryInt32(r, "limit", 20)
	resp, err := h.client.SearchMovies(r.Context(), &pb.SearchMoviesRequest{Query: q, Page: page, Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetGenres(w http.ResponseWriter, r *http.Request) {
	resp, err := h.client.GetGenres(r.Context(), &pb.GetGenresRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetMoviesByGenre(w http.ResponseWriter, r *http.Request) {
	genreID, err := pathUUID(r, "genre_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid genre_id format (expected UUID)")
		return
	}
	page := queryInt32(r, "page", 1)
	limit := queryInt32(r, "limit", 20)
	resp, err := h.client.GetMoviesByGenre(r.Context(), &pb.GetMoviesByGenreRequest{
		GenreId: genreID, Page: page, Limit: limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) RateMovie(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id format (expected UUID)")
		return
	}
	var body struct {
		UserID string `json:"user_id"`
		Score  int32  `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if _, err := uuid.Parse(body.UserID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id format (expected UUID)")
		return
	}

	resp, err := h.client.RateMovie(r.Context(), &pb.RateMovieRequest{
		MovieId: id, UserId: body.UserID, Score: body.Score,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetMovieRating(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id format (expected UUID)")
		return
	}
	resp, err := h.client.GetMovieRating(r.Context(), &pb.GetMovieRatingRequest{MovieId: id})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetTopMovies(w http.ResponseWriter, r *http.Request) {
	limit := queryInt32(r, "limit", 10)
	resp, err := h.client.GetTopMovies(r.Context(), &pb.GetTopMoviesRequest{Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetSimilarMovies(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id format (expected UUID)")
		return
	}
	limit := queryInt32(r, "limit", 10)
	resp, err := h.client.GetSimilarMovies(r.Context(), &pb.GetSimilarMoviesRequest{MovieId: id, Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return false
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return []byte(getenv("JWT_SECRET", "dev-secret")), nil
	})
	if err != nil || !parsed.Valid {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return false
	}
	if claims["role"] != "admin" {
		writeError(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
