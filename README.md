# Sharpen.Live (alerts + UI)
This repository houses both the alert server and the Sharpen.Live WebAssembly/UI frontend under a single Go module.

## Layout
- `cmd/alertserver`: alert API/WebSub service.
- `cmd/ui-server`: serves the UI (SSR) and static assets.
- `cmd/ui-wasm`: WASM entrypoint that builds `ui/main.wasm`.
- `internal/alert`: alertserver domain logic, handlers, and platform clients.
- `internal/ui`: shared UI logic (forms, admin console, roster mapping, WASM helpers).
- `ui/`: static assets (`index.html`, `styles.css`, templates, `wasm_exec.js`, generated `main.wasm`).

## One command dev stack
Build the WASM bundle and run both servers:
```bash
go run .
```

## Quick start (manual)
1) Run the alert API:
```bash
go run ./cmd/alertserver
```
2) Serve the UI:
```bash
go run ./cmd/ui-server -templates ui/templates -assets ui -api http://127.0.0.1:8880 -listen 127.0.0.1:4173
```
3) Build the WASM bundle when needed:
```bash
GOOS=js GOARCH=wasm go build -o ui/main.wasm ./cmd/ui-wasm
```

For detailed docs see `README.alertserver.md` and `README.ui.md`.
