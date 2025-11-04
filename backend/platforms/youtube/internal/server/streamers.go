package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
)

// StreamerDirectory maps YouTube channel IDs to streamer names.
type StreamerDirectory map[string]string

// LoadStreamerDirectory loads streamer records from a JSON file and returns a lookup map.
func LoadStreamerDirectory(streamersPath string) (StreamerDirectory, error) {
	resolved, err := resolveStreamersPath(streamersPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}

	var streamers []storage.Streamer
	if err := json.Unmarshal(data, &streamers); err != nil {
		return nil, err
	}

	lookup := make(StreamerDirectory)
	for _, streamer := range streamers {
		name := strings.TrimSpace(streamer.Name)
		if name == "" {
			continue
		}

		for _, platform := range streamer.Platforms {
			if !strings.EqualFold(strings.TrimSpace(platform.Name), "youtube") {
				continue
			}

			channelID := strings.TrimSpace(platform.ID)
			if channelID == "" {
				channelID = channelIDFromURL(platform.ChannelURL)
			}
			if channelID == "" {
				continue
			}
			lookup[channelID] = name
		}
	}

	return lookup, nil
}

func channelIDFromURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}

	if id := strings.TrimSpace(u.Query().Get("channel_id")); id != "" {
		return id
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(segments); i++ {
		segment := strings.TrimSpace(segments[i])
		if !strings.EqualFold(segment, "channel") {
			continue
		}
		if i+1 < len(segments) {
			return strings.TrimSpace(segments[i+1])
		}
	}

	return ""
}

func (d StreamerDirectory) Name(channelID string) (string, bool) {
	if d == nil {
		return "", false
	}
	name, ok := d[strings.TrimSpace(channelID)]
	return name, ok
}

func (d StreamerDirectory) Contains(channelID string) bool {
	_, ok := d.Name(channelID)
	return ok
}

func DefaultStreamersPath() string {
	return path.Join("backend", "data", "streamers.json")
}

func resolveStreamersPath(streamersPath string) (string, error) {
	trimmed := strings.TrimSpace(streamersPath)
	if trimmed == "" {
		return "", errors.New("server: streamers path is required")
	}

	candidates := []string{trimmed}
	if !filepath.IsAbs(trimmed) {
		if abs, err := filepath.Abs(trimmed); err == nil {
			candidates = append(candidates, abs)
		}

		if wd, err := os.Getwd(); err == nil {
			dir := wd
			for {
				candidate := filepath.Join(dir, trimmed)
				candidates = append(candidates, candidate)
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				dir = parent
			}
		}
	}

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("server: streamers file not found: %s", streamersPath)
}
