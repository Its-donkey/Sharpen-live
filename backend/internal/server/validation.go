package server

import (
	"fmt"
	"strings"

	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
)

const (
	maxStreamerNameLength = 80
	maxDescriptionLength  = 480
	maxPlatformsPerEntry  = 8
	maxLanguagesPerEntry  = 8
)

var statusDefaults = map[string]string{
	"online":  "Online",
	"busy":    "Workshop",
	"offline": "Offline",
}

func normalizeSubmission(req *submissionRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.StatusLabel = strings.TrimSpace(req.StatusLabel)
	req.Languages = filterStrings(req.Languages, maxLanguagesPerEntry)
	req.Platforms = filterPlatforms(req.Platforms, maxPlatformsPerEntry)

	if req.StatusLabel == "" && statusDefaults[req.Status] != "" {
		req.StatusLabel = statusDefaults[req.Status]
	} else if req.StatusLabel == "" {
		req.StatusLabel = statusDefaults["offline"]
	}
}

func validateSubmission(req submissionRequest) []string {
	var errs []string

	if req.Name == "" {
		errs = append(errs, "Streamer name is required.")
	} else if len(req.Name) > maxStreamerNameLength {
		errs = append(errs, fmt.Sprintf("Streamer name must be under %d characters.", maxStreamerNameLength))
	}

	if req.Description == "" {
		errs = append(errs, "Description is required.")
	} else if len(req.Description) > maxDescriptionLength {
		errs = append(errs, fmt.Sprintf("Description must be under %d characters.", maxDescriptionLength))
	}

	if req.Status == "" || statusDefaults[req.Status] == "" {
		errs = append(errs, "Status is required and must be one of: online, busy, or offline.")
	}

	if len(req.Languages) == 0 {
		errs = append(errs, "At least one language is required.")
	}

	if len(req.Platforms) == 0 {
		errs = append(errs, "At least one platform with a channel URL is required.")
	}

	return errs
}

func normalizeStreamer(req *streamerRequest) {
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.StatusLabel = strings.TrimSpace(req.StatusLabel)
	req.Languages = filterStrings(req.Languages, maxLanguagesPerEntry)
	req.Platforms = filterPlatforms(req.Platforms, maxPlatformsPerEntry)

	if req.StatusLabel == "" && statusDefaults[req.Status] != "" {
		req.StatusLabel = statusDefaults[req.Status]
	} else if req.StatusLabel == "" {
		req.StatusLabel = statusDefaults["offline"]
	}
}

func validateStreamer(req streamerRequest) []string {
	var errs []string

	if req.Name == "" {
		errs = append(errs, "Streamer name is required.")
	} else if len(req.Name) > maxStreamerNameLength {
		errs = append(errs, fmt.Sprintf("Streamer name must be under %d characters.", maxStreamerNameLength))
	}

	if req.Description == "" {
		errs = append(errs, "Description is required.")
	} else if len(req.Description) > maxDescriptionLength {
		errs = append(errs, fmt.Sprintf("Description must be under %d characters.", maxDescriptionLength))
	}

	if req.Status == "" || statusDefaults[req.Status] == "" {
		errs = append(errs, "Status is required and must be one of: online, busy, or offline.")
	}

	if len(req.Languages) == 0 {
		errs = append(errs, "At least one language is required.")
	}

	if len(req.Platforms) == 0 {
		errs = append(errs, "At least one platform with a channel URL is required.")
	}

	return errs
}

func filterStrings(values []string, max int) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			result = append(result, trimmed)
		}
		if max > 0 && len(result) >= max {
			break
		}
	}
	return result
}

func filterPlatforms(values []storage.Platform, max int) []storage.Platform {
	result := make([]storage.Platform, 0, len(values))
	for _, v := range values {
		entry := storage.Platform{
			Name:       strings.TrimSpace(v.Name),
			ChannelURL: strings.TrimSpace(v.ChannelURL),
			ID:         strings.TrimSpace(v.ID),
		}
		if entry.Name == "" || entry.ChannelURL == "" {
			continue
		}
		result = append(result, entry)
		if max > 0 && len(result) >= max {
			break
		}
	}
	return result
}
