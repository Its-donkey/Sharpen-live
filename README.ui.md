# Sharpen.Live UI
Sharpen.Live UI is the WebAssembly dashboard for the alert server. It renders the public roster and admin console while calling the alert API for data and submissions.

## Project layout
- `cmd/ui-wasm`: browser entrypoint that compiles to the WASM bundle.
- `cmd/ui-serve`: static asset server and API/SSE proxy for local development.
- `cmd/ui-server`: server-rendered HTML variant sharing the same domain logic.
- `internal/ui/...`: shared code for admin console, forms, roster mapping, and view state.
- `ui/`: static assets (`index.html`, `styles.css`, templates, `wasm_exec.js`, generated `main.wasm`).

## One command dev stack
Build the WASM bundle and run both UI + alertserver together:
```bash
go run .
```

## Building
```bash
GOOS=js GOARCH=wasm go build -o ui/main.wasm ./cmd/ui-wasm
```
The resulting `main.wasm`, `index.html`, `styles.css`, and `wasm_exec.js` bundle can be hosted by any static file server. Point it at `/` so the UI can call the alert server's `/api/server/config` endpoint.

## Local development server
Run the alert server first:
```bash
go run ./cmd/alertserver
```
Then serve the UI bundle (from the repo root):
```bash
go run ./cmd/ui-serve -dir ui -listen 127.0.0.1:4173 -api http://127.0.0.1:8880
```
For server-rendered HTML, use `go run ./cmd/ui-server -templates ui/templates -assets ui`.

## Build tags & architecture
Files that run in the browser begin with `//go:build js && wasm` (for example everything under `internal/ui/wasm`, `internal/ui/forms`, and `internal/ui/admin`). Host-only helpers such as `cmd/ui-serve` and `internal/ui/wasm/wasm_stub.go` use `//go:build !js && !wasm` so `GOOS=js GOARCH=wasm` builds only the UI logic while standard builds include the static-file server and helpers.

At runtime:
- `cmd/ui-serve` hosts static assets and proxies `/api/*` calls to the alert server.
- The browser bundle (`main.wasm`, `index.html`, `styles.css`) renders the public roster and admin console, talking to the alert server via the proxy.
- Admin handlers reuse the same data structures defined in `internal/ui/model/types.go` so the UI and server stay in sync.

## Testing, linting, and CI expectations
```bash
gofmt -w .
go vet ./...
go test ./...
```
These checks keep the WASM and native builds consistent and catch regressions in form helpers, mapper logic, and admin workflows.
