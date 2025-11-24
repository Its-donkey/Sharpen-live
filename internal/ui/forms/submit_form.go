//go:build js && wasm

package forms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"syscall/js"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

var (
	formHandlers    []js.Func
	runtimeDocument js.Value
)

var platformHandleOptions = []struct {
	Value string
	Label string
}{
	{Value: "youtube", Label: "YouTube"},
	{Value: "twitch", Label: "Twitch"},
	{Value: "facebook", Label: "Facebook"},
}

func formDocument() js.Value {
	if !runtimeDocument.Truthy() {
		runtimeDocument = js.Global().Get("document")
	}
	return runtimeDocument
}

// RenderSubmitForm rebuilds the interactive submission form in the DOM.
func RenderSubmitForm() {
	container := formDocument().Call("getElementById", "submit-streamer-section")
	if !container.Truthy() {
		return
	}

	focus := captureFocusSnapshot()
	releaseFormHandlers()

	if len(state.Submit.Platforms) == 0 {
		state.Submit.Platforms = []model.PlatformFormRow{NewPlatformRow()}
	}
	if state.Submit.Errors.Platforms == nil {
		state.Submit.Errors.Platforms = make(map[string]model.PlatformFieldError)
	}

	var builder strings.Builder
	sectionClass := "submit-streamer"
	if !state.Submit.Open {
		sectionClass += " is-collapsed"
	}
	builder.WriteString(`<section class="` + sectionClass + `" aria-labelledby="submit-streamer-title">`)
	builder.WriteString(`
  <div class="submit-streamer-header">
    <h2 id="submit-streamer-title">Know a streamer we should feature?</h2>
    <button type="button" class="submit-streamer-toggle" id="submit-toggle">`)
	if state.Submit.Open {
		builder.WriteString("Hide form")
	} else {
		builder.WriteString("Submit a streamer")
	}
	builder.WriteString(`</button>
  </div>`)

	if state.Submit.Open {
		formClass := "submit-streamer-form"
		if state.Submit.Submitting {
			formClass += " is-submitting"
		}
		builder.WriteString(`<form id="submit-streamer-form" class="` + formClass + `" aria-live="polite">`)
		builder.WriteString(`<p class="submit-streamer-help">Share the details below and our team will review the submission before adding the streamer to the roster. No additional access is required.</p>`)

		// Form grid
		builder.WriteString(`<div class="form-grid">`)

		// Platform fieldset
		builder.WriteString(`<fieldset class="platform-fieldset form-field-wide"><legend>Streaming platforms *</legend><p class="submit-streamer-help">Add each platform’s name and channel URL. If they’re the same stream link, repeat the URL.</p>`)
		builder.WriteString(`<div class="platform-rows">`)
		for _, row := range state.Submit.Platforms {
			errors := state.Submit.Errors.Platforms[row.ID]
			channelWrapper := "form-field form-field-inline highlight-change"
			if errors.Channel {
				channelWrapper += " form-field-error"
			}
			builder.WriteString(`<div class="platform-row form-grid platform-row-grid" data-platform-row="` + row.ID + `">`)
			showPlatformName := strings.HasPrefix(strings.TrimSpace(row.ChannelURL), "@") || strings.HasPrefix(strings.TrimSpace(row.Handle), "@")
			if showPlatformName {
				builder.WriteString(`<label class="form-field form-field-inline platform-name"><span>Platform name</span>`)
				builder.WriteString(`<input type="text" value="` + html.EscapeString(row.Name) + `" data-platform-name data-row="` + row.ID + `" placeholder="YouTube" required />`)
				builder.WriteString(`</label>`)
			}
			builder.WriteString(`<div class="platform-channel-group">`)
			builder.WriteString(`<label class="` + channelWrapper + `" id="platform-url-field-` + row.ID + `"><span>Channel URL</span>`)
			builder.WriteString(`<input type="url" class="channel-url-input" placeholder="https://example.com/live or @handle" value="` + html.EscapeString(row.ChannelURL) + `" data-platform-channel data-row="` + row.ID + `" required />`)
			builder.WriteString(`</label>`)
			builder.WriteString(`</div>`)
			wrapperClass := "form-field form-field-inline platform-select-wrapper"
			if showPlatformName {
				wrapperClass += " is-visible"
			}
			builder.WriteString(`<label class="` + wrapperClass + `"><span>Handle platform</span>`)
			builder.WriteString(`<select class="platform-select" data-platform-choice data-row="` + row.ID + `">`)
			selected := resolvePlatformPreset(row.Preset)
			for _, option := range platformHandleOptions {
				builder.WriteString(`<option value="` + option.Value + `"`)
				if selected == option.Value {
					builder.WriteString(` selected`)
				}
				builder.WriteString(`>` + option.Label + `</option>`)
			}
			builder.WriteString(`</select></label>`)
			builder.WriteString(`<button type="button" class="remove-platform-button platform-remove-inline" data-remove-platform="` + row.ID + `">Remove</button>`)
			builder.WriteString(`</div>`)

			handleValue := strings.TrimSpace(row.Handle)
			if handleValue == "" {
				handleValue = inferHandleFromURL(row.ChannelURL)
			}
			if handleValue != "" || strings.TrimSpace(row.Preset) != "" {
				selected := resolvePlatformPreset(row.Preset)
				builder.WriteString(`<label class="form-field form-field-inline platform-select"><span>Handle platform</span>`)
				builder.WriteString(`<select data-platform-choice data-row="` + row.ID + `">`)
				for _, option := range platformHandleOptions {
					builder.WriteString(`<option value="` + option.Value + `"`)
					if selected == option.Value {
						builder.WriteString(` selected`)
					}
					builder.WriteString(`>` + option.Label + `</option>`)
				}
				builder.WriteString(`</select></label>`)
			}

			if errors.Channel {
				builder.WriteString(`<p class="field-error-text">Provide a valid channel URL.</p>`)
			}
			builder.WriteString(`</div>`)
		}
		builder.WriteString(`</div>`)

		addDisabled := ""
		if len(state.Submit.Platforms) >= model.MaxPlatforms {
			addDisabled = " disabled"
		}

		builder.WriteString(`<button type="button" class="add-platform-button" id="add-platform"` + addDisabled + `>+ Add another platform</button>`)
		builder.WriteString(`</fieldset>`)

		// Name field
		nameClass := "form-field"
		if state.Submit.Errors.Name {
			nameClass += " form-field-error"
		}
		builder.WriteString(`<label class="` + nameClass + `" id="field-name"><span>Streamer name *</span><input type="text" id="streamer-name" value="` + html.EscapeString(state.Submit.Name) + `" required /></label>`)

		// Description
		descClass := "form-field form-field-wide"
		if state.Submit.Errors.Description {
			descClass += " form-field-error"
		}
		builder.WriteString(`<label class="` + descClass + `" id="field-description"><span>Description *</span><p class="submit-streamer-help">What does the streamer do and what makes their streams unique?</p><textarea id="streamer-description" rows="3" required>`)
		builder.WriteString(html.EscapeString(state.Submit.Description))
		builder.WriteString(`</textarea></label>`)

		// Languages
		langClass := "form-field form-field-wide"
		if state.Submit.Errors.Languages {
			langClass += " form-field-error"
		}
		builder.WriteString(`<label class="` + langClass + `" id="field-languages"><span>Languages *</span><p class="submit-streamer-help">Select every language the streamer uses on their channel.</p>`)
		selectDisabled := len(state.Submit.Languages) >= model.MaxLanguages
		builder.WriteString(`<div class="language-picker">`)
		builder.WriteString(`<select class="language-select" id="language-select"`)
		if selectDisabled {
			builder.WriteString(` disabled`)
		}
		builder.WriteString(`>`)
		builder.WriteString(`<option value="">Languages</option>`)
		for _, option := range AvailableLanguageOptions(state.Submit.Languages) {
			builder.WriteString(`<option value="` + html.EscapeString(option.Value) + `">` + html.EscapeString(option.Label) + `</option>`)
		}
		builder.WriteString(`</select>`)
		builder.WriteString(`<div class="language-tags">`)
		if len(state.Submit.Languages) == 0 {
			builder.WriteString(`<span class="language-empty">No languages selected yet.</span>`)
		} else {
			for _, value := range state.Submit.Languages {
				label := DisplayLanguage(value)
				builder.WriteString(`<span class="language-pill">` + html.EscapeString(label) + `<button type="button" data-remove-language="` + html.EscapeString(value) + `" aria-label="Remove ` + html.EscapeString(label) + `">×</button></span>`)
			}
		}
		builder.WriteString(`</div>`)
		builder.WriteString(`</div>`)
		if state.Submit.Errors.Languages {
			builder.WriteString(`<p class="field-error-text">Select at least one language.</p>`)
		}
		builder.WriteString(`</label></div>`) // end languages label and grid

		// Actions
		builder.WriteString(`<div class="submit-streamer-actions">`)
		submitLabel := "Submit streamer"
		if state.Submit.Submitting {
			submitLabel = "Submitting…"
		}
		disableSubmit := ""
		if state.Submit.Submitting {
			disableSubmit = " disabled"
		}
		builder.WriteString(`<button type="submit" class="submit-streamer-submit"` + disableSubmit + `>` + submitLabel + `</button>`)
		builder.WriteString(`<button type="button" class="submit-streamer-cancel" id="submit-cancel">Cancel</button>`)
		builder.WriteString(`</div>`)

		if state.Submit.ResultState != "" && state.Submit.ResultMessage != "" {
			builder.WriteString(`<div class="submit-streamer-result" data-state="` + html.EscapeString(state.Submit.ResultState) + `" role="status">` + html.EscapeString(state.Submit.ResultMessage) + `</div>`)
		}

		builder.WriteString(`</form>`)
	}

	builder.WriteString(`</section>`)
	container.Set("innerHTML", builder.String())

	bindSubmitFormEvents()
	restoreFocusSnapshot(focus)
}

