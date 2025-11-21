//go:build js && wasm

package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func findStreamerByID(id string) *model.Streamer {
	for idx := range adminState.Streamers {
		if adminState.Streamers[idx].ID == id {
			return &adminState.Streamers[idx]
		}
	}
	return nil
}

func handleModerateAction(action, id string) {
	action = strings.TrimSpace(action)
	id = strings.TrimSpace(id)
	if id == "" || (action != "approve" && action != "reject") {
		return
	}
	setAdminStatus(model.AdminStatus{Tone: "info", Message: strings.Title(action) + " submission…"})
	go func(act, submissionID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := adminModerateSubmission(ctx, act, submissionID); err != nil {
			setAdminStatus(model.AdminStatus{Tone: "error", Message: err.Error()})
			return
		}
		setTransientStatus(model.AdminStatus{Tone: "success", Message: "Submission updated."})
		refreshAdminData()
	}(action, id)
}

func handleDeleteStreamer(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	streamer := findStreamerByID(id)
	if streamer != nil {
		prompt := fmt.Sprintf("Remove %s from the roster?", streamer.Name)
		confirm := js.Global().Call("confirm", prompt)
		if !confirm.Truthy() {
			return
		}
	}
	setAdminStatus(model.AdminStatus{Tone: "info", Message: "Removing streamer…"})
	go func(streamerID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := adminDeleteStreamer(ctx, streamerID); err != nil {
			setAdminStatus(model.AdminStatus{Tone: "error", Message: err.Error()})
			return
		}
		setTransientStatus(model.AdminStatus{Tone: "success", Message: "Streamer removed."})
		refreshAdminData()
	}(id)
}

func handleToggleStreamerForm(id string) {
	form := adminState.StreamerForms[id]
	if form == nil {
		source := findStreamerByID(id)
		if source == nil {
			return
		}
		form = newStreamerFormFromStreamer(*source)
		adminState.StreamerForms[id] = form
	}
	form.Visible = !form.Visible
	scheduleAdminRender()
}

func handleCancelStreamerEdit(id string) {
	source := findStreamerByID(id)
	if source == nil {
		return
	}
	form := newStreamerFormFromStreamer(*source)
	form.Visible = false
	adminState.StreamerForms[id] = form
	scheduleAdminRender()
}

func handleStreamerFieldChange(key, field, value string) {
	form := streamerFormByKey(key)
	if form == nil {
		return
	}
	shouldRender := false
	switch field {
	case "name":
		form.Name = value
	case "description":
		form.Description = value
	case "languages":
		form.LanguagesInput = value
	case "status":
		form.Status = defaultStatusValue(value)
		if !form.StatusLabelDirty {
			form.StatusLabel = model.StatusLabels[form.Status]
			shouldRender = true
		}
	case "statusLabel":
		form.StatusLabel = value
		form.StatusLabelDirty = strings.TrimSpace(value) != ""
	}
	if shouldRender {
		scheduleAdminRender()
	}
}

func handlePlatformFieldChange(key, rowID, field, value string) {
	form := streamerFormByKey(key)
	if form == nil {
		return
	}
	for idx := range form.Platforms {
		if form.Platforms[idx].ID == rowID {
			if field == "name" {
				form.Platforms[idx].Name = value
			} else {
				form.Platforms[idx].ChannelURL = value
			}
			break
		}
	}
}

func handleAddPlatformRow(key string) {
	form := streamerFormByKey(key)
	if form == nil {
		return
	}
	form.Platforms = append(form.Platforms, forms.NewPlatformRow())
	scheduleAdminRender()
}

func handleRemovePlatformRow(key, rowID string) {
	form := streamerFormByKey(key)
	if form == nil {
		return
	}
	if len(form.Platforms) <= 1 {
		form.Platforms = []model.PlatformFormRow{forms.NewPlatformRow()}
		scheduleAdminRender()
		return
	}
	next := make([]model.PlatformFormRow, 0, len(form.Platforms)-1)
	for _, row := range form.Platforms {
		if row.ID != rowID {
			next = append(next, row)
		}
	}
	form.Platforms = next
	scheduleAdminRender()
}

func handleSubmitStreamerForm(key string) {
	form := streamerFormByKey(key)
	if form == nil {
		return
	}
	payload, err := buildAdminStreamerPayload(form)
	if err != nil {
		form.Error = err.Error()
		scheduleAdminRender()
		return
	}
	form.Saving = true
	form.Error = ""
	scheduleAdminRender()

	go func(formKey string, f *model.AdminStreamerForm, payload model.AdminSubmissionPayload) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := adminUpdateStreamer(ctx, formKey, payload)
		f.Saving = false
		if err != nil {
			f.Error = err.Error()
			scheduleAdminRender()
			return
		}
		f.Visible = false
		setTransientStatus(model.AdminStatus{Tone: "success", Message: "Streamer updated."})
		refreshAdminData()
	}(key, form, payload)
}

func buildAdminStreamerPayload(form *model.AdminStreamerForm) (model.AdminSubmissionPayload, error) {
	name := strings.TrimSpace(form.Name)
	if name == "" {
		return model.AdminSubmissionPayload{}, errors.New("Name is required.")
	}
	description := strings.TrimSpace(form.Description)
	if description == "" {
		return model.AdminSubmissionPayload{}, errors.New("Description is required.")
	}
	languages := parseLanguagesInput(form.LanguagesInput)
	if len(languages) == 0 {
		return model.AdminSubmissionPayload{}, errors.New("Provide at least one language.")
	}
	platforms := adminPlatformPayloads(form.Platforms)
	if len(platforms) == 0 {
		return model.AdminSubmissionPayload{}, errors.New("Add at least one platform with a channel URL.")
	}
	status := defaultStatusValue(form.Status)
	label := strings.TrimSpace(form.StatusLabel)
	if label == "" {
		label = model.StatusLabels[status]
	}
	payload := model.AdminSubmissionPayload{
		Name:        name,
		Description: description,
		Status:      status,
		StatusLabel: label,
		Languages:   languages,
		Platforms:   platforms,
	}
	return payload, nil
}

func adminPlatformPayloads(rows []model.PlatformFormRow) []model.Platform {
	payloads := make([]model.Platform, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		channel := strings.TrimSpace(row.ChannelURL)
		if name == "" || channel == "" {
			continue
		}
		payloads = append(payloads, model.Platform{Name: name, ChannelURL: channel})
	}
	return payloads
}
