package usecase

import (
	"context"
	"testing"
	"time"

	"gomovieservice/services/stream-service/internal/domain"
)

func TestStartPauseStopStreamFlow(t *testing.T) {
	ctx := context.Background()
	sessionRepo := newFakeSessionRepo()
	cache := newFakeCache()
	events := &fakeEvents{}
	service := NewService(sessionRepo, sessionRepo, cache, events)

	session, err := service.StartStream(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", domain.Quality720P)
	if err != nil {
		t.Fatalf("StartStream returned error: %v", err)
	}
	if session.ID == "" || events.startedMovieID != session.MovieID {
		t.Fatal("stream was not started or event was not published")
	}

	if err := service.PauseStream(ctx, session.ID, 90); err != nil {
		t.Fatalf("PauseStream returned error: %v", err)
	}
	paused := sessionRepo.sessions[session.ID]
	if paused.Status != domain.StatusPaused || paused.PositionSeconds != 90 {
		t.Fatal("session was not paused at expected position")
	}

	watched, err := service.StopStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("StopStream returned error: %v", err)
	}
	if watched != 90 {
		t.Fatalf("unexpected watched seconds: %d", watched)
	}
	if sessionRepo.sessions[session.ID].Status != domain.StatusFinished {
		t.Fatal("session was not finished")
	}
	if events.completedMovieID != session.MovieID {
		t.Fatal("stream.completed event was not published")
	}
	if _, ok := cache.sessions[session.ID]; ok {
		t.Fatal("session was not removed from cache")
	}
}

func TestStopStreamDoesNotPublishCompletedBelowThreshold(t *testing.T) {
	ctx := context.Background()
	sessionRepo := newFakeSessionRepo()
	cache := newFakeCache()
	events := &fakeEvents{}
	service := NewService(sessionRepo, sessionRepo, cache, events)

	session, err := service.StartStream(ctx, "user-1", "movie-1", domain.Quality720P)
	if err != nil {
		t.Fatalf("StartStream returned error: %v", err)
	}
	if err := service.PauseStream(ctx, session.ID, 40); err != nil {
		t.Fatalf("PauseStream returned error: %v", err)
	}
	if _, err := service.StopStream(ctx, session.ID); err != nil {
		t.Fatalf("StopStream returned error: %v", err)
	}
	if events.completedMovieID != "" {
		t.Fatal("stream.completed was published below threshold")
	}
}

func TestFinishUserSessions(t *testing.T) {
	ctx := context.Background()
	sessionRepo := newFakeSessionRepo()
	service := NewService(sessionRepo, sessionRepo, newFakeCache(), nil)
	sessionRepo.sessions["s1"] = &domain.Session{ID: "s1", UserID: "user-1", Status: domain.StatusPlaying}
	sessionRepo.sessions["s2"] = &domain.Session{ID: "s2", UserID: "user-2", Status: domain.StatusPlaying}

	if err := service.FinishUserSessions(ctx, "user-1"); err != nil {
		t.Fatalf("FinishUserSessions returned error: %v", err)
	}
	if sessionRepo.sessions["s1"].Status != domain.StatusFinished {
		t.Fatal("user session was not finished")
	}
	if sessionRepo.sessions["s2"].Status == domain.StatusFinished {
		t.Fatal("unrelated session was changed")
	}
}

type fakeSessionRepo struct {
	sessions  map[string]*domain.Session
	subtitles map[string][]domain.Subtitle
	next      int
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[string]*domain.Session{}, subtitles: map[string][]domain.Subtitle{}}
}

func (r *fakeSessionRepo) CreateSession(_ context.Context, session *domain.Session) error {
	r.next++
	session.ID = "session-1"
	if r.next > 1 {
		session.ID = "session-2"
	}
	copy := *session
	r.sessions[session.ID] = &copy
	return nil
}

func (r *fakeSessionRepo) GetSessionByID(_ context.Context, sessionID string) (*domain.Session, error) {
	session := r.sessions[sessionID]
	if session == nil {
		return nil, nil
	}
	copy := *session
	return &copy, nil
}

func (r *fakeSessionRepo) GetSessionsByUserID(_ context.Context, userID string) ([]domain.Session, error) {
	var out []domain.Session
	for _, session := range r.sessions {
		if session.UserID == userID {
			out = append(out, *session)
		}
	}
	return out, nil
}

func (r *fakeSessionRepo) UpdateSession(_ context.Context, session *domain.Session) error {
	copy := *session
	copy.UpdatedAt = time.Now()
	r.sessions[session.ID] = &copy
	return nil
}

func (r *fakeSessionRepo) UpdateSessionStatus(_ context.Context, sessionID string, status string) error {
	r.sessions[sessionID].Status = status
	return nil
}

func (r *fakeSessionRepo) UpdateSessionPosition(_ context.Context, sessionID string, position int32) error {
	r.sessions[sessionID].PositionSeconds = position
	return nil
}

func (r *fakeSessionRepo) UpdateSessionQuality(_ context.Context, sessionID string, quality string) error {
	r.sessions[sessionID].Quality = quality
	return nil
}

func (r *fakeSessionRepo) DeleteSession(_ context.Context, sessionID string) error {
	delete(r.sessions, sessionID)
	return nil
}

func (r *fakeSessionRepo) GetActiveSessions(_ context.Context, limit int) ([]domain.Session, error) {
	var out []domain.Session
	for _, session := range r.sessions {
		if session.Status != domain.StatusFinished {
			out = append(out, *session)
		}
	}
	return out, nil
}

func (r *fakeSessionRepo) FinishUserSessions(_ context.Context, userID string) error {
	for _, session := range r.sessions {
		if session.UserID == userID {
			session.Status = domain.StatusFinished
		}
	}
	return nil
}

func (r *fakeSessionRepo) CreateSubtitle(_ context.Context, subtitle *domain.Subtitle) error {
	r.subtitles[subtitle.MovieID] = append(r.subtitles[subtitle.MovieID], *subtitle)
	return nil
}

func (r *fakeSessionRepo) GetSubtitlesByMovieID(_ context.Context, movieID string) ([]domain.Subtitle, error) {
	return r.subtitles[movieID], nil
}

func (r *fakeSessionRepo) GetSubtitleByLang(_ context.Context, movieID string, lang string) (*domain.Subtitle, error) {
	for _, subtitle := range r.subtitles[movieID] {
		if subtitle.Lang == lang {
			copy := subtitle
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *fakeSessionRepo) DeleteSubtitle(_ context.Context, subtitleID string) error {
	return nil
}

type fakeCache struct {
	sessions map[string]*domain.Session
}

func newFakeCache() *fakeCache {
	return &fakeCache{sessions: map[string]*domain.Session{}}
}

func (c *fakeCache) GetSession(_ context.Context, sessionID string) (*domain.Session, error) {
	return c.sessions[sessionID], nil
}

func (c *fakeCache) SetSession(_ context.Context, sessionID string, session *domain.Session, ttlSeconds int) error {
	copy := *session
	c.sessions[sessionID] = &copy
	return nil
}

func (c *fakeCache) DeleteSession(_ context.Context, sessionID string) error {
	delete(c.sessions, sessionID)
	return nil
}

type fakeEvents struct {
	startedMovieID   string
	completedMovieID string
}

func (e *fakeEvents) PublishStreamStarted(_ context.Context, userID, movieID string) error {
	e.startedMovieID = movieID
	return nil
}

func (e *fakeEvents) PublishStreamCompleted(_ context.Context, userID, movieID string, watchedSeconds int32) error {
	e.completedMovieID = movieID
	return nil
}
