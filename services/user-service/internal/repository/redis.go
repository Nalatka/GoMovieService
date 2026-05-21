package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTokenStore struct {
	client *redis.Client
}

func NewRedisTokenStore(client *redis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

func (s *RedisTokenStore) Save(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, "token:"+token, userID, ttl).Err()
}

func (s *RedisTokenStore) Delete(ctx context.Context, token string) error {
	return s.client.Del(ctx, "token:"+token).Err()
}
