package state

import "github.com/Its-donkey/Sharpen-live/internal/ui/model"

var rosterSnapshot []model.Streamer

// StoreRosterSnapshot caches the latest public roster for offline use in the WASM UI.
func StoreRosterSnapshot(streamers []model.Streamer) {
	if len(streamers) == 0 {
		return
	}
	rosterSnapshot = cloneStreamers(streamers)
}

// LoadRosterSnapshot returns a deep copy of the cached roster if it exists.
func LoadRosterSnapshot() []model.Streamer {
	if len(rosterSnapshot) == 0 {
		return nil
	}
	return cloneStreamers(rosterSnapshot)
}

func cloneStreamers(entries []model.Streamer) []model.Streamer {
	if len(entries) == 0 {
		return nil
	}
	clones := make([]model.Streamer, len(entries))
	for i, s := range entries {
		clones[i] = model.Streamer{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Status:      s.Status,
			StatusLabel: s.StatusLabel,
			Languages:   append([]string(nil), s.Languages...),
			Platforms:   clonePlatforms(s.Platforms),
		}
	}
	return clones
}

func clonePlatforms(entries []model.Platform) []model.Platform {
	if len(entries) == 0 {
		return nil
	}
	clones := make([]model.Platform, len(entries))
	copy(clones, entries)
	return clones
}
