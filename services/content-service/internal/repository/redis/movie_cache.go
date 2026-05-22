package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gomovieservice/services/content-service/internal/domain"
)

const (
	movieKeyPrefix = "movie:"
	topMoviesKey   = "top_movies"
	movieTTL       = 10 * time.Minute
	topTTL         = 5 * time.Minute
)

type movieCache struct {
	client *redis.Client
	ctx    context.Context
}

func NewMovieCache(client *redis.Client) domain.MovieCache {
	return &movieCache{client: client, ctx: context.Background()}
}

func (c *movieCache) GetMovie(id uuid.UUID) (*domain.Movie, error) {
	key := fmt.Sprintf("%s%s", movieKeyPrefix, id.String()) // %d changed to %s for uuid string formatting
	data, err := c.client.Get(c.ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var movie domain.Movie
	if err := json.Unmarshal(data, &movie); err != nil {
		return nil, err
	}
	return &movie, nil
}

func (c *movieCache) SetMovie(movie *domain.Movie) error {
	key := fmt.Sprintf("%s%s", movieKeyPrefix, movie.ID.String()) // %d changed to %s
	data, err := json.Marshal(movie)
	if err != nil {
		return err
	}
	return c.client.Set(c.ctx, key, data, movieTTL).Err()
}

func (c *movieCache) DeleteMovie(id uuid.UUID) error {
	key := fmt.Sprintf("%s%s", movieKeyPrefix, id.String()) // %d changed to %s
	return c.client.Del(c.ctx, key).Err()
}

func (c *movieCache) GetTopMovies() ([]*domain.Movie, error) {
	data, err := c.client.Get(c.ctx, topMoviesKey).Bytes()
	if err != nil {
		return nil, err
	}
	var movies []*domain.Movie
	if err := json.Unmarshal(data, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func (c *movieCache) SetTopMovies(movies []*domain.Movie) error {
	data, err := json.Marshal(movies)
	if err != nil {
		return err
	}
	return c.client.Set(c.ctx, topMoviesKey, data, topTTL).Err()
}

func (c *movieCache) InvalidateTop() error {
	return c.client.Del(c.ctx, topMoviesKey).Err()
}
