# Sharpen.Live
Single Go server for Sharpen.Live (alerts, roster, submissions, and server-rendered admin UI).

## Layout
- `cmd/alertserver`: single server for UI, submissions, admin, YouTube WebSub, and JSON endpoints.
- `internal/alert`: alert/YouTube domain logic, handlers, and platform clients.
- `internal/ui`: shared UI logic (forms, roster mapping, helpers).
- `ui/`: static assets (`styles.css`, templates, JS helpers).

## Quick start (dev)
Run the UI + alerts server:
```bash
go run ./cmd/alertserver \
  -templates ui/templates \
  -assets ui \
  -listen 127.0.0.1:4173 \
  -config config.json
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

## Server
- **Run**: `go run ./cmd/alertserver -config config.json`
- **Config**: `config.json` supports `admin`, `server`, `app`, and `youtube` blocks (hub URL, callback, leaseSeconds, verify mode, `api_key`). Listen/templates/assets/log/data paths default from the `app` block unless overridden by flags. Keep `youtube.api_key` in env/secret overrides; the sample uses a placeholder.
- **Data**: Streamers stored in `data/streamers.json` by default (configurable via `app.data`). Submissions in `data/submissions.json`.
- **YouTube leases**: Background monitor renews WebSub leases when ~5% of the window remains; `/alerts` handles WebSub callbacks.
- **Admin auth**: server-rendered `/admin` login uses credentials under `admin` in `config.json`.

## UI (SSR only)
- **Serve locally**: `go run ./cmd/alertserver -templates ui/templates -assets ui -listen 127.0.0.1:4173 -config config.json`
- **Runtime**: the server hosts static assets and serves roster/submit/admin endpoints directly (no API proxy). The admin console is server-rendered at `/admin` using credentials from `config.json`.

## Endpoints (served by ui-server)
- `/` roster and submission form (SSR)
- `/submit` public submission POST
- `/streamers/watch` SSE change feed (timestamp when `data/streamers.json` changes); `/api/streamers/watch` is an alias for legacy clients
- `/api/youtube/metadata` metadata enrichment for submissions
- `/alerts` YouTube WebSub verification/notifications
- `/admin` server-rendered admin dashboard (login + moderation)

## Notes
- Static assets (`ui/`) can be hosted separately if they hit this server for `/api/*` and `/alerts`.

## Deploy (Linux)
- Build: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/alertserver ./cmd/alertserver`
- Ship to server: `dist/alertserver`, `config.json`, `ui/` (templates + assets), and `data/` (includes `streamers.json`; keep writable for submissions/logs).
- Run from that directory (so relative paths work):  
  `./dist/alertserver -listen 0.0.0.0:4173 -templates ui/templates -assets ui -config config.json`
- Ensure `/alerts` is reachable publicly at the `youtube.callback_url` in `config.json` (set it to your HTTPS domain + `/alerts`). Put TLS/reverse proxy (nginx/Caddy/Traefik) in front as needed.
- Logs are written to `data/logs/{http.json,general.json}` on each start; keep the directory writable.
