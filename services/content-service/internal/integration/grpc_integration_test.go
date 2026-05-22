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

func TestContentGRPCCreateAndListIntegration(t *testing.T) {
	genreID := uuid.New()
	movieRepo := &memMovieRepo{movies: map[uuid.UUID]*domain.Movie{}}
	genreRepo := &memGenreRepo{genres: map[uuid.UUID]*domain.Genre{
		genreID: {ID: genreID, Name: "Action"},
	}}
	ratingRepo := &memRatingRepo{}
	cache := &memMovieCache{}

	uc := usecase.NewContentUsecase(movieRepo, genreRepo, ratingRepo, cache, nil)
	server := gogrpc.NewServer()
	pb.RegisterContentServiceServer(server, grpc.NewContentHandler(uc))

	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewContentServiceClient(conn)
	_, err = client.CreateMovie(context.Background(), &pb.CreateMovieRequest{
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

type memMovieRepo struct {
	movies map[uuid.UUID]*domain.Movie
}

func (m *memMovieRepo) Create(movie *domain.Movie) (*domain.Movie, error) {
	if movie.ID == uuid.Nil {
		movie.ID = uuid.New()
	}
	movie.CreatedAt = time.Now()
	copy := *movie
	m.movies[movie.ID] = &copy
	return &copy, nil
}

func (m *memMovieRepo) GetByID(id uuid.UUID) (*domain.Movie, error) {
	movie, ok := m.movies[id]
	if !ok {
		return nil, errors.New("not found")
	}
	copy := *movie
	return &copy, nil
}

func (m *memMovieRepo) Update(movie *domain.Movie) (*domain.Movie, error) {
	copy := *movie
	m.movies[movie.ID] = &copy
	return &copy, nil
}

func (m *memMovieRepo) Delete(id uuid.UUID) error {
	delete(m.movies, id)
	return nil
}

func (m *memMovieRepo) List(page, limit int32) ([]*domain.Movie, int32, error) {
	out := make([]*domain.Movie, 0, len(m.movies))
	for _, movie := range m.movies {
		copy := *movie
		out = append(out, &copy)
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
			copy := *movie
			out = append(out, &copy)
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
		copy := *genre
		out = append(out, &copy)
	}
	return out, nil
}

func (m *memGenreRepo) GetByID(id uuid.UUID) (*domain.Genre, error) {
	genre, ok := m.genres[id]
	if !ok {
		return nil, errors.New("not found")
	}
	copy := *genre
	return &copy, nil
}

type memRatingRepo struct{}

func (m *memRatingRepo) Upsert(_ *domain.Rating) (float64, int64, error) { return 8, 1, nil }
func (m *memRatingRepo) GetMovieRating(_ uuid.UUID) (float64, int64, error) {
	return 8, 1, nil
}
func (m *memRatingRepo) DeleteByUser(_ uuid.UUID) error { return nil }

type memMovieCache struct{}

func (m *memMovieCache) GetMovie(_ uuid.UUID) (*domain.Movie, error) { return nil, nil }
func (m *memMovieCache) SetMovie(_ *domain.Movie) error              { return nil }
func (m *memMovieCache) DeleteMovie(_ uuid.UUID) error               { return nil }
func (m *memMovieCache) GetTopMovies() ([]*domain.Movie, error)      { return nil, nil }
func (m *memMovieCache) SetTopMovies(_ []*domain.Movie) error        { return nil }
func (m *memMovieCache) InvalidateTop() error                        { return nil }
