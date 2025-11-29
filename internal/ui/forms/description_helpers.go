package forms

import (
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"strings"
)

func BuildStreamerDescription(desc string, _ []model.PlatformFormRow) string {
	return strings.TrimSpace(desc)
}