func scheduleRender() {
	var fn js.Func
	fn = js.FuncOf(func(js.Value, []js.Value) any {
		RenderSubmitForm()
		fn.Release()
		return nil
	})
	js.Global().Call("setTimeout", fn, 0)
}

type focusSnapshot struct {
	ID    string
	Start int
	End   int
}

func captureFocusSnapshot() focusSnapshot {
	active := formDocument().Get("activeElement")
	if !active.Truthy() {
		return focusSnapshot{Start: -1, End: -1}
	}
	idValue := active.Get("id")
	if idValue.Type() != js.TypeString {
		return focusSnapshot{Start: -1, End: -1}
	}
	snap := focusSnapshot{ID: idValue.String(), Start: -1, End: -1}
	if start := active.Get("selectionStart"); start.Type() == js.TypeNumber {
		snap.Start = start.Int()
	}
	if end := active.Get("selectionEnd"); end.Type() == js.TypeNumber {
		snap.End = end.Int()
	}
	return snap
}

func restoreFocusSnapshot(snap focusSnapshot) {
	if snap.ID == "" {
		return
	}
	target := formDocument().Call("getElementById", snap.ID)
	if !target.Truthy() {
		return
	}
	target.Call("focus")
	if snap.Start >= 0 && snap.End >= 0 {
		if setter := target.Get("setSelectionRange"); setter.Type() == js.TypeFunction {
			target.Call("setSelectionRange", snap.Start, snap.End)
		}
	}
}

