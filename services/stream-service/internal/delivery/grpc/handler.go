package grpc

import (
	"context"

	streampb "github.com/Nalatka/GoMovieService/proto/stream"
	"gomovieservice/services/stream-service/internal/domain"
	"gomovieservice/services/stream-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StreamUsecase interface {
	StartStream(ctx context.Context, userID, movieID string, quality string) (domain.Session, error)
	StopStream(ctx context.Context, sessionID string) (int32, error)
	GetStreamStatus(ctx context.Context, sessionID string) (domain.Session, error)
	PauseStream(ctx context.Context, sessionID string, position int32) error
	ResumeStream(ctx context.Context, sessionID string) error
	SeekStream(ctx context.Context, sessionID string, position int32) error
	GetQualities(ctx context.Context, movieID string) ([]string, error)
	SetQuality(ctx context.Context, sessionID string, quality string) error
	GetSubtitles(ctx context.Context, movieID string) ([]domain.Subtitle, error)
	GetSubtitlesByLang(ctx context.Context, movieID, lang string) (domain.Subtitle, error)
	GetActiveSessions(ctx context.Context, limit int) ([]domain.Session, error)
	GetPreview(ctx context.Context, movieID string) (string, string, error)
	FinishUserSessions(ctx context.Context, userID string) error
}

type Handler struct {
	streampb.UnimplementedStreamServiceServer
	service StreamUsecase
}

func NewHandler(service StreamUsecase) *Handler {
	return &Handler{service: service}
}

// StartStream implements StartStream RPC
func (h *Handler) StartStream(ctx context.Context, req *streampb.StartStreamRequest) (*streampb.StartStreamResponse, error) {
	quality := domain.Quality720P
	switch req.Quality {
	case streampb.Quality_Q_480P:
		quality = domain.Quality480P
	case streampb.Quality_Q_1080P:
		quality = domain.Quality1080P
	}

	session, err := h.service.StartStream(ctx, req.UserId, req.MovieId, quality)
	if err != nil {
		return nil, rpcError(err)
	}

	return &streampb.StartStreamResponse{Session: sessionToPb(session)}, nil
}

// StopStream implements StopStream RPC
func (h *Handler) StopStream(ctx context.Context, req *streampb.StopStreamRequest) (*streampb.StopStreamResponse, error) {
	watchedSeconds, err := h.service.StopStream(ctx, req.SessionId)
	if err != nil {
		return nil, rpcError(err)
	}

	return &streampb.StopStreamResponse{
		Success:        true,
		WatchedSeconds: watchedSeconds,
	}, nil
}

// GetStreamStatus implements GetStreamStatus RPC
func (h *Handler) GetStreamStatus(ctx context.Context, req *streampb.GetStreamStatusRequest) (*streampb.GetStreamStatusResponse, error) {
	session, err := h.service.GetStreamStatus(ctx, req.SessionId)
	if err != nil {
		return nil, rpcError(err)
	}

	return &streampb.GetStreamStatusResponse{Session: sessionToPb(session)}, nil
}

// PauseStream implements PauseStream RPC
func (h *Handler) PauseStream(ctx context.Context, req *streampb.PauseStreamRequest) (*streampb.PauseStreamResponse, error) {
	if err := h.service.PauseStream(ctx, req.SessionId, req.PositionSeconds); err != nil {
		return nil, rpcError(err)
	}

	return &streampb.PauseStreamResponse{Success: true}, nil
}

// ResumeStream implements ResumeStream RPC
func (h *Handler) ResumeStream(ctx context.Context, req *streampb.ResumeStreamRequest) (*streampb.ResumeStreamResponse, error) {
	if err := h.service.ResumeStream(ctx, req.SessionId); err != nil {
		return nil, rpcError(err)
	}

	return &streampb.ResumeStreamResponse{Success: true}, nil
}

// SeekStream implements SeekStream RPC
func (h *Handler) SeekStream(ctx context.Context, req *streampb.SeekStreamRequest) (*streampb.SeekStreamResponse, error) {
	if err := h.service.SeekStream(ctx, req.SessionId, req.PositionSeconds); err != nil {
		return nil, rpcError(err)
	}

	return &streampb.SeekStreamResponse{Success: true}, nil
}

// GetQualities implements GetQualities RPC
func (h *Handler) GetQualities(ctx context.Context, req *streampb.GetQualitiesRequest) (*streampb.GetQualitiesResponse, error) {
	qualities, err := h.service.GetQualities(ctx, req.MovieId)
	if err != nil {
		return nil, rpcError(err)
	}

	pbQualities := make([]streampb.Quality, 0, len(qualities))
	for _, q := range qualities {
		switch q {
		case domain.Quality480P:
			pbQualities = append(pbQualities, streampb.Quality_Q_480P)
		case domain.Quality720P:
			pbQualities = append(pbQualities, streampb.Quality_Q_720P)
		case domain.Quality1080P:
			pbQualities = append(pbQualities, streampb.Quality_Q_1080P)
		}
	}

	return &streampb.GetQualitiesResponse{Qualities: pbQualities}, nil
}

