package usecase

import (
	"errors"
	"fmt"

	"gomovieservice/services/content-service/internal/domain"

	"github.com/google/uuid"
)

var (
	ErrMovieNotFound = errors.New("movie not found")
	ErrGenreNotFound = errors.New("genre not found")
	ErrInvalidScore  = errors.New("score must be between 1 and 10")
	ErrInvalidPage   = errors.New("page must be >= 1")
	ErrInvalidLimit  = errors.New("limit must be between 1 and 100")
	ErrTitleRequired = errors.New("title is required")
)

type ContentUsecase struct {
	movieRepo  domain.MovieRepository
	genreRepo  domain.GenreRepository
	ratingRepo domain.RatingRepository
	cache      domain.MovieCache
	publisher  NATSPublisher
}

// NATSPublisher is the interface for publishing events.
type NATSPublisher interface {
	PublishMovieRated(movieID string, userID string, score int32, newAvg float64) error
}

func NewContentUsecase(
	movieRepo domain.MovieRepository,
	genreRepo domain.GenreRepository,
	ratingRepo domain.RatingRepository,
	cache domain.MovieCache,
	publisher NATSPublisher,
) *ContentUsecase {
	return &ContentUsecase{
		movieRepo:  movieRepo,
		genreRepo:  genreRepo,
		ratingRepo: ratingRepo,
		cache:      cache,
		publisher:  publisher,
	}
}

// --- CreateMovie ---

func (uc *ContentUsecase) CreateMovie(movie *domain.Movie) (*domain.Movie, error) {
	if movie.Title == "" {
		return nil, ErrTitleRequired
	}
	if _, err := uc.genreRepo.GetByID(movie.GenreID); err != nil {
		return nil, fmt.Errorf("%w: genre_id %s", ErrGenreNotFound, movie.GenreID.String())
	}
	created, err := uc.movieRepo.Create(movie)
	if err != nil {
		return nil, err
	}
	_ = uc.cache.InvalidateTop()
	return created, nil
}

// --- GetMovie ---

func (uc *ContentUsecase) GetMovie(id uuid.UUID) (*domain.Movie, error) {
	if cached, err := uc.cache.GetMovie(id); err == nil && cached != nil {
		return cached, nil
	}
	movie, err := uc.movieRepo.GetByID(id)
	if err != nil {
		return nil, ErrMovieNotFound
	}
	_ = uc.cache.SetMovie(movie)
	return movie, nil
}

// --- UpdateMovie ---

func (uc *ContentUsecase) UpdateMovie(movie *domain.Movie) (*domain.Movie, error) {
	if movie.Title == "" {
		return nil, ErrTitleRequired
	}
	updated, err := uc.movieRepo.Update(movie)
	if err != nil {
		return nil, ErrMovieNotFound
	}
	_ = uc.cache.DeleteMovie(movie.ID)
	_ = uc.cache.InvalidateTop()
	return updated, nil
}

// --- DeleteMovie ---

func (uc *ContentUsecase) DeleteMovie(id uuid.UUID) error {
	if err := uc.movieRepo.Delete(id); err != nil {
		return ErrMovieNotFound
	}
	_ = uc.cache.DeleteMovie(id)
	_ = uc.cache.InvalidateTop()
	return nil
}

// --- ListMovies ---

func (uc *ContentUsecase) ListMovies(page, limit int32) ([]*domain.Movie, int32, error) {
	if err := validatePagination(page, limit); err != nil {
		return nil, 0, err
	}
	return uc.movieRepo.List(page, limit)
}

// --- SearchMovies ---

func (uc *ContentUsecase) SearchMovies(query string, page, limit int32) ([]*domain.Movie, int32, error) {
	if err := validatePagination(page, limit); err != nil {
		return nil, 0, err
	}
	return uc.movieRepo.Search(query, page, limit)
}

// --- GetGenres ---

func (uc *ContentUsecase) GetGenres() ([]*domain.Genre, error) {
	return uc.genreRepo.GetAll()
}

// --- GetMoviesByGenre ---

func (uc *ContentUsecase) GetMoviesByGenre(genreID uuid.UUID, page, limit int32) ([]*domain.Movie, int32, error) {
	if err := validatePagination(page, limit); err != nil {
		return nil, 0, err
	}
	return uc.movieRepo.GetByGenre(genreID, page, limit)
}

// --- RateMovie ---

func (uc *ContentUsecase) RateMovie(movieID, userID uuid.UUID, score int32) (float64, int64, error) {
	if score < 1 || score > 10 {
		return 0, 0, ErrInvalidScore
	}
	if _, err := uc.movieRepo.GetByID(movieID); err != nil {
		return 0, 0, ErrMovieNotFound
	}

	rating := &domain.Rating{
		MovieID: movieID,
		UserID:  userID,
		Score:   score,
	}
	newAvg, totalVotes, err := uc.ratingRepo.Upsert(rating)
	if err != nil {
		return 0, 0, err
	}

	_ = uc.cache.DeleteMovie(movieID)
	_ = uc.cache.InvalidateTop()

	_ = uc.publisher.PublishMovieRated(movieID.String(), userID.String(), score, newAvg)

	return newAvg, totalVotes, nil
}

// --- GetMovieRating ---

func (uc *ContentUsecase) GetMovieRating(movieID uuid.UUID) (float64, int64, error) {
	return uc.ratingRepo.GetMovieRating(movieID)
}

// --- GetTopMovies ---

func (uc *ContentUsecase) GetTopMovies(limit int32) ([]*domain.Movie, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if cached, err := uc.cache.GetTopMovies(); err == nil && len(cached) > 0 {
		if int32(len(cached)) >= limit {
			return cached[:limit], nil
		}
		return cached, nil
	}
	movies, err := uc.movieRepo.GetTop(limit)
	if err != nil {
		return nil, err
	}
	_ = uc.cache.SetTopMovies(movies)
	return movies, nil
}

// --- GetSimilarMovies ---

func (uc *ContentUsecase) GetSimilarMovies(movieID uuid.UUID, limit int32) ([]*domain.Movie, error) {
	if limit <= 0 {
		limit = 10
	}
	return uc.movieRepo.GetSimilar(movieID, limit)
}

// --- IncrementViews (called from NATS stream.started) ---

func (uc *ContentUsecase) IncrementViews(movieID uuid.UUID) error {
	if err := uc.movieRepo.IncrementViews(movieID); err != nil {
		return err
	}
	_ = uc.cache.DeleteMovie(movieID)
	return nil
}

// --- DeleteUserRatings (called from NATS user.deleted) ---

func (uc *ContentUsecase) DeleteUserRatings(userID uuid.UUID) error {
	return uc.ratingRepo.DeleteByUser(userID)
}

// --- helpers ---

func validatePagination(page, limit int32) error {
	if page < 1 {
		return ErrInvalidPage
	}
	if limit < 1 || limit > 100 {
		return ErrInvalidLimit
	}
	return nil
}
