package state

import (
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

var (
	// Submit holds the reactive state for the public submission form.
	Submit model.SubmitFormState

	// AdminConsole stores the long-lived admin UI view state for the WASM app.
	AdminConsole = model.AdminViewState{
		ActiveTab:     "streamers",
		StreamerForms: make(map[string]*model.AdminStreamerForm),
		YouTubeLeases: make(map[string]model.YouTubeLeaseStatus),
	}
)
