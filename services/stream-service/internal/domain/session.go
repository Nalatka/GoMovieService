package domain

import "time"

// Session represents a streaming session
type Session struct {
	ID              string
	UserID          string
	MovieID         string
	PositionSeconds int32
	Quality         string
	Status          string
	StartedAt       time.Time
	UpdatedAt       time.Time
}

// Subtitle represents subtitle information
type Subtitle struct {
	ID      string
	MovieID string
	Lang    string // "en", "ru", "kz"
	Label   string // "English", "Русский"
	FileURL string
}

// Status constants
const (
	StatusPlaying  = "playing"
	StatusPaused   = "paused"
	StatusFinished = "finished"
)

// Quality constants
const (
	Quality480P  = "480p"
	Quality720P  = "720p"
	Quality1080P = "1080p"
)

// NATS events and payloads
const (
	EventStreamStarted   = "stream.started"
	EventStreamCompleted = "stream.completed"
	EventUserDeleted     = "user.deleted"
)

type StreamStartedPayload struct {
	UserID  string `json:"user_id"`
	MovieID string `json:"movie_id"`
}

type StreamCompletedPayload struct {
	UserID         string `json:"user_id"`
	MovieID        string `json:"movie_id"`
	WatchedSeconds int32  `json:"watched_seconds"`
}

type UserDeletedPayload struct {
	UserID string `json:"user_id"`
}
