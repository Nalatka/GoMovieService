package postgres

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"gomovieservice/services/content-service/internal/domain"

	_ "github.com/lib/pq"
)

type movieRepo struct {
	db *sql.DB
}

func NewMovieRepository(db *sql.DB) domain.MovieRepository {
	return &movieRepo{db: db}
}

func (r *movieRepo) Create(m *domain.Movie) (*domain.Movie, error) {
	query := `
       INSERT INTO movies (title, description, year, genre_id, video_url, poster_url, duration_sec)
       VALUES ($1,$2,$3,$4,$5,$6,$7)
       RETURNING id, title, description, year, genre_id, video_url, poster_url, duration_sec, views, created_at`
	row := r.db.QueryRow(query, m.Title, m.Description, m.Year, m.GenreID, m.VideoURL, m.PosterURL, m.DurationSec)
	return scanMovie(row)
}

func (r *movieRepo) GetByID(id uuid.UUID) (*domain.Movie, error) {
	query := `
       SELECT m.id, m.title, m.description, m.year, m.genre_id, g.name,
              m.video_url, m.poster_url, m.duration_sec, m.views,
              COALESCE((SELECT AVG(score) FROM ratings WHERE movie_id = m.id), 0),
              m.created_at
       FROM movies m
       JOIN genres g ON g.id = m.genre_id
       WHERE m.id = $1`
	row := r.db.QueryRow(query, id)
	return scanMovieFull(row)
}

func (r *movieRepo) Update(m *domain.Movie) (*domain.Movie, error) {
	query := `
       UPDATE movies SET title=$1, description=$2, year=$3, genre_id=$4,
                         video_url=$5, poster_url=$6, duration_sec=$7
       WHERE id=$8
       RETURNING id, title, description, year, genre_id, video_url, poster_url, duration_sec, views, created_at`
	row := r.db.QueryRow(query, m.Title, m.Description, m.Year, m.GenreID,
		m.VideoURL, m.PosterURL, m.DurationSec, m.ID)
	return scanMovie(row)
}

