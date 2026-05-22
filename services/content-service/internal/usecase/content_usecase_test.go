package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gomovieservice/services/content-service/internal/domain"
)

func TestCreateMovieValidatesTitleAndGenre(t *testing.T) {
	genreID := uuid.New()
	uc, movies, genres, _, cache, _ := newTestUsecase()
	genres.items[genreID] = &domain.Genre{ID: genreID, Name: "Action"}

	if _, err := uc.CreateMovie(&domain.Movie{GenreID: genreID}); !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("expected title error, got %v", err)
	}

	created, err := uc.CreateMovie(&domain.Movie{Title: "Movie", GenreID: genreID})
	if err != nil {
		t.Fatalf("CreateMovie returned error: %v", err)
	}
	if created.ID == uuid.Nil || movies.items[created.ID] == nil {
		t.Fatal("movie was not stored")
	}
	if cache.invalidTop == 0 {
		t.Fatal("top cache was not invalidated")
	}
}

func TestGetMovieUsesCacheBeforeRepository(t *testing.T) {
	id := uuid.New()
	uc, movies, _, _, cache, _ := newTestUsecase()
	cached := &domain.Movie{ID: id, Title: "Cached"}
	cache.movies[id] = cached
	movies.items[id] = &domain.Movie{ID: id, Title: "Database"}

	got, err := uc.GetMovie(id)
	if err != nil {
		t.Fatalf("GetMovie returned error: %v", err)
	}
	if got.Title != "Cached" || movies.gets != 0 {
		t.Fatal("cache was not used before repository")
	}
}

func TestRateMovieValidatesScorePublishesAndInvalidatesCache(t *testing.T) {
	movieID := uuid.New()
	userID := uuid.New()
	uc, movies, _, ratings, cache, publisher := newTestUsecase()
	movies.items[movieID] = &domain.Movie{ID: movieID, Title: "Movie"}

	if _, _, err := uc.RateMovie(movieID, userID, 11); !errors.Is(err, ErrInvalidScore) {
		t.Fatalf("expected invalid score, got %v", err)
	}

	avg, votes, err := uc.RateMovie(movieID, userID, 8)
	if err != nil {
		t.Fatalf("RateMovie returned error: %v", err)
	}
	if avg != 8 || votes != 1 {
		t.Fatalf("unexpected rating result: avg=%v votes=%v", avg, votes)
	}
	if publisher.movieID != movieID.String() || publisher.userID != userID.String() || publisher.score != 8 {
		t.Fatal("movie.rated event was not published")
	}
	if cache.deleted[movieID] == 0 || cache.invalidTop == 0 {
		t.Fatal("cache was not invalidated")
	}
	if ratings.ratings[movieID][userID] != 8 {
		t.Fatal("rating was not stored")
	}
}

func TestIncrementViewsAndDeleteUserRatings(t *testing.T) {
	movieID := uuid.New()
	userID := uuid.New()
	uc, movies, _, ratings, _, _ := newTestUsecase()
	movies.items[movieID] = &domain.Movie{ID: movieID, Title: "Movie"}
	ratings.ratings[movieID] = map[uuid.UUID]int32{userID: 7}

	if err := uc.IncrementViews(movieID); err != nil {
		t.Fatalf("IncrementViews returned error: %v", err)
	}
	if movies.items[movieID].Views != 1 {
		t.Fatal("views were not incremented")
	}
	if err := uc.DeleteUserRatings(userID); err != nil {
		t.Fatalf("DeleteUserRatings returned error: %v", err)
	}
	if _, ok := ratings.ratings[movieID][userID]; ok {
		t.Fatal("user rating was not deleted")
	}
}

func newTestUsecase() (*ContentUsecase, *fakeMovieRepo, *fakeGenreRepo, *fakeRatingRepo, *fakeMovieCache, *fakePublisher) {
	movies := &fakeMovieRepo{items: map[uuid.UUID]*domain.Movie{}}
	genres := &fakeGenreRepo{items: map[uuid.UUID]*domain.Genre{}}
	ratings := &fakeRatingRepo{ratings: map[uuid.UUID]map[uuid.UUID]int32{}}
	cache := &fakeMovieCache{movies: map[uuid.UUID]*domain.Movie{}, deleted: map[uuid.UUID]int{}}
	publisher := &fakePublisher{}
	return NewContentUsecase(movies, genres, ratings, cache, publisher), movies, genres, ratings, cache, publisher
}

type fakeMovieRepo struct {
	items map[uuid.UUID]*domain.Movie
	gets  int
}

