package cache

import (
	"context"
	"errors"
	"time"
)

var ErrMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}