func bindSubmitFormEvents() {
	toggle := formDocument().Call("getElementById", "submit-toggle")
	addFormHandler(toggle, "click", func(js.Value, []js.Value) any {
		state.Submit.Open = !state.Submit.Open
		if !state.Submit.Open {
			ResetFormState(true)
		}
		RenderSubmitForm()
		return nil
	})

	if !state.Submit.Open {
		return
	}

	nameInput := formDocument().Call("getElementById", "streamer-name")
	addFormHandler(nameInput, "input", func(this js.Value, _ []js.Value) any {
		value := this.Get("value").String()
		state.Submit.Name = value
		if strings.TrimSpace(value) != "" {
			state.Submit.Errors.Name = false
			markFieldError("field-name", false)
		}
		return nil
	})

	descInput := formDocument().Call("getElementById", "streamer-description")
	addFormHandler(descInput, "input", func(this js.Value, _ []js.Value) any {
		value := this.Get("value").String()
		state.Submit.Description = value
		if strings.TrimSpace(value) != "" {
			state.Submit.Errors.Description = false
			markFieldError("field-description", false)
		}
		return nil
	})

	langSelect := formDocument().Call("getElementById", "language-select")
	addFormHandler(langSelect, "change", func(this js.Value, _ []js.Value) any {
		value := strings.TrimSpace(this.Get("value").String())
		if value == "" {
			return nil
		}
		if len(state.Submit.Languages) >= model.MaxLanguages {
			return nil
		}
		if !ContainsString(state.Submit.Languages, value) {
			state.Submit.Languages = append(state.Submit.Languages, value)
			state.Submit.Errors.Languages = false
		}
		RenderSubmitForm()
		return nil
	})

	langButtons := formDocument().Call("querySelectorAll", "[data-remove-language]")
	forEachNode(langButtons, func(node js.Value) {
		addFormHandler(node, "click", func(this js.Value, _ []js.Value) any {
			value := this.Get("dataset").Get("removeLanguage").String()
			if value == "" {
				return nil
			}
			filtered := make([]string, 0, len(state.Submit.Languages))
			for _, entry := range state.Submit.Languages {
				if entry != value {
					filtered = append(filtered, entry)
				}
			}
			state.Submit.Languages = filtered
			if len(filtered) > 0 {
				state.Submit.Errors.Languages = false
			}
			RenderSubmitForm()
			return nil
		})
	})

	platformInputs := formDocument().Call("querySelectorAll", "[data-platform-channel]")
	forEachNode(platformInputs, func(node js.Value) {
		addFormHandler(node, "input", func(this js.Value, _ []js.Value) any {
			rowID := this.Get("dataset").Get("row").String()
			rawValue := strings.TrimSpace(this.Get("value").String())
			normalized := CanonicalizeChannelInput(rawValue)
			if normalized != rawValue {
				this.Set("value", normalized)
			}
			for index := range state.Submit.Platforms {
				row := &state.Submit.Platforms[index]
				if row.ID == rowID {
					if rawValue == "" {
						row.ChannelURL = ""
						row.Handle = ""
						row.Preset = ""
						row.Name = ""
						RenderSubmitForm()
						return nil
					}

					if handle := extractHandle(rawValue); handle != "" {
						row.Handle = handle
						if row.Preset == "" {
							row.Preset = "youtube"
						}
						normalized := buildURLFromHandle(handle, row.Preset)
						row.ChannelURL = normalized
						row.Name = DerivePlatformLabel(normalized)
						this.Set("value", normalized)
						if strings.TrimSpace(normalized) != "" {
							clearPlatformError(rowID, "channel")
						}
						currentValue := normalized
						rowIndex := index
						fetchChannelMetadataAsync(rowIndex, currentValue)
						RenderSubmitForm()
						return nil
					}

					handleChanged := false
					if row.Handle == "" {
						if inferred := inferHandleFromURL(normalized); inferred != "" {
							row.Handle = inferred
							if row.Preset == "" {
								row.Preset = "youtube"
							}
							handleChanged = true
						}
					}
					if row.Handle != "" && normalized != buildURLFromHandle(row.Handle, row.Preset) {
						row.Handle = ""
						row.Preset = ""
						handleChanged = true
					}
					row.ChannelURL = normalized
					row.Name = DerivePlatformLabel(normalized)
					if strings.TrimSpace(normalized) != "" {
						clearPlatformError(rowID, "channel")
					}
					currentValue := normalized
					rowIndex := index
					fetchChannelMetadataAsync(rowIndex, currentValue)
					if handleChanged {
						RenderSubmitForm()
					}
					break
				}
			}
			return nil
		})
	})

	platformChoices := formDocument().Call("querySelectorAll", "[data-platform-choice]")
	forEachNode(platformChoices, func(node js.Value) {
		addFormHandler(node, "change", func(this js.Value, _ []js.Value) any {
			rowID := this.Get("dataset").Get("row").String()
			choice := resolvePlatformPreset(this.Get("value").String())
			for index := range state.Submit.Platforms {
				row := &state.Submit.Platforms[index]
				if row.ID != rowID {
					continue
				}
				row.Preset = choice
				if row.Handle != "" {
					row.ChannelURL = buildURLFromHandle(row.Handle, row.Preset)
					row.Name = DerivePlatformLabel(row.ChannelURL)
					clearPlatformError(rowID, "channel")
					fetchChannelMetadataAsync(index, row.ChannelURL)
				}
				RenderSubmitForm()
				break
			}
			return nil
		})
	})
	removeButtons := formDocument().Call("querySelectorAll", "[data-remove-platform]")
	forEachNode(removeButtons, func(node js.Value) {
		addFormHandler(node, "click", func(this js.Value, _ []js.Value) any {
			rowID := this.Get("dataset").Get("removePlatform").String()
			if rowID == "" {
				return nil
			}
			if len(state.Submit.Platforms) == 1 {
				state.Submit.Platforms = []model.PlatformFormRow{NewPlatformRow()}
				state.Submit.Errors.Platforms = make(map[string]model.PlatformFieldError)
			} else {
				next := make([]model.PlatformFormRow, 0, len(state.Submit.Platforms)-1)
				for _, row := range state.Submit.Platforms {
					if row.ID != rowID {
						next = append(next, row)
					}
				}
				state.Submit.Platforms = next
			}
			if state.Submit.Errors.Platforms != nil {
				delete(state.Submit.Errors.Platforms, rowID)
			}
			RenderSubmitForm()
			return nil
		})
	})

	addButton := formDocument().Call("getElementById", "add-platform")
	addFormHandler(addButton, "click", func(js.Value, []js.Value) any {
		if len(state.Submit.Platforms) >= model.MaxPlatforms {
			return nil
		}
		state.Submit.Platforms = append(state.Submit.Platforms, NewPlatformRow())
		RenderSubmitForm()
		return nil
	})

	cancelBtn := formDocument().Call("getElementById", "submit-cancel")
	addFormHandler(cancelBtn, "click", func(js.Value, []js.Value) any {
		ResetFormState(true)
		state.Submit.Open = false
		RenderSubmitForm()
		return nil
	})

	form := formDocument().Call("getElementById", "submit-streamer-form")
	addFormHandler(form, "submit", func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			args[0].Call("preventDefault")
		}
		handleSubmit()
		return nil
	})
}

