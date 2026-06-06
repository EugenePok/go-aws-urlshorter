package store

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"urlshortener/internal/cache"
)

const cacheKeyPrefix = "link:"

// Decorate a Store with read-through cache
// the rest of the app in unaware caching exists
type Cached struct {
	store Store
	cache cache.Cache
	ttl   time.Duration
	log   *slog.Logger
}

func NewCached(store Store, cache cache.Cache, ttl time.Duration, log *slog.Logger) *Cached {
	return &Cached{
		store: store,
		cache: cache,
		ttl:   ttl,
		log:   log,
	}
}

func (c *Cached) Save(ctx context.Context, link Link) error {
	return c.store.Save(ctx, link)
}

func (c *Cached) Get(ctx context.Context, code string) (Link, error) {
	key := cacheKeyPrefix + code

	switch url, err := c.cache.Get(ctx, key); {
	case err == nil:
		return Link{Code: code, LongURL: url}, nil
	case errors.Is(err, cache.ErrMiss):
	default:
		c.log.Warn("cache get failed; serving from store", "err", err, "code", code)
	}

	link, err := c.store.Get(ctx, code)
	if err != nil {
		return Link{}, err
	}
	if err := c.cache.Set(ctx, key, link.LongURL, c.ttl); err != nil {
		c.log.Warn("cache set failed", "err", err, "code", code)
	}
	return link, nil
}