func (r *movieRepo) Delete(id uuid.UUID) error {
	res, err := r.db.Exec(`DELETE FROM movies WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("movie %s not found", id.String())
	}
	return nil
}

func (r *movieRepo) List(page, limit int32) ([]*domain.Movie, int32, error) {
	offset := (page - 1) * limit
	rows, err := r.db.Query(`
       SELECT m.id, m.title, m.description, m.year, m.genre_id, g.name,
              m.video_url, m.poster_url, m.duration_sec, m.views,
              COALESCE((SELECT AVG(score) FROM ratings WHERE movie_id = m.id), 0),
              m.created_at
       FROM movies m JOIN genres g ON g.id = m.genre_id
       ORDER BY m.created_at DESC
       LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	movies, err := scanMovies(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int32
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM movies`).Scan(&total)
	return movies, total, nil
}

func (r *movieRepo) Search(query string, page, limit int32) ([]*domain.Movie, int32, error) {
	offset := (page - 1) * limit
	rows, err := r.db.Query(`
       SELECT m.id, m.title, m.description, m.year, m.genre_id, g.name,
              m.video_url, m.poster_url, m.duration_sec, m.views,
              COALESCE((SELECT AVG(score) FROM ratings WHERE movie_id = m.id), 0),
              m.created_at
       FROM movies m JOIN genres g ON g.id = m.genre_id
       WHERE to_tsvector('english', m.title) @@ plainto_tsquery('english', $1)
       ORDER BY m.created_at DESC
       LIMIT $2 OFFSET $3`, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	movies, err := scanMovies(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int32
	_ = r.db.QueryRow(`
       SELECT COUNT(*) FROM movies
       WHERE to_tsvector('english', title) @@ plainto_tsquery('english', $1)`, query).Scan(&total)
	return movies, total, nil
}

func (r *movieRepo) GetByGenre(genreID uuid.UUID, page, limit int32) ([]*domain.Movie, int32, error) {
	offset := (page - 1) * limit
	rows, err := r.db.Query(`
       SELECT m.id, m.title, m.description, m.year, m.genre_id, g.name,
              m.video_url, m.poster_url, m.duration_sec, m.views,
              COALESCE((SELECT AVG(score) FROM ratings WHERE movie_id = m.id), 0),
              m.created_at
       FROM movies m JOIN genres g ON g.id = m.genre_id
       WHERE m.genre_id = $1
       ORDER BY m.created_at DESC
       LIMIT $2 OFFSET $3`, genreID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	movies, err := scanMovies(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int32
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM movies WHERE genre_id=$1`, genreID).Scan(&total)
	return movies, total, nil
}

func (r *movieRepo) GetTop(limit int32) ([]*domain.Movie, error) {
	rows, err := r.db.Query(`
       SELECT m.id, m.title, m.description, m.year, m.genre_id, g.name,
              m.video_url, m.poster_url, m.duration_sec, m.views,
              COALESCE(AVG(rt.score), 0) AS avg_rating,
              m.created_at
       FROM movies m
       JOIN genres g ON g.id = m.genre_id
       LEFT JOIN ratings rt ON rt.movie_id = m.id
       GROUP BY m.id, g.name
       ORDER BY avg_rating DESC
       LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMovies(rows)
}

func (r *movieRepo) GetSimilar(movieID uuid.UUID, limit int32) ([]*domain.Movie, error) {
	rows, err := r.db.Query(`
       SELECT m.id, m.title, m.description, m.year, m.genre_id, g.name,
              m.video_url, m.poster_url, m.duration_sec, m.views,
              COALESCE((SELECT AVG(score) FROM ratings WHERE movie_id = m.id), 0),
              m.created_at
       FROM movies m JOIN genres g ON g.id = m.genre_id
       WHERE m.genre_id = (SELECT genre_id FROM movies WHERE id=$1)
         AND m.id != $1
       ORDER BY m.views DESC
       LIMIT $2`, movieID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMovies(rows)
}

func (r *movieRepo) IncrementViews(movieID uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE movies SET views = views + 1 WHERE id=$1`, movieID)
	return err
}

// Genre Repository

type genreRepo struct {
	db *sql.DB
}

func NewGenreRepository(db *sql.DB) domain.GenreRepository {
	return &genreRepo{db: db}
}

func (r *genreRepo) GetAll() ([]*domain.Genre, error) {
	rows, err := r.db.Query(`SELECT id, name FROM genres ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var genres []*domain.Genre
	for rows.Next() {
		g := &domain.Genre{}
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		genres = append(genres, g)
	}
	return genres, nil
}

func (r *genreRepo) GetByID(id uuid.UUID) (*domain.Genre, error) {
	g := &domain.Genre{}
	err := r.db.QueryRow(`SELECT id, name FROM genres WHERE id=$1`, id).Scan(&g.ID, &g.Name)
	if err != nil {
		return nil, fmt.Errorf("genre not found: %w", err)
	}
	return g, nil
}

// Rating Repository (with transactions)

type ratingRepo struct {
	db *sql.DB
}

func NewRatingRepository(db *sql.DB) domain.RatingRepository {
	return &ratingRepo{db: db}
}

func (r *ratingRepo) Upsert(rating *domain.Rating) (float64, int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.Exec(`
       INSERT INTO ratings (movie_id, user_id, score)
       VALUES ($1, $2, $3)
       ON CONFLICT (movie_id, user_id) DO UPDATE SET score = EXCLUDED.score`,
		rating.MovieID, rating.UserID, rating.Score)
	if err != nil {
		return 0, 0, err
	}

	var avgRating float64
	var totalVotes int64
	err = tx.QueryRow(`
       SELECT COALESCE(AVG(score), 0), COUNT(*)
       FROM ratings WHERE movie_id = $1`, rating.MovieID).Scan(&avgRating, &totalVotes)
	if err != nil {
		return 0, 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, 0, err
	}
	return avgRating, totalVotes, nil
}

func (r *ratingRepo) GetMovieRating(movieID uuid.UUID) (float64, int64, error) {
	var avgRating float64
	var totalVotes int64
	err := r.db.QueryRow(`
       SELECT COALESCE(AVG(score), 0), COUNT(*)
       FROM ratings WHERE movie_id = $1`, movieID).Scan(&avgRating, &totalVotes)
	return avgRating, totalVotes, err
}

func (r *ratingRepo) DeleteByUser(userID uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM ratings WHERE user_id = $1`, userID)
	return err
}

// helpers

func scanMovie(row *sql.Row) (*domain.Movie, error) {
	m := &domain.Movie{}
	err := row.Scan(&m.ID, &m.Title, &m.Description, &m.Year, &m.GenreID,
		&m.VideoURL, &m.PosterURL, &m.DurationSec, &m.Views, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func scanMovieFull(row *sql.Row) (*domain.Movie, error) {
	m := &domain.Movie{}
	err := row.Scan(&m.ID, &m.Title, &m.Description, &m.Year, &m.GenreID, &m.GenreName,
		&m.VideoURL, &m.PosterURL, &m.DurationSec, &m.Views, &m.AvgRating, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func scanMovies(rows *sql.Rows) ([]*domain.Movie, error) {
	var movies []*domain.Movie
	for rows.Next() {
		m := &domain.Movie{}
		if err := rows.Scan(&m.ID, &m.Title, &m.Description, &m.Year, &m.GenreID, &m.GenreName,
			&m.VideoURL, &m.PosterURL, &m.DurationSec, &m.Views, &m.AvgRating, &m.CreatedAt); err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}
