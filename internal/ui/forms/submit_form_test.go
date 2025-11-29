package forms

import (
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
	"testing"
)

func resetSubmitState() {
	state.Submit = model.SubmitFormState{
		Platforms: []model.PlatformFormRow{{ID: "p-1", ChannelURL: ""}},
		Errors: model.SubmitFormErrors{
			Platforms: make(map[string]model.PlatformFieldError),
		},
	}
}

func TestValidateSubmissionDetectsErrors(t *testing.T) {
	resetSubmitState()
	state.Submit.Platforms[0].ChannelURL = ""
	if ok := validateSubmission(); ok {
		t.Fatal("expected validation to fail")
	}
	if !state.Submit.Errors.Name || !state.Submit.Errors.Description || !state.Submit.Errors.Languages {
		t.Fatalf("expected name/description/language errors: %#v", state.Submit.Errors)
	}
	if len(state.Submit.Errors.Platforms) != 1 {
		t.Fatalf("expected platform error to be recorded: %#v", state.Submit.Errors.Platforms)
	}
}

func TestValidateSubmissionPassesWithCompleteData(t *testing.T) {
	resetSubmitState()
	state.Submit.Name = "Streamer"
	state.Submit.Description = "Sharpening blades"
	state.Submit.Languages = []string{"English"}
	state.Submit.Platforms[0].ChannelURL = "https://example.com"

	if ok := validateSubmission(); !ok {
		t.Fatalf("expected validation to succeed: %#v", state.Submit.Errors)
	}
	if len(state.Submit.Errors.Platforms) != 0 {
		t.Fatalf("unexpected platform errors: %#v", state.Submit.Errors.Platforms)
	}
	if state.Submit.Errors.Name || state.Submit.Errors.Description || state.Submit.Errors.Languages {
		t.Fatalf("unexpected form errors: %#v", state.Submit.Errors)
	}
}
