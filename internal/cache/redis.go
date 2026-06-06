package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(addr string) *Redis {
	return &Redis{client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrMiss
	}
	if err != nil {
		return "", fmt.Errorf("redis get: %w", err)
	}
	return val, nil
}

func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := r.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.client.Close()
}
