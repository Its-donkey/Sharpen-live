//go:build js && wasm

package wasm

import (
	"context"
	"strings"
	"syscall/js"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/ui/streamers"
)

var (
	// Document references the global browser document for DOM interactions.
	Document      js.Value
	streamerTable js.Value
	// RefreshFunc triggers a roster refresh and is reused by retry buttons.
	RefreshFunc js.Func
	// FormHandlers stores bound js.Func callbacks so they can be released later.
	FormHandlers         []js.Func
	streamersWatchSource js.Value
	streamersWatchFuncs  []js.Func
	adminHandlers        []js.Func
	adminOnlyMode        bool
)

// AdminTokenStorageKey identifies the localStorage entry for cached admin tokens.
const AdminTokenStorageKey = "sharpen-live-admin-token"

func initAdminState() {
	if strings.TrimSpace(state.AdminConsole.ActiveTab) == "" {
		state.AdminConsole.ActiveTab = "streamers"
	}
	if strings.TrimSpace(state.AdminConsole.ActivityTab) == "" {
		state.AdminConsole.ActivityTab = "website"
	}
	if state.AdminConsole.StreamerForms == nil {
		state.AdminConsole.StreamerForms = make(map[string]*model.AdminStreamerForm)
	}
	state.AdminConsole.Token = loadStoredAdminToken()
}

func loadStoredAdminToken() string {
	storage := js.Global().Get("localStorage")
	if !storage.Truthy() {
		return ""
	}
	value := storage.Call("getItem", AdminTokenStorageKey)
	if value.Type() == js.TypeString {
		return strings.TrimSpace(value.String())
	}
	return ""
}

func initSubmitForm() {
	state.Submit = model.SubmitFormState{
		Platforms: []model.PlatformFormRow{forms.NewPlatformRow()},
		Errors: model.SubmitFormErrors{
			Platforms: make(map[string]model.PlatformFieldError),
		},
	}
	forms.RenderSubmitForm()
}

func initRosterRefresh() {
	if RefreshFunc.Type() != js.TypeUndefined {
		RefreshFunc.Release()
	}
	RefreshFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
		go refreshRoster()
		return nil
	})
}

func startStreamersWatch() {
	window := js.Global()
	ctor := window.Get("EventSource")
	if ctor.Type() == js.TypeUndefined || !ctor.Truthy() {
		return
	}
	cleanupStreamersWatch()
	paths := []string{"/api/streamers/watch", "/streamers/watch"}
	TryStreamersWatch(ctor, paths)
}

func cleanupStreamersWatch() {
	if streamersWatchSource.Truthy() {
		streamersWatchSource.Call("close")
	}
	for _, fn := range streamersWatchFuncs {
		fn.Release()
	}
	streamersWatchFuncs = nil
	streamersWatchSource = js.Value{}
}

func refreshRoster() {
	setStatusRow("Loading streamer roster…", false)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	streamers, err := streamersvc.FetchStreamers(ctx)
	if err != nil {
		console := js.Global().Get("console")
		if console.Truthy() {
			console.Call("warn", "failed to load streamers, using fallback data", err.Error())
		}
	}

	renderStreamers(streamers)
}
