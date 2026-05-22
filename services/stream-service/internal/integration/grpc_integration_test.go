package integration

import (
	"context"
	"net"
	"testing"
	"time"

	streampb "github.com/Nalatka/GoMovieService/proto/stream"
	"gomovieservice/services/stream-service/internal/delivery/grpc"
	"gomovieservice/services/stream-service/internal/domain"
	"gomovieservice/services/stream-service/internal/usecase"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestStreamGRPCStartPauseStopIntegration(t *testing.T) {
	sessionRepo := newMemSessionRepo()
	cache := &memStreamCache{sessions: map[string]*domain.Session{}}
	events := &memStreamEvents{}
	service := usecase.NewService(sessionRepo, sessionRepo, cache, events)

	server := gogrpc.NewServer()
	streampb.RegisterStreamServiceServer(server, grpc.NewHandler(service))

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

	client := streampb.NewStreamServiceClient(conn)
	start, err := client.StartStream(context.Background(), &streampb.StartStreamRequest{
		UserId:  "11111111-1111-1111-1111-111111111111",
		MovieId: "22222222-2222-2222-2222-222222222222",
		Quality: streampb.Quality_Q_720P,
	})
	if err != nil {
		t.Fatalf("StartStream failed: %v", err)
	}

	_, err = client.PauseStream(context.Background(), &streampb.PauseStreamRequest{
		SessionId:       start.GetSession().GetId(),
		PositionSeconds: 90,
	})
	if err != nil {
		t.Fatalf("PauseStream failed: %v", err)
	}

	stop, err := client.StopStream(context.Background(), &streampb.StopStreamRequest{
		SessionId: start.GetSession().GetId(),
	})
	if err != nil {
		t.Fatalf("StopStream failed: %v", err)
	}
	if !stop.GetSuccess() || stop.GetWatchedSeconds() != 90 {
		t.Fatalf("unexpected stop response: %+v", stop)
	}
	if events.completedMovieID == "" {
		t.Fatal("stream.completed event was not published")
	}
}

type memSessionRepo struct {
	sessions map[string]*domain.Session
	nextID   int
}

func newMemSessionRepo() *memSessionRepo {
	return &memSessionRepo{sessions: map[string]*domain.Session{}}
}

func (m *memSessionRepo) CreateSession(_ context.Context, session *domain.Session) error {
	m.nextID++
	session.ID = "session-" + time.Now().Format("150405")
	copy := *session
	m.sessions[session.ID] = &copy
	return nil
}

func (m *memSessionRepo) GetSessionByID(_ context.Context, sessionID string) (*domain.Session, error) {
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	copy := *s
	return &copy, nil
}

func (m *memSessionRepo) GetSessionsByUserID(_ context.Context, userID string) ([]domain.Session, error) {
	out := []domain.Session{}
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *memSessionRepo) UpdateSession(_ context.Context, session *domain.Session) error {
	copy := *session
	m.sessions[session.ID] = &copy
	return nil
}

func (m *memSessionRepo) UpdateSessionStatus(_ context.Context, sessionID, status string) error {
	if s, ok := m.sessions[sessionID]; ok {
		s.Status = status
		s.UpdatedAt = time.Now()
	}
	return nil
}

func (m *memSessionRepo) UpdateSessionPosition(_ context.Context, sessionID string, position int32) error {
	if s, ok := m.sessions[sessionID]; ok {
		s.PositionSeconds = position
		s.UpdatedAt = time.Now()
	}
	return nil
}

func (m *memSessionRepo) UpdateSessionQuality(_ context.Context, sessionID, quality string) error {
	if s, ok := m.sessions[sessionID]; ok {
		s.Quality = quality
		s.UpdatedAt = time.Now()
	}
	return nil
}

func (m *memSessionRepo) DeleteSession(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *memSessionRepo) GetActiveSessions(_ context.Context, limit int) ([]domain.Session, error) {
	out := []domain.Session{}
	for _, s := range m.sessions {
		if s.Status != domain.StatusFinished {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *memSessionRepo) FinishUserSessions(_ context.Context, userID string) error {
	for _, s := range m.sessions {
		if s.UserID == userID {
			s.Status = domain.StatusFinished
		}
	}
	return nil
}

func (m *memSessionRepo) CreateSubtitle(_ context.Context, _ *domain.Subtitle) error { return nil }
func (m *memSessionRepo) GetSubtitlesByMovieID(_ context.Context, _ string) ([]domain.Subtitle, error) {
	return []domain.Subtitle{}, nil
}
func (m *memSessionRepo) GetSubtitleByLang(_ context.Context, _, _ string) (*domain.Subtitle, error) {
	return nil, nil
}
func (m *memSessionRepo) DeleteSubtitle(_ context.Context, _ string) error { return nil }

type memStreamCache struct {
	sessions map[string]*domain.Session
}

func (m *memStreamCache) GetSession(_ context.Context, sessionID string) (*domain.Session, error) {
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	copy := *s
	return &copy, nil
}

func (m *memStreamCache) SetSession(_ context.Context, sessionID string, session *domain.Session, _ int) error {
	copy := *session
	m.sessions[sessionID] = &copy
	return nil
}

func (m *memStreamCache) DeleteSession(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

type memStreamEvents struct {
	completedMovieID string
}

func (m *memStreamEvents) PublishStreamStarted(_ context.Context, _, _ string) error { return nil }

func (m *memStreamEvents) PublishStreamCompleted(_ context.Context, _, movieID string, _ int32) error {
	m.completedMovieID = movieID
	return nil
}
