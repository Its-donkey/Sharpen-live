package forms

import (
	"fmt"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
	"strings"
	// ValidateSubmitForm checks the provided form state for required fields and returns
	// a populated error map. Callers are responsible for storing the errors on the
	// state if needed.
)

func ValidateSubmitForm(form *model.SubmitFormState) model.SubmitFormErrors {
	errors := model.SubmitFormErrors{
		Platforms: make(map[string]model.PlatformFieldError),
	}
	if form == nil {
		errors.Name = true
		errors.Description = true
		errors.Languages = true
		return errors
	}

	if strings.TrimSpace(form.Name) == "" {
		errors.Name = true
	}
	if strings.TrimSpace(form.Description) == "" {
		errors.Description = true
	}
	if len(form.Languages) == 0 {
		errors.Languages = true
	}

	for idx, row := range form.Platforms {
		rowErr := model.PlatformFieldError{
			Channel: strings.TrimSpace(row.ChannelURL) == "",
		}
		if rowErr.Channel {
			key := row.ID
			if key == "" {
				key = fmt.Sprintf("row-%d", idx)
			}
			errors.Platforms[key] = rowErr
		}
	}

	return errors
}

func validateSubmission() bool {
	errors := ValidateSubmitForm(&state.Submit)
	state.Submit.Errors = errors
	return !(errors.Name || errors.Description || errors.Languages || len(errors.Platforms) > 0)
}
