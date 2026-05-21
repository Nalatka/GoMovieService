package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Nalatka/GoMovieService/services/stream-service/internal/domain"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (r *RedisCache) GetSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var session domain.Session
	if err := json.Unmarshal([]byte(val), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *RedisCache) SetSession(ctx context.Context, sessionID string, session *domain.Session, ttlSeconds int) error {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	ttl := time.Duration(ttlSeconds) * time.Second
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisCache) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return r.client.Del(ctx, key).Err()
}
