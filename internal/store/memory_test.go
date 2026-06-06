package store

import (
	"context"
	"errors"
	"testing"
)

func TestMemory_SaveAndGet(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	link := Link{Code: "abc", LongURL: "https://example.com"}
	err := m.Save(ctx, link)
	if err != nil {
		t.Fatalf("Save %v", err)
	}
	got, err := m.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get %v", err)
	}
	if got.LongURL != link.LongURL {
		t.Errorf("got Long URL = %q, want = %q", got.LongURL, link.LongURL)
	}
}

func TestMemory_Get_NotFound(t *testing.T) {
	m := NewMemory()
	if _, err := m.Get(context.Background(), "notfound"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Got %v, want ErrNotFound", err)
	}
}

func TestMemory_Save_Duplicate(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	if err := m.Save(ctx, Link{Code: "abc"}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := m.Save(ctx, Link{Code: "abc"}); !errors.Is(err, ErrCodeExists) {
		t.Fatalf("Got %v, want ErrCodeExists", err)
	}
}
