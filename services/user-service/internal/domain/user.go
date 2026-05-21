package domain

import "time"

type User struct {
	ID           string
	Email        string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type WatchlistItem struct {
	MovieID string
	Title   string
	AddedAt time.Time
}

type HistoryItem struct {
	MovieID        string
	Title          string
	WatchedSeconds int32
	WatchedAt      time.Time
}
