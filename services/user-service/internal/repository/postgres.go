package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Nalatka/GoMovieService/services/user-service/internal/domain"
	"github.com/Nalatka/GoMovieService/services/user-service/internal/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateUser(ctx context.Context, email string, username string, passwordHash string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, username, password)
		VALUES ($1, $2, $3)
		RETURNING id::text, email, username, password, created_at, updated_at
	`, email, username, passwordHash).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, mapError(err)
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, email, username, password, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, mapError(err)
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, email, username, password, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, mapError(err)
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, id string, username string, email string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `
		UPDATE users
		SET username = $2, email = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, email, username, password, created_at, updated_at
	`, id, username, email).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, mapError(err)
}

func (r *PostgresRepository) DeleteUser(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return usecase.ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) GetWatchlist(ctx context.Context, userID string) ([]domain.WatchlistItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT movie_id::text, movie_id::text, added_at
		FROM watchlist
		WHERE user_id = $1
		ORDER BY added_at DESC
	`, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.WatchlistItem, 0)
	for rows.Next() {
		var item domain.WatchlistItem
		if err := rows.Scan(&item.MovieID, &item.Title, &item.AddedAt); err != nil {
			return nil, mapError(err)
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *PostgresRepository) AddToWatchlist(ctx context.Context, userID string, movieID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO watchlist (user_id, movie_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, movie_id) DO NOTHING
	`, userID, movieID)
	return mapError(err)
}

func (r *PostgresRepository) RemoveFromWatchlist(ctx context.Context, userID string, movieID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM watchlist WHERE user_id = $1 AND movie_id = $2`, userID, movieID)
	return mapError(err)
}

func (r *PostgresRepository) GetHistory(ctx context.Context, userID string, limit int32) ([]domain.HistoryItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT movie_id::text, movie_id::text, watched_seconds, watched_at
		FROM watch_history
		WHERE user_id = $1
		ORDER BY watched_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.HistoryItem, 0)
	for rows.Next() {
		var item domain.HistoryItem
		if err := rows.Scan(&item.MovieID, &item.Title, &item.WatchedSeconds, &item.WatchedAt); err != nil {
			return nil, mapError(err)
		}
		items = append(items, item)
	}
	return items, mapError(rows.Err())
}

func (r *PostgresRepository) AddToHistory(ctx context.Context, userID string, movieID string, watchedSeconds int32) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO watch_history (user_id, movie_id, watched_seconds)
		VALUES ($1, $2, $3)
	`, userID, movieID, watchedSeconds)
	return mapError(err)
}

func (r *PostgresRepository) GetRecommendations(ctx context.Context, userID string, limit int32) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT movie_id::text
		FROM watch_history
		WHERE user_id = $1
		ORDER BY movie_id::text
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapError(err)
		}
		ids = append(ids, id)
	}
	return ids, mapError(rows.Err())
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return usecase.ConflictError(err)
		}
		if strings.HasPrefix(pgErr.Code, "23") {
			return usecase.ErrInvalidInput
		}
	}
	return err
}
