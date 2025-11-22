//go:build js && wasm

package main

import "github.com/Its-donkey/Sharpen-live/internal/ui/wasm"

func main() {
	wasm.RunApp()
}
