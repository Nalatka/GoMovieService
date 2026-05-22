package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gomovieservice/services/stream-service/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInvalidStatus = errors.New("invalid status")
)

type SessionRepository interface {
	CreateSession(ctx context.Context, session *domain.Session) error
	GetSessionByID(ctx context.Context, sessionID string) (*domain.Session, error)
	GetSessionsByUserID(ctx context.Context, userID string) ([]domain.Session, error)
	UpdateSession(ctx context.Context, session *domain.Session) error
	UpdateSessionStatus(ctx context.Context, sessionID string, status string) error
	UpdateSessionPosition(ctx context.Context, sessionID string, position int32) error
	UpdateSessionQuality(ctx context.Context, sessionID string, quality string) error
	DeleteSession(ctx context.Context, sessionID string) error
	GetActiveSessions(ctx context.Context, limit int) ([]domain.Session, error)
	FinishUserSessions(ctx context.Context, userID string) error
}

type SubtitleRepository interface {
	CreateSubtitle(ctx context.Context, subtitle *domain.Subtitle) error
	GetSubtitlesByMovieID(ctx context.Context, movieID string) ([]domain.Subtitle, error)
	GetSubtitleByLang(ctx context.Context, movieID string, lang string) (*domain.Subtitle, error)
	DeleteSubtitle(ctx context.Context, subtitleID string) error
}

type CacheRepository interface {
	GetSession(ctx context.Context, sessionID string) (*domain.Session, error)
	SetSession(ctx context.Context, sessionID string, session *domain.Session, ttlSeconds int) error
	DeleteSession(ctx context.Context, sessionID string) error
}

type EventPublisher interface {
	PublishStreamStarted(ctx context.Context, userID, movieID string) error
	PublishStreamCompleted(ctx context.Context, userID, movieID string, watchedSeconds int32) error
}

type Service struct {
	sessionRepo  SessionRepository
	subtitleRepo SubtitleRepository
	cache        CacheRepository
	events       EventPublisher
}

func NewService(
	sessionRepo SessionRepository,
	subtitleRepo SubtitleRepository,
	cache CacheRepository,
	events EventPublisher,
) *Service {
	return &Service{
		sessionRepo:  sessionRepo,
		subtitleRepo: subtitleRepo,
		cache:        cache,
		events:       events,
	}
}

// StartStream creates a new streaming session
func (s *Service) StartStream(ctx context.Context, userID, movieID string, quality string) (domain.Session, error) {
	if userID == "" || movieID == "" {
		return domain.Session{}, ErrInvalidInput
	}

	if quality == "" {
		quality = domain.Quality720P
	}

	session := domain.Session{
		UserID:          userID,
		MovieID:         movieID,
		PositionSeconds: 0,
		Quality:         quality,
		Status:          domain.StatusPlaying,
		StartedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.sessionRepo.CreateSession(ctx, &session); err != nil {
		return domain.Session{}, err
	}

	// Cache session (2 hours TTL)
	_ = s.cache.SetSession(ctx, session.ID, &session, 7200)

	// Publish event
	if s.events != nil {
		_ = s.events.PublishStreamStarted(ctx, userID, movieID)
	}

	return session, nil
}

// StopStream closes a streaming session
func (s *Service) StopStream(ctx context.Context, sessionID string) (int32, error) {
	if sessionID == "" {
		return 0, ErrInvalidInput
	}

	session, err := s.sessionRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	if session == nil {
		return 0, ErrNotFound
	}

	watchedSeconds := session.PositionSeconds

	// Update status to finished
	if err := s.sessionRepo.UpdateSessionStatus(ctx, sessionID, domain.StatusFinished); err != nil {
		return 0, err
	}

	// Remove from cache
	_ = s.cache.DeleteSession(ctx, sessionID)

	// Publish event
	if s.events != nil && watchedSeconds > 80 {
		_ = s.events.PublishStreamCompleted(ctx, session.UserID, session.MovieID, watchedSeconds)
	}

	return watchedSeconds, nil
}

// GetStreamStatus retrieves current session status
func (s *Service) GetStreamStatus(ctx context.Context, sessionID string) (domain.Session, error) {
	if sessionID == "" {
		return domain.Session{}, ErrInvalidInput
	}

	// Try cache first
	if cached, err := s.cache.GetSession(ctx, sessionID); err == nil && cached != nil {
		return *cached, nil
	}

	// Get from database
	session, err := s.sessionRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return domain.Session{}, err
	}
	if session == nil {
		return domain.Session{}, ErrNotFound
	}

	// Cache the session
	_ = s.cache.SetSession(ctx, sessionID, session, 7200)

	return *session, nil
}

