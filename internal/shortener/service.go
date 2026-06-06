package shortener

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"urlshortener/internal/store"
)

const codeLength = 7
const maxRetries = 5

var ErrInvalidURL = errors.New("invalid URL")

type Service struct {
	store store.Store
}

func NewService(store store.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Shorten(ctx context.Context, longURL string) (string, error) {
	longURL = strings.TrimSpace(longURL)
	if err := validateURL(longURL); err != nil {
		return "", err
	}
	for attempt := 0; attempt < maxRetries; attempt++ {
		code, err := GenerateCode(codeLength)
		if err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}
		err = s.store.Save(ctx, store.Link{
			Code:      code,
			LongURL:   longURL,
			CreatedAt: time.Now().UTC(),
		})
		switch {
		case err == nil:
			return code, nil
		case errors.Is(err, store.ErrCodeExists):
			continue //collisions happened
		default:
			return "", fmt.Errorf("Save Link: %w", err)
		}
	}
	return "", errors.New("could not generate a unique short code after retries")
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	link, err := s.store.Get(ctx, code)
	if err != nil {
		return "", err
	}
	return link.LongURL, nil
}

func validateURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: empty url", ErrInvalidURL)
	}
	url, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}
	if url.Host == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	return nil
}
