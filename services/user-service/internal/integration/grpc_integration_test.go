package integration

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	userpb "github.com/Nalatka/GoMovieService/proto"
	"gomovieservice/services/user-service/internal/delivery/grpc"
	"gomovieservice/services/user-service/internal/domain"
	"gomovieservice/services/user-service/internal/usecase"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestUserGRPCRegisterAndGetUserIntegration(t *testing.T) {
	ctx := context.Background()
	repo := newMemUserRepo()
	service := usecase.NewService(repo, &memTokenStore{}, &memUserEvents{}, &memEmailSender{}, "secret")
	service.SetAdminEmails("admin@example.com")
	service.RegisterUser(ctx, "warmup@example.com", "warmup", "password1")

	server := gogrpc.NewServer()
	userpb.RegisterUserServiceServer(server, grpc.NewHandler(service))

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

	client := userpb.NewUserServiceClient(conn)
	reg, err := client.RegisterUser(ctx, &userpb.RegisterUserRequest{
		Email:    "Admin@Example.com",
		Username: "aidana",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}
	if reg.GetUser().GetRole() != "admin" {
		t.Fatalf("expected admin role, got %s", reg.GetUser().GetRole())
	}

	get, err := client.GetUser(ctx, &userpb.GetUserRequest{UserId: reg.GetUser().GetId()})
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if get.GetUser().GetUsername() != "aidana" {
		t.Fatalf("unexpected username: %s", get.GetUser().GetUsername())
	}
}

type memUserRepo struct {
	users   map[string]domain.User
	byEmail map[string]string
	next    int
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{
		users:   map[string]domain.User{},
		byEmail: map[string]string{},
	}
}

func (r *memUserRepo) CreateUser(_ context.Context, email, username, passwordHash, role string) (domain.User, error) {
	if _, exists := r.byEmail[email]; exists {
		return domain.User{}, usecase.ErrConflict
	}
	r.next++
	id := "user-" + time.Now().Format("150405") + "-" + strconv.Itoa(r.next)
	user := domain.User{
		ID:           id,
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	r.users[id] = user
	r.byEmail[email] = id
	return user, nil
}

func (r *memUserRepo) GetUserByID(_ context.Context, id string) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, usecase.ErrNotFound
	}
	return u, nil
}

func (r *memUserRepo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	id, ok := r.byEmail[email]
	if !ok {
		return domain.User{}, usecase.ErrNotFound
	}
	return r.users[id], nil
}

func (r *memUserRepo) UpdateUser(_ context.Context, id, username, email, role string) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, usecase.ErrNotFound
	}
	u.Username = username
	u.Email = email
	if role != "" {
		u.Role = role
	}
	r.users[id] = u
	r.byEmail[email] = id
	return u, nil
}

func (r *memUserRepo) DeleteUser(_ context.Context, id string) error {
	delete(r.users, id)
	return nil
}

func (r *memUserRepo) GetWatchlist(_ context.Context, _ string) ([]domain.WatchlistItem, error) {
	return []domain.WatchlistItem{}, nil
}

func (r *memUserRepo) AddToWatchlist(_ context.Context, _, _ string) error {
	return nil
}

func (r *memUserRepo) RemoveFromWatchlist(_ context.Context, _, _ string) error {
	return nil
}

func (r *memUserRepo) GetHistory(_ context.Context, _ string, _ int32) ([]domain.HistoryItem, error) {
	return []domain.HistoryItem{}, nil
}

func (r *memUserRepo) AddToHistory(_ context.Context, _, _ string, _ int32) error {
	return nil
}

func (r *memUserRepo) GetRecommendations(_ context.Context, _ string, _ int32) ([]string, error) {
	return []string{}, nil
}

type memTokenStore struct{}

func (m *memTokenStore) Save(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (m *memTokenStore) Delete(_ context.Context, _ string) error { return nil }

type memUserEvents struct{}

func (m *memUserEvents) PublishUserRegistered(_ context.Context, _ domain.User) error { return nil }
func (m *memUserEvents) PublishUserDeleted(_ context.Context, _ string) error         { return nil }

type memEmailSender struct{}

func (m *memEmailSender) SendWelcome(_ context.Context, _, _ string) error { return nil }
