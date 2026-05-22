package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gomovieservice/services/stream-service/internal/domain"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Session operations

func (r *PostgresRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	query := `
		INSERT INTO sessions (user_id, movie_id, position_seconds, quality, status, started_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text
	`

	err := r.pool.QueryRow(ctx, query,
		session.UserID,
		session.MovieID,
		session.PositionSeconds,
		session.Quality,
		session.Status,
		session.StartedAt,
		session.UpdatedAt,
	).Scan(&session.ID)

	return mapError(err)
}

func (r *PostgresRepository) GetSessionByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	query := `
		SELECT id::text, user_id::text, movie_id::text, position_seconds, quality, status, started_at, updated_at
		FROM sessions
		WHERE id = $1::uuid
	`

	var session domain.Session
	err := r.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.MovieID,
		&session.PositionSeconds,
		&session.Quality,
		&session.Status,
		&session.StartedAt,
		&session.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &session, mapError(err)
}

func (r *PostgresRepository) GetSessionsByUserID(ctx context.Context, userID string) ([]domain.Session, error) {
	query := `
		SELECT id::text, user_id::text, movie_id::text, position_seconds, quality, status, started_at, updated_at
		FROM sessions
		WHERE user_id = $1::uuid
		ORDER BY updated_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		var session domain.Session
		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.MovieID,
			&session.PositionSeconds,
			&session.Quality,
			&session.Status,
			&session.StartedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, mapError(err)
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

func (r *PostgresRepository) UpdateSession(ctx context.Context, session *domain.Session) error {
	query := `
		UPDATE sessions
		SET position_seconds = $1, quality = $2, status = $3, updated_at = $4
		WHERE id = $5::uuid
	`

	_, err := r.pool.Exec(ctx, query,
		session.PositionSeconds,
		session.Quality,
		session.Status,
		time.Now(),
		session.ID,
	)

	return mapError(err)
}

func (r *PostgresRepository) UpdateSessionStatus(ctx context.Context, sessionID string, status string) error {
	query := `UPDATE sessions SET status = $1, updated_at = $2 WHERE id = $3::uuid`

	_, err := r.pool.Exec(ctx, query, status, time.Now(), sessionID)
	return mapError(err)
}

func (r *PostgresRepository) UpdateSessionPosition(ctx context.Context, sessionID string, position int32) error {
	query := `UPDATE sessions SET position_seconds = $1, updated_at = $2 WHERE id = $3::uuid`

	_, err := r.pool.Exec(ctx, query, position, time.Now(), sessionID)
	return mapError(err)
}

func (r *PostgresRepository) UpdateSessionQuality(ctx context.Context, sessionID string, quality string) error {
	query := `UPDATE sessions SET quality = $1, updated_at = $2 WHERE id = $3::uuid`

	_, err := r.pool.Exec(ctx, query, quality, time.Now(), sessionID)
	return mapError(err)
}

func (r *PostgresRepository) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1::uuid`

	_, err := r.pool.Exec(ctx, query, sessionID)
	return mapError(err)
}

func (r *PostgresRepository) GetActiveSessions(ctx context.Context, limit int) ([]domain.Session, error) {
	query := `
		SELECT id::text, user_id::text, movie_id::text, position_seconds, quality, status, started_at, updated_at
		FROM sessions
		WHERE status != $1
		ORDER BY updated_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, domain.StatusFinished, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		var session domain.Session
		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.MovieID,
			&session.PositionSeconds,
			&session.Quality,
			&session.Status,
			&session.StartedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, mapError(err)
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

func (r *PostgresRepository) FinishUserSessions(ctx context.Context, userID string) error {
	query := `
		UPDATE sessions
		SET status = $1, updated_at = $2
		WHERE user_id = $3::uuid AND status != $1
	`

	_, err := r.pool.Exec(ctx, query, domain.StatusFinished, time.Now(), userID)
	return mapError(err)
}

// Subtitle operations

func (r *PostgresRepository) CreateSubtitle(ctx context.Context, subtitle *domain.Subtitle) error {
	query := `
		INSERT INTO subtitles (movie_id, lang, label, file_url)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text
	`

	err := r.pool.QueryRow(ctx, query,
		subtitle.MovieID,
		subtitle.Lang,
		subtitle.Label,
		subtitle.FileURL,
	).Scan(&subtitle.ID)

	return mapError(err)
}

func (r *PostgresRepository) GetSubtitlesByMovieID(ctx context.Context, movieID string) ([]domain.Subtitle, error) {
	query := `
		SELECT id::text, movie_id::text, lang, label, file_url
		FROM subtitles
		WHERE movie_id = $1::uuid
		ORDER BY lang ASC
	`

	rows, err := r.pool.Query(ctx, query, movieID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var subtitles []domain.Subtitle
	for rows.Next() {
		var subtitle domain.Subtitle
		if err := rows.Scan(
			&subtitle.ID,
			&subtitle.MovieID,
			&subtitle.Lang,
			&subtitle.Label,
			&subtitle.FileURL,
		); err != nil {
			return nil, mapError(err)
		}
		subtitles = append(subtitles, subtitle)
	}

	return subtitles, rows.Err()
}

func (r *PostgresRepository) GetSubtitleByLang(ctx context.Context, movieID string, lang string) (*domain.Subtitle, error) {
	query := `
		SELECT id::text, movie_id::text, lang, label, file_url
		FROM subtitles
		WHERE movie_id = $1::uuid AND lang = $2
	`

	var subtitle domain.Subtitle
	err := r.pool.QueryRow(ctx, query, movieID, lang).Scan(
		&subtitle.ID,
		&subtitle.MovieID,
		&subtitle.Lang,
		&subtitle.Label,
		&subtitle.FileURL,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &subtitle, mapError(err)
}

func (r *PostgresRepository) DeleteSubtitle(ctx context.Context, subtitleID string) error {
	query := `DELETE FROM subtitles WHERE id = $1::uuid`

	_, err := r.pool.Exec(ctx, query, subtitleID)
	return mapError(err)
}

// mapError converts postgres errors to domain errors
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if err == pgx.ErrNoRows {
		return fmt.Errorf("not found")
	}
	return err
}
