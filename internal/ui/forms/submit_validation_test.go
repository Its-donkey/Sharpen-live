package forms

import (
	"testing"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func TestValidateSubmitFormNil(t *testing.T) {
	errs := ValidateSubmitForm(nil)
	if !errs.Name || !errs.Description || !errs.Languages {
		t.Fatalf("expected required fields to be flagged, got %+v", errs)
	}
}

func TestValidateSubmitFormPlatforms(t *testing.T) {
	state := &model.SubmitFormState{
		Name:        "Name",
		Description: "Desc",
		Languages:   []string{"English"},
		Platforms: []model.PlatformFormRow{
			{ID: "row-1", ChannelURL: ""},
			{ID: "row-2", ChannelURL: "https://example.com"},
		},
	}
	errs := ValidateSubmitForm(state)
	if errs.Name || errs.Description || errs.Languages {
		t.Fatalf("expected no core field errors, got %+v", errs)
	}
	if _, ok := errs.Platforms["row-1"]; !ok {
		t.Fatalf("expected platform error for empty URL")
	}
	if len(errs.Platforms) != 1 {
		t.Fatalf("expected only one platform error, got %+v", errs.Platforms)
	}
}
