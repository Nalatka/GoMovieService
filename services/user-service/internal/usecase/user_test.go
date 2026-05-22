package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gomovieservice/services/user-service/internal/domain"
)

func TestRegisterUserCreatesUserTokenEventAndEmail(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	tokens := &fakeTokens{}
	events := &fakeEvents{}
	email := &fakeEmail{}
	service := NewService(repo, tokens, events, email, "secret")
	service.bcryptCost = bcrypt.MinCost

	user, token, err := service.RegisterUser(ctx, "A@Example.COM", "aidana", "password1")
	if err != nil {
		t.Fatalf("RegisterUser returned error: %v", err)
	}
	if user.Email != "a@example.com" {
		t.Fatalf("email was not normalized: %s", user.Email)
	}
	if user.Role != "user" {
		t.Fatalf("expected user role, got %s", user.Role)
	}
	if token == "" {
		t.Fatal("token was empty")
	}
	if tokens.saved[token] != user.ID {
		t.Fatal("token was not saved")
	}
	if len(events.registered) != 1 || events.registered[0].ID != user.ID {
		t.Fatal("registration event was not published")
	}
	if email.to != user.Email {
		t.Fatal("welcome email was not sent")
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.users[user.ID].PasswordHash), []byte("password1")) != nil {
		t.Fatal("password was not hashed correctly")
	}
}

func TestRegisterUserAssignsAdminRoleFromConfiguredEmails(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	service := NewService(repo, &fakeTokens{}, nil, nil, "secret")
	service.bcryptCost = bcrypt.MinCost
	service.SetAdminEmails("admin@example.com")

	user, _, err := service.RegisterUser(ctx, "admin@example.com", "admin", "password1")
	if err != nil {
		t.Fatalf("RegisterUser returned error: %v", err)
	}
	if user.Role != "admin" {
		t.Fatalf("expected admin role, got %s", user.Role)
	}
}

func TestRegisterUserRejectsDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	service := NewService(repo, &fakeTokens{}, &fakeEvents{}, &fakeEmail{}, "secret")
	service.bcryptCost = bcrypt.MinCost

	if _, _, err := service.RegisterUser(ctx, "a@example.com", "first", "password1"); err != nil {
		t.Fatalf("first register returned error: %v", err)
	}
	if _, _, err := service.RegisterUser(ctx, "a@example.com", "second", "password1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestLoginUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	service := NewService(repo, &fakeTokens{}, &fakeEvents{}, &fakeEmail{}, "secret")
	service.bcryptCost = bcrypt.MinCost

	user, _, err := service.RegisterUser(ctx, "a@example.com", "aidana", "password1")
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	loggedIn, token, err := service.LoginUser(ctx, "a@example.com", "password1")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if loggedIn.ID != user.ID || token == "" {
		t.Fatal("login did not return expected user and token")
	}
	if _, _, err := service.LoginUser(ctx, "a@example.com", "wrongpass"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLogoutUserDeletesToken(t *testing.T) {
	tokens := &fakeTokens{saved: map[string]string{"token": "user-1"}}
	service := NewService(newFakeRepo(), tokens, nil, nil, "secret")
	if err := service.LogoutUser(context.Background(), "token"); err != nil {
		t.Fatalf("LogoutUser returned error: %v", err)
	}
	if _, ok := tokens.saved["token"]; ok {
		t.Fatal("token was not deleted")
	}
}

func TestUpdateUserChangesRoleWhenProvided(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	service := NewService(repo, &fakeTokens{}, &fakeEvents{}, &fakeEmail{}, "secret")
	service.bcryptCost = bcrypt.MinCost

	user, _, err := service.RegisterUser(ctx, "a@example.com", "aidana", "password1")
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	updated, err := service.UpdateUser(ctx, user.ID, "aidana2", "a2@example.com", "admin")
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if updated.Role != "admin" {
		t.Fatalf("expected role admin, got %s", updated.Role)
	}
}

func TestDeleteUserPublishesEvent(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	events := &fakeEvents{}
	service := NewService(repo, &fakeTokens{}, events, nil, "secret")
	service.bcryptCost = bcrypt.MinCost

	user, _, err := service.RegisterUser(ctx, "a@example.com", "aidana", "password1")
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if err := service.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if len(events.deleted) != 1 || events.deleted[0] != user.ID {
		t.Fatal("delete event was not published")
	}
}

func TestWatchlistHistoryAndRecommendations(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	service := NewService(repo, &fakeTokens{}, nil, nil, "secret")
	service.bcryptCost = bcrypt.MinCost

	user, _, err := service.RegisterUser(ctx, "a@example.com", "aidana", "password1")
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if err := service.AddToWatchlist(ctx, user.ID, "movie-1"); err != nil {
		t.Fatalf("AddToWatchlist returned error: %v", err)
	}
	watchlist, err := service.GetWatchlist(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetWatchlist returned error: %v", err)
	}
	if len(watchlist) != 1 || watchlist[0].MovieID != "movie-1" {
		t.Fatal("watchlist item was not returned")
	}
	if err := service.AddToHistory(ctx, user.ID, "movie-1", 100); err != nil {
		t.Fatalf("AddToHistory returned error: %v", err)
	}
	history, err := service.GetHistory(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 1 || history[0].WatchedSeconds != 100 {
		t.Fatal("history item was not returned")
	}
	recs, err := service.GetRecommendations(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("GetRecommendations returned error: %v", err)
	}
	if len(recs) != 1 || recs[0] != "movie-1" {
		t.Fatal("recommendations were not returned from history")
	}
}

type fakeRepo struct {
	users     map[string]domain.User
	byEmail   map[string]string
	watchlist map[string][]domain.WatchlistItem
	history   map[string][]domain.HistoryItem
	next      int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:     map[string]domain.User{},
		byEmail:   map[string]string{},
		watchlist: map[string][]domain.WatchlistItem{},
		history:   map[string][]domain.HistoryItem{},
	}
}

func (r *fakeRepo) CreateUser(_ context.Context, email string, username string, passwordHash string, role string) (domain.User, error) {
	if _, ok := r.byEmail[email]; ok {
		return domain.User{}, ErrConflict
	}
	r.next++
	user := domain.User{ID: "user-1", Email: email, Username: username, PasswordHash: passwordHash, Role: role, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if r.next > 1 {
		user.ID = "user-2"
	}
	r.users[user.ID] = user
	r.byEmail[email] = user.ID
	return user, nil
}

func (r *fakeRepo) GetUserByID(_ context.Context, id string) (domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return user, nil
}

func (r *fakeRepo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	id, ok := r.byEmail[email]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return r.users[id], nil
}

func (r *fakeRepo) UpdateUser(_ context.Context, id string, username string, email string, role string) (domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	user.Username = username
	user.Email = email
	if role != "" {
		user.Role = role
	}
	r.users[id] = user
	r.byEmail[email] = id
	return user, nil
}

func (r *fakeRepo) DeleteUser(_ context.Context, id string) error {
	user, ok := r.users[id]
	if !ok {
		return ErrNotFound
	}
	delete(r.byEmail, user.Email)
	delete(r.users, id)
	return nil
}

func (r *fakeRepo) GetWatchlist(_ context.Context, userID string) ([]domain.WatchlistItem, error) {
	return r.watchlist[userID], nil
}

func (r *fakeRepo) AddToWatchlist(_ context.Context, userID string, movieID string) error {
	r.watchlist[userID] = append(r.watchlist[userID], domain.WatchlistItem{MovieID: movieID, Title: movieID, AddedAt: time.Now()})
	return nil
}

func (r *fakeRepo) RemoveFromWatchlist(_ context.Context, userID string, movieID string) error {
	items := r.watchlist[userID]
	filtered := make([]domain.WatchlistItem, 0, len(items))
	for _, item := range items {
		if item.MovieID != movieID {
			filtered = append(filtered, item)
		}
	}
	r.watchlist[userID] = filtered
	return nil
}

func (r *fakeRepo) GetHistory(_ context.Context, userID string, limit int32) ([]domain.HistoryItem, error) {
	items := r.history[userID]
	if int32(len(items)) > limit {
		return items[:limit], nil
	}
	return items, nil
}

func (r *fakeRepo) AddToHistory(_ context.Context, userID string, movieID string, watchedSeconds int32) error {
	r.history[userID] = append(r.history[userID], domain.HistoryItem{MovieID: movieID, Title: movieID, WatchedSeconds: watchedSeconds, WatchedAt: time.Now()})
	return nil
}

func (r *fakeRepo) GetRecommendations(_ context.Context, userID string, limit int32) ([]string, error) {
	seen := map[string]bool{}
	ids := make([]string, 0)
	for _, item := range r.history[userID] {
		if !seen[item.MovieID] {
			seen[item.MovieID] = true
			ids = append(ids, item.MovieID)
		}
		if int32(len(ids)) == limit {
			break
		}
	}
	return ids, nil
}

type fakeTokens struct {
	saved map[string]string
}

func (t *fakeTokens) Save(_ context.Context, token string, userID string, _ time.Duration) error {
	if t.saved == nil {
		t.saved = map[string]string{}
	}
	t.saved[token] = userID
	return nil
}

func (t *fakeTokens) Delete(_ context.Context, token string) error {
	delete(t.saved, token)
	return nil
}

type fakeEvents struct {
	registered []domain.User
	deleted    []string
}

func (e *fakeEvents) PublishUserRegistered(_ context.Context, user domain.User) error {
	e.registered = append(e.registered, user)
	return nil
}

func (e *fakeEvents) PublishUserDeleted(_ context.Context, userID string) error {
	e.deleted = append(e.deleted, userID)
	return nil
}

type fakeEmail struct {
	to       string
	username string
}

func (e *fakeEmail) SendWelcome(_ context.Context, to string, username string) error {
	e.to = to
	e.username = username
	return nil
}