func (r *fakeMovieRepo) Create(movie *domain.Movie) (*domain.Movie, error) {
	if movie.ID == uuid.Nil {
		movie.ID = uuid.New()
	}
	movie.CreatedAt = time.Now()
	r.items[movie.ID] = movie
	return movie, nil
}

func (r *fakeMovieRepo) GetByID(id uuid.UUID) (*domain.Movie, error) {
	r.gets++
	movie, ok := r.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return movie, nil
}

func (r *fakeMovieRepo) Update(movie *domain.Movie) (*domain.Movie, error) {
	if _, ok := r.items[movie.ID]; !ok {
		return nil, errors.New("not found")
	}
	r.items[movie.ID] = movie
	return movie, nil
}

func (r *fakeMovieRepo) Delete(id uuid.UUID) error {
	if _, ok := r.items[id]; !ok {
		return errors.New("not found")
	}
	delete(r.items, id)
	return nil
}

func (r *fakeMovieRepo) List(page, limit int32) ([]*domain.Movie, int32, error) {
	return []*domain.Movie{}, int32(len(r.items)), nil
}

func (r *fakeMovieRepo) Search(query string, page, limit int32) ([]*domain.Movie, int32, error) {
	return []*domain.Movie{}, 0, nil
}

func (r *fakeMovieRepo) GetByGenre(genreID uuid.UUID, page, limit int32) ([]*domain.Movie, int32, error) {
	return []*domain.Movie{}, 0, nil
}

func (r *fakeMovieRepo) GetTop(limit int32) ([]*domain.Movie, error) {
	return []*domain.Movie{}, nil
}

func (r *fakeMovieRepo) GetSimilar(movieID uuid.UUID, limit int32) ([]*domain.Movie, error) {
	return []*domain.Movie{}, nil
}

func (r *fakeMovieRepo) IncrementViews(movieID uuid.UUID) error {
	movie, ok := r.items[movieID]
	if !ok {
		return errors.New("not found")
	}
	movie.Views++
	return nil
}

type fakeGenreRepo struct {
	items map[uuid.UUID]*domain.Genre
}

func (r *fakeGenreRepo) GetAll() ([]*domain.Genre, error) {
	out := make([]*domain.Genre, 0, len(r.items))
	for _, genre := range r.items {
		out = append(out, genre)
	}
	return out, nil
}

func (r *fakeGenreRepo) GetByID(id uuid.UUID) (*domain.Genre, error) {
	genre, ok := r.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return genre, nil
}

type fakeRatingRepo struct {
	ratings map[uuid.UUID]map[uuid.UUID]int32
}

func (r *fakeRatingRepo) Upsert(rating *domain.Rating) (float64, int64, error) {
	if r.ratings[rating.MovieID] == nil {
		r.ratings[rating.MovieID] = map[uuid.UUID]int32{}
	}
	r.ratings[rating.MovieID][rating.UserID] = rating.Score
	return r.GetMovieRating(rating.MovieID)
}

func (r *fakeRatingRepo) GetMovieRating(movieID uuid.UUID) (float64, int64, error) {
	var total int32
	var count int64
	for _, score := range r.ratings[movieID] {
		total += score
		count++
	}
	if count == 0 {
		return 0, 0, nil
	}
	return float64(total) / float64(count), count, nil
}

func (r *fakeRatingRepo) DeleteByUser(userID uuid.UUID) error {
	for movieID := range r.ratings {
		delete(r.ratings[movieID], userID)
	}
	return nil
}

type fakeMovieCache struct {
	movies     map[uuid.UUID]*domain.Movie
	deleted    map[uuid.UUID]int
	invalidTop int
}

func (c *fakeMovieCache) GetMovie(id uuid.UUID) (*domain.Movie, error) {
	return c.movies[id], nil
}

func (c *fakeMovieCache) SetMovie(movie *domain.Movie) error {
	c.movies[movie.ID] = movie
	return nil
}

func (c *fakeMovieCache) DeleteMovie(id uuid.UUID) error {
	c.deleted[id]++
	delete(c.movies, id)
	return nil
}

func (c *fakeMovieCache) GetTopMovies() ([]*domain.Movie, error) {
	return nil, nil
}

func (c *fakeMovieCache) SetTopMovies(movies []*domain.Movie) error {
	return nil
}

func (c *fakeMovieCache) InvalidateTop() error {
	c.invalidTop++
	return nil
}

type fakePublisher struct {
	movieID string
	userID  string
	score   int32
	newAvg  float64
}

func (p *fakePublisher) PublishMovieRated(movieID string, userID string, score int32, newAvg float64) error {
	p.movieID = movieID
	p.userID = userID
	p.score = score
	p.newAvg = newAvg
	return nil
}
