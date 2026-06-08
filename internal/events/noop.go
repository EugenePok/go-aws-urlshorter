package events

import "context"

// Noop is a Publisher that discards events. Used when async click tracking is
// disabled (no queue configured). It is trivially non-blocking.
type Noop struct{}

func (Noop) PublishClick(context.Context, ClickEvent) error {
	return nil
}
