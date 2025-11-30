package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func defaultSubmitState(r *http.Request) model.SubmitFormState {
	state := model.SubmitFormState{
		Platforms: []model.PlatformFormRow{},
		Errors: model.SubmitFormErrors{
			Platforms: make(map[string]model.PlatformFieldError),
		},
	}
	if v := r.URL.Query().Get("name"); v != "" {
		state.Name = v
	}
	if v := r.URL.Query().Get("description"); v != "" {
		state.Description = v
	}
	if v := r.URL.Query()["language"]; len(v) > 0 {
		state.Languages = v
	}
	if v := r.URL.Query().Get("platform_url"); v != "" {
		row := forms.NewPlatformRow()
		row.ChannelURL = v
		state.Platforms = append(state.Platforms, row)
	}
	ensureSubmitDefaults(&state)
	return state
}

func ensureSubmitDefaults(state *model.SubmitFormState) {
	if state == nil {
		return
	}
	if len(state.Platforms) == 0 {
		state.Platforms = []model.PlatformFormRow{forms.NewPlatformRow()}
	}
	if state.Errors.Platforms == nil {
		state.Errors.Platforms = make(map[string]model.PlatformFieldError)
	}
}

func hasSubmitErrors(errs model.SubmitFormErrors) bool {
	return errs.Name || errs.Description || errs.Languages || len(errs.Platforms) > 0
}

func parseSubmitForm(r *http.Request) (model.SubmitFormState, []int, error) {
	if err := r.ParseForm(); err != nil {
		return model.SubmitFormState{}, nil, err
	}

	state := model.SubmitFormState{
		Name:        strings.TrimSpace(r.Form.Get("name")),
		Description: strings.TrimSpace(r.Form.Get("description")),
		Languages:   normalizeLanguages(append([]string{}, r.Form["language"]...)),
	}
	state.Languages = append(state.Languages, normalizeLanguages(r.Form["languages"])...)

	removed := make([]int, 0, 4)
	for _, idx := range r.Form["remove_platform"] {
		row, err := strconv.Atoi(idx)
		if err != nil || row < 0 {
			continue
		}
		removed = append(removed, row)
	}

	for row := 0; ; row++ {
		prefix := fmt.Sprintf("platform[%d].", row)
		platform := r.Form.Get(prefix + "platform")
		handle := r.Form.Get(prefix + "handle")
		channelURL := r.Form.Get(prefix + "channel_url")
		if platform == "" && handle == "" && channelURL == "" {
			if row == 0 {
				continue
			}
			break
		}
		state.Platforms = append(state.Platforms, model.PlatformFormRow{
			Name:       strings.TrimSpace(platform),
			Handle:     strings.TrimSpace(handle),
			ChannelURL: strings.TrimSpace(channelURL),
		})
	}

	return state, removed, nil
}

func normalizeLanguages(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func removePlatformRow(platforms []model.PlatformFormRow, row int) []model.PlatformFormRow {
	if row < 0 || row >= len(platforms) {
		return platforms
	}
	out := make([]model.PlatformFormRow, 0, len(platforms)-1)
	out = append(out, platforms[:row]...)
	out = append(out, platforms[row+1:]...)
	return out
}

func submitStreamer(ctx context.Context, streamerSvc StreamerService, form model.SubmitFormState) (string, error) {
	if streamerSvc == nil {
		return "", errors.New("streamer service unavailable")
	}
	req := streamersvc.CreateRequest{
		Alias:       strings.TrimSpace(form.Name),
		Description: forms.BuildStreamerDescription(form.Description, form.Platforms),
		Languages:   append([]string(nil), form.Languages...),
		PlatformURL: forms.FirstPlatformURL(form.Platforms),
	}
	result, err := streamerSvc.Create(ctx, req)
	if err != nil {
		return "", err
	}
	alias := strings.TrimSpace(result.Submission.Alias)
	id := strings.TrimSpace(result.Submission.ID)
	switch {
	case alias != "" && id != "":
		return fmt.Sprintf("%s queued with submission %s.", alias, id), nil
	case alias != "":
		return fmt.Sprintf("%s submitted for review.", alias), nil
	default:
		return "Streamer submitted successfully.", nil
	}
}
