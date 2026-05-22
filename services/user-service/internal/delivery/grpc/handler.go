package grpc

import (
	"context"
	"errors"

	userpb "github.com/Nalatka/GoMovieService/proto"
	"gomovieservice/services/user-service/internal/domain"
	"gomovieservice/services/user-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserUsecase interface {
	RegisterUser(ctx context.Context, email string, username string, password string) (domain.User, string, error)
	LoginUser(ctx context.Context, email string, password string) (domain.User, string, error)
	LogoutUser(ctx context.Context, token string) error
	GetUser(ctx context.Context, id string) (domain.User, error)
	UpdateUser(ctx context.Context, id string, username string, email string) (domain.User, error)
	DeleteUser(ctx context.Context, id string) error
	GetWatchlist(ctx context.Context, userID string) ([]domain.WatchlistItem, error)
	AddToWatchlist(ctx context.Context, userID string, movieID string) error
	RemoveFromWatchlist(ctx context.Context, userID string, movieID string) error
	GetHistory(ctx context.Context, userID string, limit int32) ([]domain.HistoryItem, error)
	AddToHistory(ctx context.Context, userID string, movieID string, watchedSeconds int32) error
	GetRecommendations(ctx context.Context, userID string, limit int32) ([]string, error)
}

type Handler struct {
	userpb.UnimplementedUserServiceServer
	service UserUsecase
}

func NewHandler(service UserUsecase) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterUser(ctx context.Context, req *userpb.RegisterUserRequest) (*userpb.RegisterUserResponse, error) {
	user, token, err := h.service.RegisterUser(ctx, req.GetEmail(), req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, rpcError(err)
	}
	return &userpb.RegisterUserResponse{User: toProtoUser(user), Token: token}, nil
}

func (h *Handler) LoginUser(ctx context.Context, req *userpb.LoginUserRequest) (*userpb.LoginUserResponse, error) {
	user, token, err := h.service.LoginUser(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, rpcError(err)
	}
	return &userpb.LoginUserResponse{User: toProtoUser(user), Token: token}, nil
}

func (h *Handler) LogoutUser(ctx context.Context, req *userpb.LogoutUserRequest) (*userpb.LogoutUserResponse, error) {
	if err := h.service.LogoutUser(ctx, req.GetToken()); err != nil {
		return nil, rpcError(err)
	}
	return &userpb.LogoutUserResponse{Success: true}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	user, err := h.service.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, rpcError(err)
	}
	return &userpb.GetUserResponse{User: toProtoUser(user)}, nil
}

func (h *Handler) UpdateUser(ctx context.Context, req *userpb.UpdateUserRequest) (*userpb.UpdateUserResponse, error) {
	user, err := h.service.UpdateUser(ctx, req.GetUserId(), req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, rpcError(err)
	}
	return &userpb.UpdateUserResponse{User: toProtoUser(user)}, nil
}

func (h *Handler) DeleteUser(ctx context.Context, req *userpb.DeleteUserRequest) (*userpb.DeleteUserResponse, error) {
	if err := h.service.DeleteUser(ctx, req.GetUserId()); err != nil {
		return nil, rpcError(err)
	}
	return &userpb.DeleteUserResponse{Success: true}, nil
}

func (h *Handler) GetWatchlist(ctx context.Context, req *userpb.GetWatchlistRequest) (*userpb.GetWatchlistResponse, error) {
	items, err := h.service.GetWatchlist(ctx, req.GetUserId())
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*userpb.WatchlistItem, 0, len(items))
	for _, item := range items {
		out = append(out, &userpb.WatchlistItem{MovieId: item.MovieID, Title: item.Title, AddedAt: timestamppb.New(item.AddedAt)})
	}
	return &userpb.GetWatchlistResponse{Items: out}, nil
}

func (h *Handler) AddToWatchlist(ctx context.Context, req *userpb.AddToWatchlistRequest) (*userpb.AddToWatchlistResponse, error) {
	if err := h.service.AddToWatchlist(ctx, req.GetUserId(), req.GetMovieId()); err != nil {
		return nil, rpcError(err)
	}
	return &userpb.AddToWatchlistResponse{Success: true}, nil
}

func (h *Handler) RemoveFromWatchlist(ctx context.Context, req *userpb.RemoveFromWatchlistRequest) (*userpb.RemoveFromWatchlistResponse, error) {
	if err := h.service.RemoveFromWatchlist(ctx, req.GetUserId(), req.GetMovieId()); err != nil {
		return nil, rpcError(err)
	}
	return &userpb.RemoveFromWatchlistResponse{Success: true}, nil
}

func (h *Handler) GetHistory(ctx context.Context, req *userpb.GetHistoryRequest) (*userpb.GetHistoryResponse, error) {
	items, err := h.service.GetHistory(ctx, req.GetUserId(), req.GetLimit())
	if err != nil {
		return nil, rpcError(err)
	}
	out := make([]*userpb.HistoryItem, 0, len(items))
	for _, item := range items {
		out = append(out, &userpb.HistoryItem{MovieId: item.MovieID, Title: item.Title, WatchedSeconds: item.WatchedSeconds, WatchedAt: timestamppb.New(item.WatchedAt)})
	}
	return &userpb.GetHistoryResponse{Items: out}, nil
}

func (h *Handler) AddToHistory(ctx context.Context, req *userpb.AddToHistoryRequest) (*userpb.AddToHistoryResponse, error) {
	if err := h.service.AddToHistory(ctx, req.GetUserId(), req.GetMovieId(), req.GetWatchedSeconds()); err != nil {
		return nil, rpcError(err)
	}
	return &userpb.AddToHistoryResponse{Success: true}, nil
}

func (h *Handler) GetRecommendations(ctx context.Context, req *userpb.GetRecommendationsRequest) (*userpb.GetRecommendationsResponse, error) {
	ids, err := h.service.GetRecommendations(ctx, req.GetUserId(), req.GetLimit())
	if err != nil {
		return nil, rpcError(err)
	}
	return &userpb.GetRecommendationsResponse{MovieIds: ids}, nil
}

func toProtoUser(user domain.User) *userpb.User {
	return &userpb.User{
		Id:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		CreatedAt: timestamppb.New(user.CreatedAt),
		Role:      user.Role,
	}
}

func rpcError(err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, usecase.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, usecase.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, usecase.ErrConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
