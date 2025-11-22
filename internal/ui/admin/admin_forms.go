//go:build js && wasm

package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	state "github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

func newStreamerFormFromStreamer(s model.Streamer) *model.AdminStreamerForm {
	form := &model.AdminStreamerForm{
		ID:             s.ID,
		Name:           s.Name,
		Description:    s.Description,
		Status:         defaultStatusValue(s.Status),
		StatusLabel:    strings.TrimSpace(s.StatusLabel),
		LanguagesInput: strings.Join(s.Languages, ", "),
		Platforms:      make([]model.PlatformFormRow, 0, len(s.Platforms)),
	}
	if form.StatusLabel == "" {
		form.StatusLabel = model.StatusLabels[form.Status]
	}
	if len(form.Platforms) == 0 {
		for _, p := range s.Platforms {
			form.Platforms = append(form.Platforms, model.PlatformFormRow{
				ID:         fmt.Sprintf("platform-%d", time.Now().UnixNano()),
				Name:       p.Name,
				ChannelURL: p.ChannelURL,
			})
		}
	}
	if len(form.Platforms) == 0 {
		form.Platforms = []model.PlatformFormRow{forms.NewPlatformRow()}
	}
	return form
}

func defaultStatusValue(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "online" || value == "busy" || value == "offline" {
		return value
	}
	return "online"
}

func parseLanguagesInput(input string) []string {
	entries := strings.Split(input, ",")
	values := make([]string, 0, len(entries))
	seen := make(map[string]struct{})
	for _, item := range entries {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
	}
	return values
}

func streamerFormByKey(key string) *model.AdminStreamerForm {
	if key == "" {
		return nil
	}
	return state.AdminConsole.StreamerForms[key]
}

func ensurePlatformRows(form *model.AdminStreamerForm) {
	if form == nil {
		return
	}
	if len(form.Platforms) == 0 {
		form.Platforms = []model.PlatformFormRow{forms.NewPlatformRow()}
	}
}
