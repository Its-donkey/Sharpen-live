package settings

import (
	"context"
	"sync"
)

// MemoryStore provides an in-memory implementation of Store for tests.
type MemoryStore struct {
	mu       sync.RWMutex
	settings Settings
	ok       bool
}

// NewMemoryStore constructs a MemoryStore optionally seeded with settings.
func NewMemoryStore(initial Settings, hasValue bool) *MemoryStore {
	return &MemoryStore{
		settings: initial,
		ok:       hasValue,
	}
}

// EnsureSchema satisfies the Store interface. No-op for memory store.
func (m *MemoryStore) EnsureSchema(context.Context) error {
	return nil
}

// Load returns the stored settings or ErrNotFound when none have been saved.
func (m *MemoryStore) Load(ctx context.Context) (Settings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ok {
		return Settings{}, ErrNotFound
	}
	return m.settings, nil
}

// Save replaces the stored settings.
func (m *MemoryStore) Save(ctx context.Context, settings Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = settings
	m.ok = true
	return nil
}