func addFormHandler(node js.Value, event string, handler func(js.Value, []js.Value) any) {
	if !node.Truthy() {
		return
	}
	fn := js.FuncOf(handler)
	node.Call("addEventListener", event, fn)
	formHandlers = append(formHandlers, fn)
}

func releaseFormHandlers() {
	for _, fn := range formHandlers {
		fn.Release()
	}
	formHandlers = formHandlers[:0]
}

func forEachNode(list js.Value, fn func(js.Value)) {
	if !list.Truthy() {
		return
	}
	length := list.Get("length").Int()
	for i := 0; i < length; i++ {
		fn(list.Index(i))
	}
}

func markFieldError(fieldID string, hasError bool) {
	field := formDocument().Call("getElementById", fieldID)
	if !field.Truthy() {
		return
	}
	classList := field.Get("classList")
	if hasError {
		classList.Call("add", "form-field-error")
	} else {
		classList.Call("remove", "form-field-error")
	}
}

func clearPlatformError(rowID, field string) {
	if state.Submit.Errors.Platforms == nil {
		state.Submit.Errors.Platforms = make(map[string]model.PlatformFieldError)
	}
	platformErr, ok := state.Submit.Errors.Platforms[rowID]
	if !ok {
		return
	}
	switch field {
	case "channel":
		platformErr.Channel = false
	}
	if !platformErr.Channel {
		delete(state.Submit.Errors.Platforms, rowID)
	} else {
		state.Submit.Errors.Platforms[rowID] = platformErr
	}
	markFieldError("platform-url-field-"+rowID, platformErr.Channel)
}

