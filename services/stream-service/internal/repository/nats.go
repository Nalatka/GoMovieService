package repository

import (
	"context"
	"encoding/json"
	"log"

	"github.com/Nalatka/GoMovieService/services/stream-service/internal/domain"
	"github.com/nats-io/nats.go"
)

type NATSEvents struct {
	conn *nats.Conn
}

func NewNATSEvents(conn *nats.Conn) *NATSEvents {
	return &NATSEvents{conn: conn}
}

// PublishStreamStarted publishes stream.started event
func (e *NATSEvents) PublishStreamStarted(ctx context.Context, userID, movieID string) error {
	payload := domain.StreamStartedPayload{
		UserID:  userID,
		MovieID: movieID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return e.conn.Publish(domain.EventStreamStarted, data)
}

// PublishStreamCompleted publishes stream.completed event
func (e *NATSEvents) PublishStreamCompleted(ctx context.Context, userID, movieID string, watchedSeconds int32) error {
	payload := domain.StreamCompletedPayload{
		UserID:         userID,
		MovieID:        movieID,
		WatchedSeconds: watchedSeconds,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return e.conn.Publish(domain.EventStreamCompleted, data)
}

// SubscribeUserDeleted subscribes to user.deleted event
type UserDeletedHandler func(ctx context.Context, userID string) error

func (e *NATSEvents) SubscribeUserDeleted(ctx context.Context, handler UserDeletedHandler) error {
	_, err := e.conn.Subscribe(domain.EventUserDeleted, func(msg *nats.Msg) {
		var payload domain.UserDeletedPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Printf("failed to unmarshal user.deleted payload: %v", err)
			return
		}

		if err := handler(ctx, payload.UserID); err != nil {
			log.Printf("failed to handle user.deleted event: %v", err)
		}
	})

	return err
}
