package events

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrBufferFull = errors.New("publish buffer full")

type Async struct {
	inner   Publisher
	buf     chan ClickEvent
	wg      sync.WaitGroup
	log     *slog.Logger
	timeout time.Duration
}

func NewAsync(inner Publisher, bufSize, workers int, log *slog.Logger) *Async {
	a := &Async{
		inner:   inner,
		buf:     make(chan ClickEvent, bufSize),
		log:     log,
		timeout: 2 * time.Second,
	}
	for i := 0; i < workers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
	return a
}

func (a *Async) worker() {
	defer a.wg.Done()
	for event := range a.buf {
		ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
		if err := a.inner.PublishClick(ctx, event); err != nil {
			a.log.Warn("async publish click failed", "err", err, "code", event.Code)
		}
		cancel()
	}
}

func (a *Async) PublishClick(_ context.Context, event ClickEvent) error {
	select {
	case a.buf <- event:
		return nil
	default:
		return ErrBufferFull
	}
}

func (a *Async) Close() {
	close(a.buf)
	a.wg.Wait()
}