func handleSubmit() {
	if state.Submit.Submitting {
		return
	}

	valid := validateSubmission()
	if !valid {
		RenderSubmitForm()
		return
	}

	trimmedName := strings.TrimSpace(state.Submit.Name)
	trimmedDescription := strings.TrimSpace(state.Submit.Description)
	description := BuildStreamerDescription(trimmedDescription, state.Submit.Platforms)
	payload := model.CreateStreamerRequest{
		Streamer: model.StreamerPayload{
			Alias:       trimmedName,
			Description: description,
			Languages:   append([]string(nil), state.Submit.Languages...),
		},
	}

	if url := FirstPlatformURL(state.Submit.Platforms); url != "" {
		payload.Platforms.URL = url
	}

	state.Submit.Submitting = true
	state.Submit.ResultState = ""
	state.Submit.ResultMessage = ""
	RenderSubmitForm()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		message, err := submitStreamerRequest(ctx, payload)
		if err != nil {
			state.Submit.Submitting = false
			state.Submit.ResultState = "error"
			state.Submit.ResultMessage = err.Error()
			RenderSubmitForm()
			return
		}

		state.Submit.Submitting = false
		state.Submit.ResultState = "success"
		if message == "" {
			message = "Submission received and queued for review."
		}
		state.Submit.ResultMessage = message
		ClearFormFields()
		RenderSubmitForm()
	}()
}

