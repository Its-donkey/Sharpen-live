//go:build js && wasm

package wasm

import (
	"strings"
	"syscall/js"

	"github.com/Its-donkey/Sharpen-live/internal/ui/admin"
)

// RunApp bootstraps the Sharpen Live WASM UI and blocks forever.
func RunApp() {
	done := make(chan struct{})
	window := js.Global()
	Document = window.Get("document")
	pathname := window.Get("location").Get("pathname").String()
	adminOnlyMode = strings.HasSuffix(pathname, "/admin") || strings.HasSuffix(pathname, "/admin/")

	initAdminState()
	if adminOnlyMode {
		buildAdminShell()
	} else {
		buildShell()
	}
	initSubmitForm()
	initRosterRefresh()
	if !adminOnlyMode {
		startStreamersWatch()
		go refreshRoster()
	} else {
		admin.RenderAdminConsole()
	}
	<-done
}

func buildShell() {
	root := Document.Call("getElementById", "app-root")
	if !root.Truthy() {
		js.Global().Get("console").Call("error", "app root missing")
		return
	}

	root.Set("innerHTML", mainLayout())
	streamerTable = Document.Call("getElementById", "streamer-rows")
	admin.RenderAdminConsole()
}

func buildAdminShell() {
	root := Document.Call("getElementById", "app-root")
	if !root.Truthy() {
		js.Global().Get("console").Call("error", "app root missing")
		return
	}
	root.Set("innerHTML", adminOnlyLayout())
	admin.RenderAdminConsole()
}
