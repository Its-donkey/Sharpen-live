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
		s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
		return
	}
	if platformURL != "" {
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
			currentPlatformURL = youtubeChannelURLFromPlatform(record.Platforms.YouTube)
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
		s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
		return
	}
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
	out := make([]model.Streamer, 0, len(records))
	for _, rec := range records {
		name := strings.TrimSpace(rec.Streamer.Alias)
		if name == "" {
			name = rec.Streamer.ID
		}
		var platforms []model.Platform
		if yt := rec.Platforms.YouTube; yt != nil {
			if url := youtubeChannelURLFromPlatform(yt); url != "" {
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
		out = append(out, model.Streamer{
			ID:          rec.Streamer.ID,
			Name:        name,
			Description: strings.TrimSpace(rec.Streamer.Description),
			Languages:   append([]string(nil), rec.Streamer.Languages...),
			Platforms:   platforms,
		})
	}
	return out
}
