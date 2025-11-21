//go:build js && wasm

package admin

import (
	"strings"
	"syscall/js"

	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

const adminTokenStorageKey = "sharpen-live-admin-token"

var (
	adminState    = &state.AdminConsole
	adminDocument js.Value
)

func getDocument() js.Value {
	if !adminDocument.Truthy() {
		adminDocument = js.Global().Get("document")
	}
	return adminDocument
}

func persistAdminToken(token string) {
	storage := js.Global().Get("localStorage")
	if !storage.Truthy() {
		return
	}
	if strings.TrimSpace(token) == "" {
		storage.Call("removeItem", adminTokenStorageKey)
		return
	}
	storage.Call("setItem", adminTokenStorageKey, token)
}
