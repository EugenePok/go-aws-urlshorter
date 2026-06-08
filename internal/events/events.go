package events

import (
	"context"
	"time"
)

type ClickEvent struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent,omitempty"`
	Referer   string    `json:"referer,omitempty"`
}

type Publisher interface {
	PublishClick(ctx context.Context, event ClickEvent) error
}
