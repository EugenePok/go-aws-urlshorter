package stats

import (
	"context"
	"time"
)

type Delta struct {
	Code        string
	Count       int64
	LastClicked time.Time
}

type Store interface {
	ApplyDeltas(ctx context.Context, deltas []Delta) error
}