// PauseStream pauses a streaming session
func (s *Service) PauseStream(ctx context.Context, sessionID string, position int32) error {
	if sessionID == "" {
		return ErrInvalidInput
	}

	session, err := s.sessionRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrNotFound
	}

	session.Status = domain.StatusPaused
	session.PositionSeconds = position
	session.UpdatedAt = time.Now()

	if err := s.sessionRepo.UpdateSession(ctx, session); err != nil {
		return err
	}

	// Update cache
	_ = s.cache.SetSession(ctx, sessionID, session, 7200)

	return nil
}

// ResumeStream resumes a streaming session
func (s *Service) ResumeStream(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ErrInvalidInput
	}

	session, err := s.sessionRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrNotFound
	}

	session.Status = domain.StatusPlaying
	session.UpdatedAt = time.Now()

	if err := s.sessionRepo.UpdateSession(ctx, session); err != nil {
		return err
	}

	// Update cache
	_ = s.cache.SetSession(ctx, sessionID, session, 7200)

	return nil
}

// SeekStream seeks to a specific position
func (s *Service) SeekStream(ctx context.Context, sessionID string, position int32) error {
	if sessionID == "" {
		return ErrInvalidInput
	}
	if position < 0 {
		return errors.New("position cannot be negative")
	}

	if err := s.sessionRepo.UpdateSessionPosition(ctx, sessionID, position); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.cache.DeleteSession(ctx, sessionID)

	return nil
}

// GetQualities returns available qualities
func (s *Service) GetQualities(ctx context.Context, movieID string) ([]string, error) {
	if movieID == "" {
		return nil, ErrInvalidInput
	}

	// Return all available qualities
	return []string{domain.Quality480P, domain.Quality720P, domain.Quality1080P}, nil
}

// SetQuality changes the quality of a streaming session
func (s *Service) SetQuality(ctx context.Context, sessionID string, quality string) error {
	if sessionID == "" {
		return ErrInvalidInput
	}

	validQualities := map[string]bool{
		domain.Quality480P:  true,
		domain.Quality720P:  true,
		domain.Quality1080P: true,
	}
	if !validQualities[quality] {
		return errors.New("invalid quality")
	}

	if err := s.sessionRepo.UpdateSessionQuality(ctx, sessionID, quality); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.cache.DeleteSession(ctx, sessionID)

	return nil
}

// GetSubtitles returns all subtitles for a movie
func (s *Service) GetSubtitles(ctx context.Context, movieID string) ([]domain.Subtitle, error) {
	if movieID == "" {
		return nil, ErrInvalidInput
	}

	subtitles, err := s.subtitleRepo.GetSubtitlesByMovieID(ctx, movieID)
	if err != nil {
		return nil, err
	}

	if subtitles == nil {
		subtitles = []domain.Subtitle{}
	}

	return subtitles, nil
}

// GetSubtitlesByLang returns subtitles for a specific language
func (s *Service) GetSubtitlesByLang(ctx context.Context, movieID, lang string) (domain.Subtitle, error) {
	if movieID == "" || lang == "" {
		return domain.Subtitle{}, ErrInvalidInput
	}

	subtitle, err := s.subtitleRepo.GetSubtitleByLang(ctx, movieID, lang)
	if err != nil {
		return domain.Subtitle{}, err
	}
	if subtitle == nil {
		return domain.Subtitle{}, ErrNotFound
	}

	return *subtitle, nil
}

// GetActiveSessions returns all active sessions (for admin)
func (s *Service) GetActiveSessions(ctx context.Context, limit int) ([]domain.Session, error) {
	if limit <= 0 {
		limit = 100
	}

	sessions, err := s.sessionRepo.GetActiveSessions(ctx, limit)
	if err != nil {
		return nil, err
	}

	if sessions == nil {
		sessions = []domain.Session{}
	}

	return sessions, nil
}

// GetPreview returns preview and trailer URLs
func (s *Service) GetPreview(ctx context.Context, movieID string) (string, string, error) {
	if movieID == "" {
		return "", "", ErrInvalidInput
	}

	// Placeholder URLs - would typically call content service
	previewURL := fmt.Sprintf("https://cdn.example.com/movies/%s/preview.jpg", movieID)
	trailerURL := fmt.Sprintf("https://cdn.example.com/movies/%s/trailer.mp4", movieID)

	return previewURL, trailerURL, nil
}

// FinishUserSessions marks all user sessions as finished (for user deletion)
func (s *Service) FinishUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalidInput
	}

	return s.sessionRepo.FinishUserSessions(ctx, userID)
}
