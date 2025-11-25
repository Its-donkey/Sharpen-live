# Sharpen.Live
Alerts + WebAssembly UI in a single Go module. This README now covers both the alert server and the UI.

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

## Requirements
- Go 1.21+
- (Optional) `make` for your own helper scripts

## Continuous integration
`go fmt` (as lint), `go vet ./...`, and `go test ./...` run in CI. Match that locally:
```bash
gofmt -w .
go vet ./...
go test ./...
```

## Alert server
- **Run**: `go run ./cmd/alertserver`
- **Config**: `config.json` supports `admin`, `server`, and `youtube` blocks (hub URL, callback, leaseSeconds, verify mode). Flags/env vars override file values.
- **Data**: Streamers stored in `data/streamers.json` by default. Submissions in `data/submissions.json`.
- **YouTube leases**: Background monitor renews WebSub leases when ~5% of the window remains.
- **Admin auth**: `/api/admin/login` issues bearer tokens; set creds + `token_ttl_seconds` under `admin` in `config.json`.

## UI (WASM + SSR)
- **Build WASM**: `GOOS=js GOARCH=wasm go build -o ui/main.wasm ./cmd/ui-wasm`
- **Serve locally**: `go run ./cmd/ui-server -templates ui/templates -assets ui -listen 127.0.0.1:4173 -api http://127.0.0.1:8880`
- **Build tags**: Browser code uses `//go:build js && wasm`; host-only helpers use `//go:build !js && !wasm`.
- **Runtime**: `ui-server` hosts static assets and proxies `/api/*` to the alert server. The browser bundle renders the roster + admin console using shared `internal/ui/model` types.

## API reference (alertserver)
All routes live in `internal/api/v1/router.go`. Keep this table in sync.

| Method | Path                          | Description |
| ------ | ----------------------------- | ----------- |
| GET    | `/alerts`                     | WebSub hub verification. |
| POST   | `/api/youtube/subscribe`      | Proxy subscribe with enforced defaults. |
| POST   | `/api/youtube/unsubscribe`    | Proxy unsubscribe. |
| POST   | `/api/youtube/channel`        | Resolve `@handle` to channel ID. |
| GET    | `/api/streamers`              | Return all streamers. |
| GET    | `/api/streamers/watch`        | SSE for `streamers.json` changes. |
| POST   | `/api/streamers`              | Queue a streamer submission. |
| PATCH  | `/api/streamers`              | Update alias/description/languages. |
| DELETE | `/api/streamers`              | Remove a streamer. |
| POST   | `/api/youtube/metadata`       | Scrape a URL and return meta description/title. |
| GET    | `/api/server/config`          | UI runtime config. |
| POST   | `/api/admin/login`            | Issue admin bearer token. |
| GET    | `/api/admin/submissions`      | List pending submissions. |
| POST   | `/api/admin/submissions`      | Approve/reject a submission. |
| POST   | `/api/admin/streamers/status` | Refresh and record online status. |
| GET    | `/api/admin/monitor/youtube`  | Summarize YouTube lease status. |

## Notes
- Static assets (`ui/`) can be hosted separately; point them at `/api/server/config` for API base/paths.
- `cmd/ui-server` and `cmd/alertserver` share the same module—no extra checkout needed.