// SetQuality implements SetQuality RPC
func (h *Handler) SetQuality(ctx context.Context, req *streampb.SetQualityRequest) (*streampb.SetQualityResponse, error) {
	quality := domain.Quality720P
	switch req.Quality {
	case streampb.Quality_Q_480P:
		quality = domain.Quality480P
	case streampb.Quality_Q_1080P:
		quality = domain.Quality1080P
	}

	if err := h.service.SetQuality(ctx, req.SessionId, quality); err != nil {
		return nil, rpcError(err)
	}

	return &streampb.SetQualityResponse{Success: true}, nil
}

// GetSubtitles implements GetSubtitles RPC
func (h *Handler) GetSubtitles(ctx context.Context, req *streampb.GetSubtitlesRequest) (*streampb.GetSubtitlesResponse, error) {
	subtitles, err := h.service.GetSubtitles(ctx, req.MovieId)
	if err != nil {
		return nil, rpcError(err)
	}

	pbSubtitles := make([]*streampb.Subtitle, 0, len(subtitles))
	for _, s := range subtitles {
		pbSubtitles = append(pbSubtitles, &streampb.Subtitle{
			Lang:    s.Lang,
			Label:   s.Label,
			FileUrl: s.FileURL,
		})
	}

	return &streampb.GetSubtitlesResponse{Subtitles: pbSubtitles}, nil
}

// GetSubtitlesByLang implements GetSubtitlesByLang RPC
func (h *Handler) GetSubtitlesByLang(ctx context.Context, req *streampb.GetSubtitlesByLangRequest) (*streampb.GetSubtitlesByLangResponse, error) {
	subtitle, err := h.service.GetSubtitlesByLang(ctx, req.MovieId, req.Lang)
	if err != nil {
		return nil, rpcError(err)
	}

	return &streampb.GetSubtitlesByLangResponse{
		Subtitle: &streampb.Subtitle{
			Lang:    subtitle.Lang,
			Label:   subtitle.Label,
			FileUrl: subtitle.FileURL,
		},
	}, nil
}

// GetActiveSessions implements GetActiveSessions RPC
func (h *Handler) GetActiveSessions(ctx context.Context, req *streampb.GetActiveSessionsRequest) (*streampb.GetActiveSessionsResponse, error) {
	sessions, err := h.service.GetActiveSessions(ctx, int(req.Limit))
	if err != nil {
		return nil, rpcError(err)
	}

	pbSessions := make([]*streampb.Session, 0, len(sessions))
	for _, s := range sessions {
		pbSessions = append(pbSessions, sessionToPb(s))
	}

	return &streampb.GetActiveSessionsResponse{
		Sessions: pbSessions,
		Total:    int32(len(pbSessions)),
	}, nil
}

// GetPreview implements GetPreview RPC
func (h *Handler) GetPreview(ctx context.Context, req *streampb.GetPreviewRequest) (*streampb.GetPreviewResponse, error) {
	previewURL, trailerURL, err := h.service.GetPreview(ctx, req.MovieId)
	if err != nil {
		return nil, rpcError(err)
	}

	return &streampb.GetPreviewResponse{
		PreviewUrl: previewURL,
		TrailerUrl: trailerURL,
	}, nil
}

// Helper functions

func sessionToPb(session domain.Session) *streampb.Session {
	status := streampb.SessionStatus_UNKNOWN
	switch session.Status {
	case domain.StatusPlaying:
		status = streampb.SessionStatus_PLAYING
	case domain.StatusPaused:
		status = streampb.SessionStatus_PAUSED
	case domain.StatusFinished:
		status = streampb.SessionStatus_FINISHED
	}

	quality := streampb.Quality_Q_720P
	switch session.Quality {
	case domain.Quality480P:
		quality = streampb.Quality_Q_480P
	case domain.Quality1080P:
		quality = streampb.Quality_Q_1080P
	}

	return &streampb.Session{
		Id:              session.ID,
		UserId:          session.UserID,
		MovieId:         session.MovieID,
		PositionSeconds: session.PositionSeconds,
		Quality:         quality,
		Status:          status,
		StartedAt:       timestamppb.New(session.StartedAt),
		UpdatedAt:       timestamppb.New(session.UpdatedAt),
	}
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}

	switch err {
	case usecase.ErrNotFound:
		return status.Error(codes.NotFound, "not found")
	case usecase.ErrInvalidInput:
		return status.Error(codes.InvalidArgument, "invalid input")
	case usecase.ErrInvalidStatus:
		return status.Error(codes.InvalidArgument, "invalid status")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
