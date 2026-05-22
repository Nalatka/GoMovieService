package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"gomovieservice/services/content-service/internal/domain"
	"gomovieservice/services/content-service/internal/usecase"

	pb "github.com/Nalatka/GoMovieService/proto/content"
)

type ContentHandler struct {
	pb.UnimplementedContentServiceServer
	uc *usecase.ContentUsecase
}

func NewContentHandler(uc *usecase.ContentUsecase) *ContentHandler {
	return &ContentHandler{uc: uc}
}

func (h *ContentHandler) CreateMovie(ctx context.Context, req *pb.CreateMovieRequest) (*pb.CreateMovieResponse, error) {
	genreUUID, err := uuid.Parse(req.GenreId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid genre_id UUID format")
	}

	movie := &domain.Movie{
		Title:       req.Title,
		Description: req.Description,
		Year:        req.Year,
		GenreID:     genreUUID,
		VideoURL:    req.VideoUrl,
		PosterURL:   req.PosterUrl,
		DurationSec: req.DurationSec,
	}
	created, err := h.uc.CreateMovie(movie)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.CreateMovieResponse{Movie: domainToProto(created)}, nil
}

func (h *ContentHandler) GetMovie(ctx context.Context, req *pb.GetMovieRequest) (*pb.GetMovieResponse, error) {
	movieUUID, err := uuid.Parse(req.MovieId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid movie_id UUID format")
	}

	movie, err := h.uc.GetMovie(movieUUID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetMovieResponse{Movie: domainToProto(movie)}, nil
}

func (h *ContentHandler) UpdateMovie(ctx context.Context, req *pb.UpdateMovieRequest) (*pb.UpdateMovieResponse, error) {
	movieUUID, err := uuid.Parse(req.MovieId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid movie_id UUID format")
	}
	genreUUID, err := uuid.Parse(req.GenreId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid genre_id UUID format")
	}

	movie := &domain.Movie{
		ID:          movieUUID,
		Title:       req.Title,
		Description: req.Description,
		Year:        req.Year,
		GenreID:     genreUUID,
		VideoURL:    req.VideoUrl,
		PosterURL:   req.PosterUrl,
		DurationSec: req.DurationSec,
	}
	updated, err := h.uc.UpdateMovie(movie)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateMovieResponse{Movie: domainToProto(updated)}, nil
}

func (h *ContentHandler) DeleteMovie(ctx context.Context, req *pb.DeleteMovieRequest) (*pb.DeleteMovieResponse, error) {
	movieUUID, err := uuid.Parse(req.MovieId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid movie_id UUID format")
	}

	if err := h.uc.DeleteMovie(movieUUID); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.DeleteMovieResponse{Success: true}, nil
}

func (h *ContentHandler) ListMovies(ctx context.Context, req *pb.ListMoviesRequest) (*pb.ListMoviesResponse, error) {
	movies, total, err := h.uc.ListMovies(req.Page, req.Limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ListMoviesResponse{Movies: domainSliceToProto(movies), Total: total}, nil
}

func (h *ContentHandler) SearchMovies(ctx context.Context, req *pb.SearchMoviesRequest) (*pb.SearchMoviesResponse, error) {
	movies, total, err := h.uc.SearchMovies(req.Query, req.Page, req.Limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.SearchMoviesResponse{Movies: domainSliceToProto(movies), Total: total}, nil
}

func (h *ContentHandler) GetGenres(ctx context.Context, req *pb.GetGenresRequest) (*pb.GetGenresResponse, error) {
	genres, err := h.uc.GetGenres()
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetGenresResponse{Genres: genreSliceToProto(genres)}, nil
}

func (h *ContentHandler) GetMoviesByGenre(ctx context.Context, req *pb.GetMoviesByGenreRequest) (*pb.GetMoviesByGenreResponse, error) {
	genreUUID, err := uuid.Parse(req.GenreId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid genre_id UUID format")
	}

	movies, total, err := h.uc.GetMoviesByGenre(genreUUID, req.Page, req.Limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetMoviesByGenreResponse{Movies: domainSliceToProto(movies), Total: total}, nil
}

func (h *ContentHandler) RateMovie(ctx context.Context, req *pb.RateMovieRequest) (*pb.RateMovieResponse, error) {
	movieUUID, err := uuid.Parse(req.MovieId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid movie_id UUID format")
	}
	userUUID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id UUID format")
	}

	newAvg, total, err := h.uc.RateMovie(movieUUID, userUUID, req.Score)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.RateMovieResponse{Success: true, NewAvg: newAvg, TotalVotes: total}, nil
}

func (h *ContentHandler) GetMovieRating(ctx context.Context, req *pb.GetMovieRatingRequest) (*pb.GetMovieRatingResponse, error) {
	movieUUID, err := uuid.Parse(req.MovieId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid movie_id UUID format")
	}

	avg, total, err := h.uc.GetMovieRating(movieUUID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetMovieRatingResponse{AvgRating: avg, TotalVotes: total}, nil
}

func (h *ContentHandler) GetTopMovies(ctx context.Context, req *pb.GetTopMoviesRequest) (*pb.GetTopMoviesResponse, error) {
	movies, err := h.uc.GetTopMovies(req.Limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetTopMoviesResponse{Movies: domainSliceToProto(movies)}, nil
}

func (h *ContentHandler) GetSimilarMovies(ctx context.Context, req *pb.GetSimilarMoviesRequest) (*pb.GetSimilarMoviesResponse, error) {
	movieUUID, err := uuid.Parse(req.MovieId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid movie_id UUID format")
	}

	movies, err := h.uc.GetSimilarMovies(movieUUID, req.Limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetSimilarMoviesResponse{Movies: domainSliceToProto(movies)}, nil
}

// --- Mappers & Helpers ---

func domainToProto(m *domain.Movie) *pb.Movie {
	var ts *timestamppb.Timestamp
	if !m.CreatedAt.IsZero() {
		ts = timestamppb.New(m.CreatedAt)
	} else {
		ts = timestamppb.New(time.Now())
	}
	return &pb.Movie{
		Id:          m.ID.String(),
		Title:       m.Title,
		Description: m.Description,
		Year:        m.Year,
		GenreId:     m.GenreID.String(),
		GenreName:   m.GenreName,
		VideoUrl:    m.VideoURL,
		PosterUrl:   m.PosterURL,
		DurationSec: m.DurationSec,
		AvgRating:   m.AvgRating,
		Views:       m.Views,
		CreatedAt:   ts,
	}
}

func domainSliceToProto(movies []*domain.Movie) []*pb.Movie {
	result := make([]*pb.Movie, len(movies))
	for i, m := range movies {
		result[i] = domainToProto(m)
	}
	return nil // compiler error fix: return result
}

func genreSliceToProto(genres []*domain.Genre) []*pb.Genre {
	result := make([]*pb.Genre, len(genres))
	for i, g := range genres {
		result[i] = &pb.Genre{
			Id:   g.ID.String(),
			Name: g.Name,
		}
	}
	return result
}

func toGRPCError(err error) error {
	switch err {
	case usecase.ErrMovieNotFound:
		return status.Error(codes.NotFound, err.Error())
	case usecase.ErrGenreNotFound:
		return status.Error(codes.NotFound, err.Error())
	case usecase.ErrInvalidScore:
		return status.Error(codes.InvalidArgument, err.Error())
	case usecase.ErrInvalidPage, usecase.ErrInvalidLimit, usecase.ErrTitleRequired:
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
