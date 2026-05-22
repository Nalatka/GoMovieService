package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"gomovieservice/services/content-service/internal/usecase"
)

type MovieRatedEvent struct {
	MovieID string  `json:"movie_id"`
	UserID  string  `json:"user_id"`
	Score   int32   `json:"score"`
	NewAvg  float64 `json:"new_avg"`
}

type UserDeletedEvent struct {
	UserID string `json:"user_id"`
}

type StreamStartedEvent struct {
	UserID  string `json:"user_id"`
	MovieID string `json:"movie_id"`
}

type Publisher struct {
	js jetstream.JetStream
}

func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{js: js}
}

func (p *Publisher) PublishMovieRated(movieID, userID string, score int32, newAvg float64) error {
	event := MovieRatedEvent{
		MovieID: movieID,
		UserID:  userID,
		Score:   score,
		NewAvg:  newAvg,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal movie.rated: %w", err)
	}

	_, err = p.js.Publish(context.Background(), "movie.rated", data)
	return err
}

type Subscriber struct {
	js jetstream.JetStream
	uc *usecase.ContentUsecase
}

func NewSubscriber(js jetstream.JetStream, uc *usecase.ContentUsecase) *Subscriber {
	return &Subscriber{js: js, uc: uc}
}

func (s *Subscriber) Subscribe() error {
	ctx := context.Background()

	_, err := s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "CONTENT_EVENTS",
		Subjects: []string{"user.deleted", "stream.started"},
	})
	if err != nil {
		return fmt.Errorf("create stream: %w", err)
	}

	// user.deleted → delete ratings
	cons1, err := s.js.CreateOrUpdateConsumer(ctx, "CONTENT_EVENTS", jetstream.ConsumerConfig{
		Durable:       "content-user-deleted",
		FilterSubject: "user.deleted",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return err
	}
	_, err = cons1.Consume(func(msg jetstream.Msg) {
		var event UserDeletedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Printf("user.deleted unmarshal error: %v", err)
			_ = msg.Nak()
			return
		}

		userUUID, err := uuid.Parse(event.UserID)
		if err != nil {
			log.Printf("invalid user_id UUID from event: %v", err)
			_ = msg.Ack() // Ack to drop poison message
			return
		}

		if err := s.uc.DeleteUserRatings(userUUID); err != nil {
			log.Printf("DeleteUserRatings error: %v", err)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return err
	}

	// stream.started → increment views
	cons2, err := s.js.CreateOrUpdateConsumer(ctx, "CONTENT_EVENTS", jetstream.ConsumerConfig{
		Durable:       "content-stream-started",
		FilterSubject: "stream.started",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return err
	}
	_, err = cons2.Consume(func(msg jetstream.Msg) {
		var event StreamStartedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Printf("stream.started unmarshal error: %v", err)
			_ = msg.Nak()
			return
		}

		movieUUID, err := uuid.Parse(event.MovieID)
		if err != nil {
			log.Printf("invalid movie_id UUID from event: %v", err)
			_ = msg.Ack() // Ack to drop poison message
			return
		}

		if err := s.uc.IncrementViews(movieUUID); err != nil {
			log.Printf("IncrementViews error: %v", err)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	return err
}
