package integration

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/Nalatka/GoMovieService/proto/content"
	"github.com/google/uuid"
	"gomovieservice/services/content-service/internal/delivery/grpc"
	"gomovieservice/services/content-service/internal/domain"
	"gomovieservice/services/content-service/internal/usecase"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func newContentBufconnClient(t *testing.T, server *gogrpc.Server) pb.ContentServiceClient {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewContentServiceClient(conn)
}

// TestContentGRPCCreateAndListIntegration verifies the Create → List gRPC flow.
func TestContentGRPCCreateAndListIntegration(t *testing.T) {
	genreID := uuid.New()
	movieRepo := &memMovieRepo{movies: map[uuid.UUID]*domain.Movie{}}
	genreRepo := &memGenreRepo{genres: map[uuid.UUID]*domain.Genre{
		genreID: {ID: genreID, Name: "Action"},
	}}
	cache := newMemMovieCache()

	uc := usecase.NewContentUsecase(movieRepo, genreRepo, &memRatingRepo{}, cache, nil)
	server := gogrpc.NewServer()
	pb.RegisterContentServiceServer(server, grpc.NewContentHandler(uc))
	client := newContentBufconnClient(t, server)

	_, err := client.CreateMovie(context.Background(), &pb.CreateMovieRequest{
		Title:       "Movie 1",
		Description: "desc",
		Year:        2025,
		GenreId:     genreID.String(),
		VideoUrl:    "https://cdn/video.mp4",
		PosterUrl:   "https://cdn/poster.jpg",
		DurationSec: 120,
	})
	if err != nil {
		t.Fatalf("CreateMovie failed: %v", err)
	}

	list, err := client.ListMovies(context.Background(), &pb.ListMoviesRequest{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListMovies failed: %v", err)
	}
	if len(list.GetMovies()) != 1 {
		t.Fatalf("expected 1 movie in list, got %d", len(list.GetMovies()))
	}
	if list.GetMovies()[0].GetTitle() != "Movie 1" {
		t.Fatalf("unexpected title: %s", list.GetMovies()[0].GetTitle())
	}
}

// TestContentGRPCCacheHitIntegration verifies that a second GetMovie call is
// served from the cache (Redis in production) and does NOT hit the repository.
// This proves the cache layer is wired in and working end-to-end through gRPC.
func TestContentGRPCCacheHitIntegration(t *testing.T) {
	genreID := uuid.New()
	movieRepo := &memMovieRepo{movies: map[uuid.UUID]*domain.Movie{}}
	genreRepo := &memGenreRepo{genres: map[uuid.UUID]*domain.Genre{
		genreID: {ID: genreID, Name: "Action"},
	}}
	cache := newMemMovieCache()

	uc := usecase.NewContentUsecase(movieRepo, genreRepo, &memRatingRepo{}, cache, nil)
	server := gogrpc.NewServer()
	pb.RegisterContentServiceServer(server, grpc.NewContentHandler(uc))
	client := newContentBufconnClient(t, server)

	created, err := client.CreateMovie(context.Background(), &pb.CreateMovieRequest{
		Title:       "Cached Movie",
		Description: "will be cached",
		Year:        2024,
		GenreId:     genreID.String(),
		VideoUrl:    "https://cdn/v.mp4",
		PosterUrl:   "https://cdn/p.jpg",
		DurationSec: 90,
	})
	if err != nil {
		t.Fatalf("CreateMovie failed: %v", err)
	}
	movieID := created.GetMovie().GetId()

	// First GetMovie — should call repo (cache miss) and populate cache.
	_, err = client.GetMovie(context.Background(), &pb.GetMovieRequest{MovieId: movieID})
	if err != nil {
		t.Fatalf("GetMovie (1st) failed: %v", err)
	}
	hitsAfterFirst := movieRepo.getByIDCalls

	// Second GetMovie — should be served from cache, repo must NOT be called again.
	_, err = client.GetMovie(context.Background(), &pb.GetMovieRequest{MovieId: movieID})
	if err != nil {
		t.Fatalf("GetMovie (2nd) failed: %v", err)
	}

	if movieRepo.getByIDCalls != hitsAfterFirst {
		t.Fatalf("cache miss on second GetMovie: repo was called %d time(s) after first fetch (expected 0 additional calls)",
			movieRepo.getByIDCalls-hitsAfterFirst)
	}
}

// TestContentGRPCCacheInvalidatedOnRating verifies that rating a movie
// invalidates the cache so subsequent reads reflect up-to-date data.
func TestContentGRPCCacheInvalidatedOnRating(t *testing.T) {
	genreID := uuid.New()
	movieRepo := &memMovieRepo{movies: map[uuid.UUID]*domain.Movie{}}
	genreRepo := &memGenreRepo{genres: map[uuid.UUID]*domain.Genre{
		genreID: {ID: genreID, Name: "Drama"},
	}}
	cache := newMemMovieCache()

	uc := usecase.NewContentUsecase(movieRepo, genreRepo, &memRatingRepo{}, cache, nil)
	server := gogrpc.NewServer()
	pb.RegisterContentServiceServer(server, grpc.NewContentHandler(uc))
	client := newContentBufconnClient(t, server)

	created, err := client.CreateMovie(context.Background(), &pb.CreateMovieRequest{
		Title:       "Drama Film",
		Description: "a drama",
		Year:        2023,
		GenreId:     genreID.String(),
		VideoUrl:    "https://cdn/v.mp4",
		PosterUrl:   "https://cdn/p.jpg",
		DurationSec: 120,
	})
	if err != nil {
		t.Fatalf("CreateMovie failed: %v", err)
	}
	movieID := created.GetMovie().GetId()

	// Populate the cache
	if _, err = client.GetMovie(context.Background(), &pb.GetMovieRequest{MovieId: movieID}); err != nil {
		t.Fatalf("GetMovie failed: %v", err)
	}
	if cache.Size() == 0 {
		t.Fatal("expected movie to be in cache after GetMovie")
	}

	// RateMovie should invalidate the cache entry for this movie
	if _, err = client.RateMovie(context.Background(), &pb.RateMovieRequest{
		MovieId: movieID,
		UserId:  uuid.New().String(),
		Score:   9,
	}); err != nil {
		t.Fatalf("RateMovie failed: %v", err)
	}

	// After rating, the cache entry must have been evicted
	if cache.Size() != 0 {
		t.Fatal("expected cache to be invalidated after RateMovie")
	}
}

// ── In-memory stubs ──────────────────────────────────────────────────────────

type memMovieRepo struct {
	movies       map[uuid.UUID]*domain.Movie
	getByIDCalls int
}

func (m *memMovieRepo) Create(movie *domain.Movie) (*domain.Movie, error) {
	if movie.ID == uuid.Nil {
		movie.ID = uuid.New()
	}
	movie.CreatedAt = time.Now()
	cp := *movie
	m.movies[movie.ID] = &cp
	return &cp, nil
}

func (m *memMovieRepo) GetByID(id uuid.UUID) (*domain.Movie, error) {
	m.getByIDCalls++
	movie, ok := m.movies[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *movie
	return &cp, nil
}

func (m *memMovieRepo) Update(movie *domain.Movie) (*domain.Movie, error) {
	cp := *movie
	m.movies[movie.ID] = &cp
	return &cp, nil
}

func (m *memMovieRepo) Delete(id uuid.UUID) error {
	delete(m.movies, id)
	return nil
}

func (m *memMovieRepo) List(page, limit int32) ([]*domain.Movie, int32, error) {
	out := make([]*domain.Movie, 0, len(m.movies))
	for _, movie := range m.movies {
		cp := *movie
		out = append(out, &cp)
	}
	return out, int32(len(out)), nil
}

func (m *memMovieRepo) Search(_ string, page, limit int32) ([]*domain.Movie, int32, error) {
	return m.List(page, limit)
}

func (m *memMovieRepo) GetByGenre(genreID uuid.UUID, page, limit int32) ([]*domain.Movie, int32, error) {
	out := make([]*domain.Movie, 0)
	for _, movie := range m.movies {
		if movie.GenreID == genreID {
			cp := *movie
			out = append(out, &cp)
		}
	}
	return out, int32(len(out)), nil
}

func (m *memMovieRepo) GetTop(limit int32) ([]*domain.Movie, error) {
	out, _, _ := m.List(1, limit)
	return out, nil
}

func (m *memMovieRepo) GetSimilar(_ uuid.UUID, limit int32) ([]*domain.Movie, error) {
	out, _, _ := m.List(1, limit)
	return out, nil
}

func (m *memMovieRepo) IncrementViews(movieID uuid.UUID) error {
	if movie, ok := m.movies[movieID]; ok {
		movie.Views++
	}
	return nil
}

type memGenreRepo struct {
	genres map[uuid.UUID]*domain.Genre
}

func (m *memGenreRepo) GetAll() ([]*domain.Genre, error) {
	out := make([]*domain.Genre, 0, len(m.genres))
	for _, genre := range m.genres {
		cp := *genre
		out = append(out, &cp)
	}
	return out, nil
}

func (m *memGenreRepo) GetByID(id uuid.UUID) (*domain.Genre, error) {
	genre, ok := m.genres[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *genre
	return &cp, nil
}

type memRatingRepo struct{}

func (m *memRatingRepo) Upsert(_ *domain.Rating) (float64, int64, error) { return 8, 1, nil }
func (m *memRatingRepo) GetMovieRating(_ uuid.UUID) (float64, int64, error) {
	return 8, 1, nil
}
func (m *memRatingRepo) DeleteByUser(_ uuid.UUID) error { return nil }

// memMovieCache is a real in-memory cache used in integration tests to verify
// that the cache layer (Redis in production) is correctly wired through the usecase.
type memMovieCache struct {
	movies    map[uuid.UUID]*domain.Movie
	topMovies []*domain.Movie
}

func newMemMovieCache() *memMovieCache {
	return &memMovieCache{movies: map[uuid.UUID]*domain.Movie{}}
}

func (m *memMovieCache) Size() int { return len(m.movies) }

func (m *memMovieCache) GetMovie(id uuid.UUID) (*domain.Movie, error) {
	movie, ok := m.movies[id]
	if !ok {
		return nil, nil
	}
	cp := *movie
	return &cp, nil
}

func (m *memMovieCache) SetMovie(movie *domain.Movie) error {
	cp := *movie
	m.movies[movie.ID] = &cp
	return nil
}

func (m *memMovieCache) DeleteMovie(id uuid.UUID) error {
	delete(m.movies, id)
	return nil
}

func (m *memMovieCache) GetTopMovies() ([]*domain.Movie, error) {
	if m.topMovies == nil {
		return nil, nil
	}
	return m.topMovies, nil
}

func (m *memMovieCache) SetTopMovies(movies []*domain.Movie) error {
	m.topMovies = movies
	return nil
}

func (m *memMovieCache) InvalidateTop() error {
	m.topMovies = nil
	return nil
}