func fetchChannelMetadataAsync(rowIndex int, target string) {
	go func(target string, idx int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		desc, title, handle, channelID, err := requestChannelDescription(ctx, target)
		if err != nil {
			return
		}
		metaDesc := strings.TrimSpace(desc)
		metaTitle := strings.TrimSpace(title)
		metaHandle := strings.TrimSpace(handle)
		metaChannelID := strings.TrimSpace(channelID)
		updated := false
		row := &state.Submit.Platforms[idx]
		if metaDesc != "" && strings.TrimSpace(state.Submit.Description) == "" {
			state.Submit.Description = metaDesc
			state.Submit.Errors.Description = false
			updated = true
		}
		if metaTitle != "" && strings.TrimSpace(state.Submit.Name) == "" {
			state.Submit.Name = metaTitle
			state.Submit.Errors.Name = false
			updated = true
		}
		hasHandleInput := extractHandle(row.Handle) != "" || extractHandle(row.ChannelURL) != ""
		if metaHandle != "" && hasHandleInput {
			row.Handle = metaHandle
			updated = true
		}
		if metaChannelID != "" {
			row.ChannelID = metaChannelID
			updated = true
		}
		if row.HubSecret == "" {
			row.HubSecret = GenerateHubSecret()
			updated = true
		}
		if updated {
			scheduleRender()
		}
	}(target, rowIndex)
}

func submitStreamerRequest(ctx context.Context, payload model.CreateStreamerRequest) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/streamers", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
			return "", errors.New(trimmed)
		}
		return "", fmt.Errorf("submission failed: %s", resp.Status)
	}

	var created model.CreateStreamerResponse
	if err := json.Unmarshal(body, &created); err != nil {
		return "Streamer submitted successfully.", nil
	}
	alias := strings.TrimSpace(created.Streamer.Alias)
	id := strings.TrimSpace(created.Streamer.ID)
	switch {
	case alias != "" && id != "":
		return fmt.Sprintf("%s added with ID %s.", alias, id), nil
	case alias != "":
		return fmt.Sprintf("%s added to the roster.", alias), nil
	default:
		return "Streamer submitted successfully.", nil
	}
}

func requestChannelDescription(ctx context.Context, target string) (string, string, string, string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", "", "", errors.New("empty url")
	}
	reqPayload, err := json.Marshal(model.MetadataRequest{URL: target})
	if err != nil {
		return "", "", "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/youtube/metadata", bytes.NewReader(reqPayload))
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", "", fmt.Errorf(strings.TrimSpace(string(body)))
	}

	var meta model.MetadataResponse
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", "", "", "", err
	}
	return meta.Description, meta.Title, meta.Handle, meta.ChannelID, nil
}
