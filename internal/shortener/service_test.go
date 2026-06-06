package shortener

import (
	"context"
	"errors"
	"testing"
	"urlshortener/internal/store"
)

func TestService_ShortenAndResolve(t *testing.T) {
	svc := NewService(store.NewMemory())
	ctx := context.Background()

	const original = "https://sample.com/?q=1"

	code, err := svc.Shorten(ctx, original)
	if err != nil {
		t.Fatalf("Shorten %v", err)
	}
	if len(code) != codeLength {
		t.Fatalf("got code length %d, want %d", len(code), codeLength)
	}

	got, err := svc.Resolve(ctx, code)
	if err != nil {
		t.Fatalf("Resolve %v", err)
	}
	if got != original {
		t.Errorf("Resolve = %q, want %q", got, original)
	}
}

func TestService_Shorten_Invalid(t *testing.T) {
	svc := NewService(store.NewMemory())
	ctx := context.Background()
	for _, url := range []string{"", " ", "abc", "ftp://hello.com", "http://"} {
		if _, err := svc.Shorten(ctx, url); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("Shorten (%q) error = %v, want invalid error", url, err)
		}
	}
}

func TestService_Resolve_NotFound(t *testing.T) {
	svc := NewService(store.NewMemory())
	ctx := context.Background()
	if _, err := svc.Resolve(ctx, "Missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
