package store

import (
	"context"
	"sync"
)

type Memory struct {
	mu    sync.RWMutex
	links map[string]Link
}

func NewMemory() *Memory {
	return &Memory{links: make(map[string]Link)}
}

func (m *Memory) Save(_ context.Context, link Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.links[link.Code]; ok {
		return ErrCodeExists
	}
	m.links[link.Code] = link
	return nil
}

func (m *Memory) Get(_ context.Context, code string) (Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return link, nil
}
