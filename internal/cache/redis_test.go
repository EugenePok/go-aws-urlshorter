//go:build integration

package cache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func newTestRedis(t *testing.T) *Redis {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	r := NewRedis(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Ping(ctx); err != nil {
		t.Fatalf("redis ping: %v (is redis up?)", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestRedis_SetThenGet(t *testing.T) {
	r := newTestRedis(t)
	ctx := context.Background()
	key := "test:k1"
	value := "hello"
	if err := r.Set(ctx, key, value, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := r.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != value {
		t.Errorf("got %q, want %q", got, value)
	}
}

func TestRedis_Miss(t *testing.T) {
	r := newTestRedis(t)
	if _, err := r.Get(context.Background(), "miss"); !errors.Is(err, ErrMiss) {
		t.Errorf("err = %v, want ErrMiss", err)
	}
}

func TestRedis_TTLExpires(t *testing.T) {
	r := newTestRedis(t)
	ctx := context.Background()
	if err := r.Set(ctx, "test:ttl", "v", 100*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := r.Get(ctx, "test:ttl"); !errors.Is(err, ErrMiss) {
		t.Errorf("after TTL err = %v, want ErrMiss", err)
	}
}
