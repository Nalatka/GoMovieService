package repository

import (
	"context"
	"encoding/json"

	"github.com/Nalatka/GoMovieService/services/user-service/internal/domain"
	"github.com/Nalatka/GoMovieService/services/user-service/internal/usecase"
	"github.com/nats-io/nats.go"
)

type NATSEvents struct {
	conn *nats.Conn
}

func NewNATSEvents(conn *nats.Conn) *NATSEvents {
	return &NATSEvents{conn: conn}
}

func (e *NATSEvents) PublishUserRegistered(ctx context.Context, user domain.User) error {
	payload := map[string]string{
		"user_id":  user.ID,
		"email":    user.Email,
		"username": user.Username,
	}
	return e.publish(ctx, "user.registered", payload)
}

func (e *NATSEvents) PublishUserDeleted(ctx context.Context, userID string) error {
	payload := map[string]string{"user_id": userID}
	return e.publish(ctx, "user.deleted", payload)
}

func (e *NATSEvents) SubscribeStreamCompleted(service *usecase.Service) (*nats.Subscription, error) {
	return e.conn.Subscribe("stream.completed", func(msg *nats.Msg) {
		var payload struct {
			UserID         string `json:"user_id"`
			MovieID        string `json:"movie_id"`
			WatchedSeconds int32  `json:"watched_seconds"`
		}
		if json.Unmarshal(msg.Data, &payload) != nil {
			return
		}
		_ = service.AddToHistory(context.Background(), payload.UserID, payload.MovieID, payload.WatchedSeconds)
	})
}

func (e *NATSEvents) publish(ctx context.Context, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- e.conn.Publish(subject, data)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
		e.conn.Flush()
		return e.conn.LastError()
	}
}
