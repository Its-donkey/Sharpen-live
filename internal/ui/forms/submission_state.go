package forms

import (
	"fmt"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
	"time"
)

func ClearFormFields() {
	state.Submit.Name = ""
	state.Submit.Description = ""
	state.Submit.Languages = []string{"English"}
	state.Submit.Platforms = []model.PlatformFormRow{NewPlatformRow()}
	state.Submit.Errors = model.SubmitFormErrors{Platforms: make(map[string]model.PlatformFieldError)}
}

func ResetFormState(includeResult bool) {
	ClearFormFields()
	state.Submit.Submitting = false
	if includeResult {
		state.Submit.ResultMessage = ""
		state.Submit.ResultState = ""
	}
}

func NewPlatformRow() model.PlatformFormRow {
	return model.PlatformFormRow{
		ID:   fmt.Sprintf("platform-%d", time.Now().UnixNano()),
		Name: "", Preset: "", ChannelURL: "",
	}
}

func validateSubmission() bool {
	errs := ValidateSubmitForm(&state.Submit)
	state.Submit.Errors = errs
	return !(errs.Name || errs.Description || errs.Languages || len(errs.Platforms) > 0)
}
