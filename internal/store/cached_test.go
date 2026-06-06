package store

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
	"urlshortener/internal/cache"
)

type fakeCache struct {
	data map[string]string
	fail bool
	gets int
	sets int
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: map[string]string{}}
}

func (f *fakeCache) Get(_ context.Context, key string) (string, error) {
	f.gets++
	if f.fail {
		return "", errors.New("cache down")
	}
	v, ok := f.data[key]
	if !ok {
		return "", cache.ErrMiss
	}
	return v, nil
}

func (f *fakeCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	f.sets++
	if f.fail {
		return errors.New("cache down")
	}
	f.data[key] = value
	return nil
}

// countingStore counts Get calls so we can prove a cache hit skips the store.
type countingStore struct {
	Store
	gets int
}

func (c *countingStore) Get(ctx context.Context, code string) (Link, error) {
	c.gets++
	return c.Store.Get(ctx, code)
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func seededMemory(t *testing.T) *Memory {
	t.Helper()
	m := NewMemory()
	if err := m.Save(context.Background(),
		Link{Code: "abc", LongURL: "http://sample.com?q=1", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCached_MissPopulateThenHitSkipsStore(t *testing.T) {
	cs := &countingStore{Store: seededMemory(t)}
	fc := newFakeCache()
	c := NewCached(cs, fc, time.Minute, discardLogger())
	ctx := context.Background()

	got, err := c.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if got.LongURL != "http://sample.com?q=1" {
		t.Errorf("LongURL = %q", got.LongURL)
	}

	if fc.gets != 1 {
		t.Errorf("cache gets = %d want 1", fc.gets)
	}
	if cs.gets != 1 {
		t.Errorf("store gets = %d, want 1", cs.gets)
	}
	if fc.sets != 1 {
		t.Errorf("cache sets = %d want 1", fc.sets)
	}

	if _, err := c.Get(ctx, "abc"); err != nil {
		t.Fatalf("second get : %v", err)
	}
	if cs.gets != 1 {
		t.Errorf("store gets = %d want 1 to read from cache", cs.gets)
	}
	if fc.gets != 2 {
		t.Errorf("cache gets = %d want 2", fc.gets)
	}
}

func TestCached_DegradesWhenCacheDown(t *testing.T) {
	fc := newFakeCache()
	fc.fail = true
	c := NewCached(seededMemory(t), fc, time.Minute, discardLogger())

	got, err := c.Get(context.Background(), "abc")
	if err != nil {
		t.Fatalf("want graceful degradation, got err : %v", err)
	}
	if got.LongURL != "http://sample.com?q=1" {
		t.Errorf("LongURL = %q", got.LongURL)
	}
}

func TestCached_NotFoundPropagatesAndIsNotCached(t *testing.T) {
	fc := newFakeCache()
	c := NewCached(NewMemory(), fc, time.Minute, discardLogger())

	if _, err := c.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if fc.sets != 0 {
		t.Errorf("cache sets = %d, want 0 (misses are not cached)", fc.sets)
	}
}

func TestCached_SaveDoesNotPopulateCache(t *testing.T) {
	fc := newFakeCache()
	c := NewCached(NewMemory(), fc, time.Minute, discardLogger())

	if err := c.Save(context.Background(), Link{Code: "abc", LongURL: "https://x.com"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if fc.sets != 0 {
		t.Errorf("cache sets = %d, want 0 (write path doesn't cache)", fc.sets)
	}
}
