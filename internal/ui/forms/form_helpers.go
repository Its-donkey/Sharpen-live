package forms

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

// BuildStreamerDescription trims and normalizes the streamer description gathered from the form.
func BuildStreamerDescription(description string, _ []model.PlatformFormRow) string {
	return strings.TrimSpace(description)
}

// DerivePlatformLabel extracts a short, human-friendly label from a platform URL or handle.
func DerivePlatformLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "@") {
		return raw
	}
	if parsed, err := url.Parse(raw); err == nil {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(segments) > 0 && strings.HasPrefix(segments[0], "@") {
			return segments[0]
		}
		if host := parsed.Hostname(); host != "" {
			return host
		}
	}
	return raw
}

// CanonicalizeChannelInput converts user-entered platform identifiers into canonical URLs.
func CanonicalizeChannelInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return trimmed
	}
	if handle := extractHandle(trimmed); handle != "" {
		return buildURLFromHandle(handle, "youtube")
	}
	if strings.HasPrefix(lower, "youtube.com/") || strings.HasPrefix(lower, "www.youtube.com/") || strings.HasPrefix(lower, "m.youtube.com/") {
		return "https://" + trimmed
	}
	if strings.HasPrefix(lower, "youtu.be/") {
		return "https://" + trimmed
	}
	return trimmed
}

// FirstPlatformURL returns the first non-empty channel URL from the provided rows.
func FirstPlatformURL(rows []model.PlatformFormRow) string {
	for _, row := range rows {
		if trimmed := strings.TrimSpace(row.ChannelURL); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractHandle(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "@") && len(trimmed) > 1 {
		return trimmed
	}
	return ""
}

func resolvePlatformPreset(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "twitch":
		return "twitch"
	case "facebook":
		return "facebook"
	default:
		return "youtube"
	}
}

func buildURLFromHandle(handle, preset string) string {
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle == "" {
		return ""
	}
	switch resolvePlatformPreset(preset) {
	case "twitch":
		return "https://www.twitch.tv/" + handle
	case "facebook":
		return "https://www.facebook.com/" + handle
	default:
		return "https://www.youtube.com/@" + handle
	}
}

// GenerateHubSecret produces a random hub secret for PubSub subscriptions.
func GenerateHubSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("hub-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ClearFormFields resets the submission state to a single blank platform row.
func ClearFormFields() {
	state.Submit.Name = ""
	state.Submit.Description = ""
	state.Submit.Languages = nil
	state.Submit.Platforms = []model.PlatformFormRow{NewPlatformRow()}
	state.Submit.Errors = model.SubmitFormErrors{
		Platforms: make(map[string]model.PlatformFieldError),
	}
}

// ResetFormState clears the form and optionally wipes the result message.
func ResetFormState(includeResult bool) {
	ClearFormFields()
	state.Submit.Submitting = false
	if includeResult {
		state.Submit.ResultMessage = ""
		state.Submit.ResultState = ""
	}
}

// NewPlatformRow creates a uniquely identified platform row for the submission forms.
func NewPlatformRow() model.PlatformFormRow {
	return model.PlatformFormRow{
		ID:         fmt.Sprintf("platform-%d", time.Now().UnixNano()),
		Name:       "",
		Preset:     "",
		ChannelURL: "",
	}
}

// AvailableLanguageOptions filters out already-selected languages from the picker options.
func AvailableLanguageOptions(selected []string) []model.LanguageOption {
	options := make([]model.LanguageOption, 0, len(model.LanguageOptions))
	selectedSet := make(map[string]struct{}, len(selected))
	for _, value := range selected {
		selectedSet[value] = struct{}{}
	}
	for _, option := range model.LanguageOptions {
		if _, exists := selectedSet[option.Value]; !exists {
			options = append(options, option)
		}
	}
	return options
}

// ContainsString reports whether the provided slice already contains the target value.
func ContainsString(list []string, target string) bool {
	for _, entry := range list {
		if entry == target {
			return true
		}
	}
	return false
}

var languageLabelByValue = func() map[string]string {
	values := make(map[string]string, len(model.LanguageOptions))
	for _, option := range model.LanguageOptions {
		values[option.Value] = option.Label
	}
	return values
}()

// DisplayLanguage returns the user-facing label for the provided language value.
func DisplayLanguage(value string) string {
	if label := languageLabelByValue[value]; label != "" {
		return label
	}
	return value
}
