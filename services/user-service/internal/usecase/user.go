package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gomovieservice/services/user-service/internal/domain"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
)

type UserRepository interface {
	CreateUser(ctx context.Context, email string, username string, passwordHash string, role string) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	UpdateUser(ctx context.Context, id string, username string, email string) (domain.User, error)
	DeleteUser(ctx context.Context, id string) error
	GetWatchlist(ctx context.Context, userID string) ([]domain.WatchlistItem, error)
	AddToWatchlist(ctx context.Context, userID string, movieID string) error
	RemoveFromWatchlist(ctx context.Context, userID string, movieID string) error
	GetHistory(ctx context.Context, userID string, limit int32) ([]domain.HistoryItem, error)
	AddToHistory(ctx context.Context, userID string, movieID string, watchedSeconds int32) error
	GetRecommendations(ctx context.Context, userID string, limit int32) ([]string, error)
}

type TokenStore interface {
	Save(ctx context.Context, token string, userID string, ttl time.Duration) error
	Delete(ctx context.Context, token string) error
}

type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, user domain.User) error
	PublishUserDeleted(ctx context.Context, userID string) error
}

type EmailSender interface {
	SendWelcome(ctx context.Context, to string, username string) error
}

type Service struct {
	repo        UserRepository
	tokens      TokenStore
	events      EventPublisher
	email       EmailSender
	jwtSecret   []byte
	tokenTTL    time.Duration
	bcryptCost  int
	adminEmails map[string]bool
}

func NewService(repo UserRepository, tokens TokenStore, events EventPublisher, email EmailSender, jwtSecret string) *Service {
	if jwtSecret == "" {
		jwtSecret = "dev-secret"
	}
	return &Service{
		repo:        repo,
		tokens:      tokens,
		events:      events,
		email:       email,
		jwtSecret:   []byte(jwtSecret),
		tokenTTL:    24 * time.Hour,
		bcryptCost:  bcrypt.DefaultCost,
		adminEmails: map[string]bool{},
	}
}

func (s *Service) SetAdminEmails(emails string) {
	s.adminEmails = parseAdminEmails(emails)
}

func (s *Service) RegisterUser(ctx context.Context, email string, username string, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)
	if email == "" || username == "" || len(password) < 6 {
		return domain.User{}, "", ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return domain.User{}, "", err
	}
	role := "user"
	if s.adminEmails[email] {
		role = "admin"
	}
	user, err := s.repo.CreateUser(ctx, email, username, string(hash), role)
	if err != nil {
		return domain.User{}, "", err
	}
	token, err := s.issueToken(ctx, user)
	if err != nil {
		return domain.User{}, "", err
	}
	if s.events != nil {
		if err := s.events.PublishUserRegistered(ctx, user); err != nil {
			return domain.User{}, "", err
		}
	}
	if s.email != nil {
		if err := s.email.SendWelcome(ctx, user.Email, user.Username); err != nil {
			return domain.User{}, "", err
		}
	}
	return user, token, nil
}

func (s *Service) LoginUser(ctx context.Context, email string, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", ErrInvalidInput
	}
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.User{}, "", ErrInvalidCredentials
		}
		return domain.User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return domain.User{}, "", ErrInvalidCredentials
	}
	token, err := s.issueToken(ctx, user)
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

func (s *Service) LogoutUser(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrInvalidInput
	}
	return s.tokens.Delete(ctx, token)
}

func (s *Service) GetUser(ctx context.Context, id string) (domain.User, error) {
	if strings.TrimSpace(id) == "" {
		return domain.User{}, ErrInvalidInput
	}
	return s.repo.GetUserByID(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, id string, username string, email string) (domain.User, error) {
	id = strings.TrimSpace(id)
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)
	if id == "" || email == "" || username == "" {
		return domain.User{}, ErrInvalidInput
	}
	return s.repo.UpdateUser(ctx, id, username, email)
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidInput
	}
	if err := s.repo.DeleteUser(ctx, id); err != nil {
		return err
	}
	if s.events != nil {
		return s.events.PublishUserDeleted(ctx, id)
	}
	return nil
}

func (s *Service) GetWatchlist(ctx context.Context, userID string) ([]domain.WatchlistItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.GetWatchlist(ctx, userID)
}

func (s *Service) AddToWatchlist(ctx context.Context, userID string, movieID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(movieID) == "" {
		return ErrInvalidInput
	}
	return s.repo.AddToWatchlist(ctx, userID, movieID)
}

func (s *Service) RemoveFromWatchlist(ctx context.Context, userID string, movieID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(movieID) == "" {
		return ErrInvalidInput
	}
	return s.repo.RemoveFromWatchlist(ctx, userID, movieID)
}

func (s *Service) GetHistory(ctx context.Context, userID string, limit int32) ([]domain.HistoryItem, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.GetHistory(ctx, userID, normalizeLimit(limit))
}

func (s *Service) AddToHistory(ctx context.Context, userID string, movieID string, watchedSeconds int32) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(movieID) == "" || watchedSeconds < 0 {
		return ErrInvalidInput
	}
	return s.repo.AddToHistory(ctx, userID, movieID, watchedSeconds)
}

func (s *Service) GetRecommendations(ctx context.Context, userID string, limit int32) ([]string, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.GetRecommendations(ctx, userID, normalizeLimit(limit))
}

func (s *Service) issueToken(ctx context.Context, user domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(s.tokenTTL).Unix(),
		"iat":  time.Now().Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}
	if err := s.tokens.Save(ctx, token, user.ID, s.tokenTTL); err != nil {
		return "", err
	}
	return token, nil
}

func parseAdminEmails(emails string) map[string]bool {
	out := map[string]bool{}
	for _, email := range strings.Split(emails, ",") {
		email = strings.TrimSpace(strings.ToLower(email))
		if email != "" {
			out[email] = true
		}
	}
	return out
}

func normalizeLimit(limit int32) int32 {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func ConflictError(err error) error {
	return fmt.Errorf("%w: %v", ErrConflict, err)
}
