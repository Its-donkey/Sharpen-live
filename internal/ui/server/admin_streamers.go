package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/onboarding"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	youtubeui "github.com/Its-donkey/Sharpen-live/internal/ui/platforms/youtube"
)

func (s *server) handleAdminStreamerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid update request.")
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to edit streamers.")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	alias := strings.TrimSpace(r.FormValue("alias"))
	description := strings.TrimSpace(r.FormValue("description"))
	languages := parseLanguagesInput(r.FormValue("languages"))
	platformURL := strings.TrimSpace(r.FormValue("platform_url"))
	if id == "" || alias == "" || description == "" {
		s.redirectAdmin(w, r, "", "Name and description are required.")
		return
	}
	if s.streamerService == nil {
		s.redirectAdmin(w, r, "", "Streamer service unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	_, err := s.streamerService.Update(ctx, streamersvc.UpdateRequest{
		ID:          id,
		Alias:       &alias,
		Description: &description,
		Languages:   &languages,
	})
	if err != nil {
		s.logger.Warn("admin", "streamer update failed", map[string]any{
			"streamer_id": id,
			"error":       err.Error(),
		})
		s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
		return
	}
	s.logger.Info("admin", "streamer updated", map[string]any{
		"streamer_id": id,
		"alias":       alias,
	})
	if platformURL != "" {
		// Check if YouTube is enabled before allowing platform updates
		if !s.isYouTubeEnabled() {
			s.logger.Warn("admin", "YouTube disabled, skipping platform update", map[string]any{
				"streamerId": id,
				"siteKey":    s.siteKey,
			})
			s.redirectAdmin(w, r, "", "YouTube is disabled for this site. Enable it in settings to update platforms.")
			return
		}
		baseStore, ok := s.streamersStore.(*streamers.Store)
		if !ok {
			s.redirectAdmin(w, r, "", "Platform updates are unavailable.")
			return
		}
		record, err := baseStore.Get(id)
		if err != nil {
			s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
			return
		}
		currentPlatformURL := ""
		if record.Platforms.YouTube != nil {
			currentPlatformURL = youtubeui.ChannelURLFromPlatform(record.Platforms.YouTube)
		}
		if strings.EqualFold(strings.TrimSpace(currentPlatformURL), platformURL) {
			s.redirectAdmin(w, r, "Streamer updated.", "")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		onboardErr := onboarding.FromURL(ctx, record, platformURL, onboarding.Options{
			Client:       &http.Client{Timeout: 10 * time.Second},
			HubURL:       s.youtubeConfig.HubURL,
			CallbackURL:  s.youtubeConfig.CallbackURL,
			VerifyMode:   s.youtubeConfig.Verify,
			LeaseSeconds: s.youtubeConfig.LeaseSeconds,
			Store:        baseStore,
		})
		if onboardErr != nil {
			s.redirectAdmin(w, r, "", fmt.Sprintf("Failed to update platform: %v", onboardErr))
			return
		}
	}
	s.redirectAdmin(w, r, "Streamer updated.", "")
}

func (s *server) handleAdminStreamerDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid delete request.")
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to delete streamers.")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		s.redirectAdmin(w, r, "", "Missing streamer id.")
		return
	}
	if s.streamerService == nil {
		s.redirectAdmin(w, r, "", "Streamer service unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	err := s.streamerService.Delete(ctx, streamersvc.DeleteRequest{ID: id})
	if err != nil {
		s.logger.Warn("admin", "streamer delete failed", map[string]any{
			"streamer_id": id,
			"error":       err.Error(),
		})
		s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
		return
	}
	s.logger.Info("admin", "streamer deleted", map[string]any{
		"streamer_id": id,
	})
	s.redirectAdmin(w, r, "Streamer removed.", "")
}

func parseLanguagesInput(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
		if len(values) >= model.MaxLanguages {
			break
		}
	}
	return values
}

func mapStreamerRecords(records []streamers.Record) []model.Streamer {
	online := make([]model.Streamer, 0, len(records))
	offline := make([]model.Streamer, 0, len(records))
	for _, rec := range records {
		name := strings.TrimSpace(rec.Streamer.Alias)
		if name == "" {
			name = rec.Streamer.ID
		}
		status, statusLabel, isLive := mapStreamerStatus(rec.Status)
		var platforms []model.Platform
		if yt := rec.Platforms.YouTube; yt != nil {
			if url := youtubePlatformURL(yt, rec.Status, isLive); url != "" {
				platforms = append(platforms, model.Platform{Name: "YouTube", ChannelURL: url})
			}
		}
		if tw := rec.Platforms.Twitch; tw != nil {
			if username := strings.TrimSpace(tw.Username); username != "" {
				platforms = append(platforms, model.Platform{
					Name:       "Twitch",
					ChannelURL: "https://www.twitch.tv/" + username,
				})
			}
		}
		if fb := rec.Platforms.Facebook; fb != nil {
			if page := strings.TrimSpace(fb.PageID); page != "" {
				platforms = append(platforms, model.Platform{
					Name:       "Facebook",
					ChannelURL: "https://www.facebook.com/" + page,
				})
			}
		}
		mapped := model.Streamer{
			ID:          rec.Streamer.ID,
			Name:        name,
			Description: strings.TrimSpace(rec.Streamer.Description),
			Status:      status,
			StatusLabel: statusLabel,
			Languages:   append([]string(nil), rec.Streamer.Languages...),
			Platforms:   platforms,
		}
		if isLive {
			online = append(online, mapped)
		} else {
			offline = append(offline, mapped)
		}
	}
	return append(online, offline...)
}

func mapStreamerStatus(status *streamers.Status) (state, label string, live bool) {
	state = "offline"
	label = model.StatusLabels[state]
	if status == nil {
		return
	}

	live = status.Live
	if yt := status.YouTube; yt != nil && yt.Live {
		live = true
	}
	if tw := status.Twitch; tw != nil && tw.Live {
		live = true
	}
	if fb := status.Facebook; fb != nil && fb.Live {
		live = true
	}

	switch {
	case live:
		state = "online"
	case len(status.Platforms) > 0:
		state = "busy"
	default:
		state = "offline"
	}

	if lbl := model.StatusLabels[state]; lbl != "" {
		label = lbl
	} else {
		label = state
	}
	return
}

func youtubePlatformURL(yt *streamers.YouTubePlatform, status *streamers.Status, live bool) string {
	channelURL := youtubeui.ChannelURLFromPlatform(yt)
	if channelURL == "" {
		return ""
	}
	if status != nil && status.YouTube != nil && status.YouTube.Live {
		if vid := strings.TrimSpace(status.YouTube.VideoID); vid != "" {
			return "https://www.youtube.com/watch?v=" + vid
		}
		return youtubeui.LiveURLFromChannel(channelURL)
	}
	if live {
		return youtubeui.LiveURLFromChannel(channelURL)
	}
	return channelURL
}
