// file name — /internal/ui/state/roster_cache.go
package state

import (
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"sync"
)

// RosterCache maintains an in-memory snapshot of streamers.
type RosterCache struct {
	mu        sync.RWMutex
	streamers []model.Streamer
}

// NewRosterCache constructs an empty RosterCache.
func NewRosterCache() *RosterCache {
	return &RosterCache{}
}

// Snapshot returns a copy of the current snapshot.
//
// Callers can safely modify the returned slice without affecting the cache.
func (c *RosterCache) Snapshot() []model.Streamer {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cp := make([]model.Streamer, len(c.streamers))
	copy(cp, c.streamers)
	return cp
}

// Update replaces the current snapshot.
func (c *RosterCache) Update(streamers []model.Streamer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cp := make([]model.Streamer, len(streamers))
	copy(cp, streamers)
	c.streamers = cp
}
