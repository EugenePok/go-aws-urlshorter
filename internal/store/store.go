package store

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("link not found")
var ErrCodeExists = errors.New("code already exists")

type Link struct {
	Code      string
	LongURL   string
	CreatedAt time.Time
}

type Store interface {
	Save(ctx context.Context, link Link) error
	Get(ctx context.Context, code string) (Link, error)
}
