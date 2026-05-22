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

	streampb "github.com/Nalatka/GoMovieService/proto/stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type apiHandler struct {
	client streampb.StreamServiceClient
}

func main() {
	port := getenv("PORT", "8083")
	upstream := getenv("STREAM_SERVICE_ADDR", "stream-service:50053")
	addr := ":" + port

	conn, err := grpc.NewClient(upstream, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	handler := &apiHandler{client: streampb.NewStreamServiceClient(conn)}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.health(upstream))
	mux.HandleFunc("/streams/start", handler.startStream)
	mux.HandleFunc("/streams/qualities", handler.getQualities)
	mux.HandleFunc("/streams/subtitles", handler.getSubtitles)
	mux.HandleFunc("/streams/subtitles/lang", handler.getSubtitlesByLang)
	mux.HandleFunc("/streams/active", handler.getActiveSessions)
	mux.HandleFunc("/streams/preview", handler.getPreview)
	mux.HandleFunc("/streams/", handler.streamSession)

	log.Printf("stream-gateway listening on %s, upstream %s", addr, upstream)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (h *apiHandler) health(upstream string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"gateway": "stream-gateway", "status": "ok", "upstream": upstream})
	}
}

func (h *apiHandler) startStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		UserID  string `json:"user_id"`
		MovieID string `json:"movie_id"`
		Quality string `json:"quality"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.StartStream(ctx, &streampb.StartStreamRequest{UserId: body.UserID, MovieId: body.MovieID, Quality: qualityToProto(body.Quality)})
	writeGRPC(w, http.StatusCreated, resp, err)
}

func (h *apiHandler) streamSession(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	if len(parts) < 2 || parts[0] != "streams" {
		notFound(w)
		return
	}
	sessionID := parts[1]
	if len(parts) == 2 {
		h.sessionStatus(w, r, sessionID)
		return
	}
	if len(parts) != 3 {
		notFound(w)
		return
	}
	switch parts[2] {
	case "stop":
		h.stopStream(w, r, sessionID)
	case "pause":
		h.pauseStream(w, r, sessionID)
	case "resume":
		h.resumeStream(w, r, sessionID)
	case "seek":
		h.seekStream(w, r, sessionID)
	case "quality":
		h.setQuality(w, r, sessionID)
	default:
		notFound(w)
	}
}

func (h *apiHandler) sessionStatus(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetStreamStatus(ctx, &streampb.GetStreamStatusRequest{SessionId: sessionID})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) stopStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.StopStream(ctx, &streampb.StopStreamRequest{SessionId: sessionID})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) pauseStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		PositionSeconds int32 `json:"position_seconds"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.PauseStream(ctx, &streampb.PauseStreamRequest{SessionId: sessionID, PositionSeconds: body.PositionSeconds})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) resumeStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.ResumeStream(ctx, &streampb.ResumeStreamRequest{SessionId: sessionID})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) seekStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		PositionSeconds int32 `json:"position_seconds"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.SeekStream(ctx, &streampb.SeekStreamRequest{SessionId: sessionID, PositionSeconds: body.PositionSeconds})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) setQuality(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		Quality string `json:"quality"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.SetQuality(ctx, &streampb.SetQualityRequest{SessionId: sessionID, Quality: qualityToProto(body.Quality)})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) getQualities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetQualities(ctx, &streampb.GetQualitiesRequest{MovieId: r.URL.Query().Get("movie_id")})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) getSubtitles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetSubtitles(ctx, &streampb.GetSubtitlesRequest{MovieId: r.URL.Query().Get("movie_id")})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) getSubtitlesByLang(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetSubtitlesByLang(ctx, &streampb.GetSubtitlesByLangRequest{MovieId: r.URL.Query().Get("movie_id"), Lang: r.URL.Query().Get("lang")})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) getActiveSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetActiveSessions(ctx, &streampb.GetActiveSessionsRequest{Limit: int32(queryInt(r, "limit"))})
	writeGRPC(w, http.StatusOK, resp, err)
}

func (h *apiHandler) getPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	resp, err := h.client.GetPreview(ctx, &streampb.GetPreviewRequest{MovieId: r.URL.Query().Get("movie_id")})
	writeGRPC(w, http.StatusOK, resp, err)
}

func qualityToProto(quality string) streampb.Quality {
	switch strings.ToLower(quality) {
	case "480p":
		return streampb.Quality_Q_480P
	case "1080p":
		return streampb.Quality_Q_1080P
	default:
		return streampb.Quality_Q_720P
	}
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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
