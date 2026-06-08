package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type recordingPublisher struct {
	mu     sync.Mutex
	events []ClickEvent
}

func (r *recordingPublisher) PublishClick(_ context.Context, e ClickEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *recordingPublisher) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func TestAsync_ForwardsToInner(t *testing.T) {
	rec := &recordingPublisher{}
	a := NewAsync(rec, 16, 2, discardLogger())

	for i := 0; i < 5; i++ {
		if err := a.PublishClick(context.Background(), ClickEvent{Code: "abc"}); err != nil {
			t.Fatalf("PublishClick: %v", err)
		}
	}
	a.Close()
	if got := rec.count(); got != 5 {
		t.Errorf("forwarded %d events, want 5", got)
	}
}

func TestAsync_DropsWhenFull(t *testing.T) {
	a := NewAsync(&recordingPublisher{}, 1, 0, discardLogger())
	if err := a.PublishClick(context.Background(), ClickEvent{Code: "abc"}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if err := a.PublishClick(context.Background(), ClickEvent{Code: "abc"}); !errors.Is(err, ErrBufferFull) {
		t.Errorf("second publish err = %v, want ErrBufferFull", err)
	}
	a.Close()
}

func TestNoop(t *testing.T) {
	if err := (Noop{}).PublishClick(context.Background(), ClickEvent{Code: "abc"}); err != nil {
		t.Errorf("Noop returned %v, want nil", err)
	}
}
