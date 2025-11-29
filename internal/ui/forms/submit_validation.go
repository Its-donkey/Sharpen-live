package forms

import (
	"fmt"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"strings"
)

func ValidateSubmitForm(form *model.SubmitFormState) model.SubmitFormErrors {
	errs := model.SubmitFormErrors{Platforms: make(map[string]model.PlatformFieldError)}
	if form == nil {
		errs.Name, errs.Description, errs.Languages = true, true, true
		return errs
	}
	if strings.TrimSpace(form.Name) == "" {
		errs.Name = true
	}
	if strings.TrimSpace(form.Description) == "" {
		errs.Description = true
	}
	if len(form.Languages) == 0 {
		errs.Languages = true
	}
	for idx, row := range form.Platforms {
		if strings.TrimSpace(row.ChannelURL) == "" {
			key := row.ID
			if key == "" {
				key = fmt.Sprintf("row-%d", idx)
			}
			errs.Platforms[key] = model.PlatformFieldError{Channel: true}
		}
	}
	return errs
}
