package domain

import (
	"time"

	"github.com/google/uuid"
)

type Genre struct {
	ID   uuid.UUID
	Name string
}

type Movie struct {
	ID          uuid.UUID
	Title       string
	Description string
	Year        int32
	GenreID     uuid.UUID
	GenreName   string
	VideoURL    string
	PosterURL   string
	DurationSec int32
	Views       int64
	AvgRating   float64
	CreatedAt   time.Time
}

type Rating struct {
	ID        uuid.UUID
	MovieID   uuid.UUID
	UserID    uuid.UUID
	Score     int32
	CreatedAt time.Time
}

type MovieRepository interface {
	Create(movie *Movie) (*Movie, error)
	GetByID(id uuid.UUID) (*Movie, error)
	Update(movie *Movie) (*Movie, error)
	Delete(id uuid.UUID) error
	List(page, limit int32) ([]*Movie, int32, error)
	Search(query string, page, limit int32) ([]*Movie, int32, error)
	GetByGenre(genreID uuid.UUID, page, limit int32) ([]*Movie, int32, error)
	GetTop(limit int32) ([]*Movie, error)
	GetSimilar(movieID uuid.UUID, limit int32) ([]*Movie, error)
	IncrementViews(movieID uuid.UUID) error
}

type GenreRepository interface {
	GetAll() ([]*Genre, error)
	GetByID(id uuid.UUID) (*Genre, error)
}

type RatingRepository interface {
	Upsert(rating *Rating) (avgRating float64, totalVotes int64, err error)
	GetMovieRating(movieID uuid.UUID) (avgRating float64, totalVotes int64, err error)
	DeleteByUser(userID uuid.UUID) error
}

type MovieCache interface {
	GetMovie(id uuid.UUID) (*Movie, error)
	SetMovie(movie *Movie) error
	DeleteMovie(id uuid.UUID) error
	GetTopMovies() ([]*Movie, error)
	SetTopMovies(movies []*Movie) error
	InvalidateTop() error
}
