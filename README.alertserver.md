# Sharpen.Live alert server

A lightweight Go service that proxies YouTube WebSub subscriptions, stores streamer metadata, and exposes operational endpoints for downstream automation. The companion WebAssembly UI lives in this repository under `ui/` and is served by the `cmd/ui-*` entrypoints so you can run both pieces from the same module while still deploying them as separate binaries.

## Requirements
- Go 1.21+
- (Optional) `make` for your own helper scripts

## Continuous integration
Every push/PR triggers `.github/workflows/ci.yml`, which runs `gofmt` (as a lint check), `go vet ./...`, and `go test ./...`. Run those locally before opening a PR to avoid CI failures:

```bash
gofmt -w .
go vet ./...
go test ./...
```

## Architecture
See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the layered overview (cmd ➜ app ➜ router ➜ services ➜ stores/platform clients), background workers, and testing conventions.

## One command dev stack
Run the alert server, build the UI WASM, and serve the UI together:
```bash
go run .
```

## Running the alert server
1. Start the HTTP server:
   ```bash
   go run ./cmd/alertserver
   ```
2. Streamer data is appended to `data/streamers.json`. Provide a different path through `CreateOptions.FilePath` if you embed the handler elsewhere. Serve the UI with `go run ./cmd/ui-server -assets ui -templates ui/templates ...` so API traffic and UI assets stay decoupled.

## Companion UI (alGUI)
The UI source and assets live in `ui/` with entrypoints under `cmd/ui-*`. Build `ui/main.wasm` via `GOOS=js GOARCH=wasm go build -o ui/main.wasm ./cmd/ui-wasm`, then serve it locally with `go run ./cmd/ui-server -assets ui -templates ui/templates -api http://127.0.0.1:8880` if you prefer server-rendered HTML. Both binaries share the same module so no extra checkout is needed.

## Configuration
The WebSub defaults can be configured via environment variables or CLI flags (flags take precedence):

| Flag | Environment variable | Description | Default |
| ---- | -------------------- | ----------- | ------- |
| `-youtube-hub-url` | `YOUTUBE_HUB_URL` | PubSubHubbub hub endpoint used for subscribe/unsubscribe flows. | `https://pubsubhubbub.appspot.com/subscribe` |
| `-youtube-callback-url` | `YOUTUBE_CALLBACK_URL` | Callback URL that the hub invokes for alert delivery. | `https://sharpen.live/alerts` |
| `-youtube-lease-seconds` | `YOUTUBE_LEASE_SECONDS` | Lease duration requested during subscribe/unsubscribe. | `864000` |
| `-youtube-default-mode` | `YOUTUBE_DEFAULT_MODE` | WebSub mode enforced when omitted (typically `subscribe`). | `subscribe` |
| `-youtube-verify-mode` | `YOUTUBE_VERIFY_MODE` | Verification strategy requested (`sync` or `async`). | `async` |

### `config.json`
The binary also reads `config.json` on startup for file-based overrides. This is the best place to pin the HTTP listener address/port alongside the YouTube defaults:

```json
{
  "admin": {
    "email": "admin@sharpen.live",
    "password": "change-me",
    "token_ttl_seconds": 86400
  },
  "server": {
    "addr": "127.0.0.1",
    "port": ":8880"
  },
  "youtube": {
    "hub_url": "https://pubsubhubbub.appspot.com/subscribe",
    "callback_url": "https://sharpen.live/alerts",
    "lease_seconds": 864000,
    "verify": "async"
  }
}
```

Omit any field to fall back to the defaults above. The legacy top-level keys (`hub_url`, `callback_url`, etc.) are still honored for backward compatibility, but nesting them under `youtube` keeps the file organized.

When `/alerts` receives a push notification, the server fetches the YouTube watch page for the referenced video, inspects its embedded metadata, and automatically updates the matching streamer record’s `status` when the notification corresponds to a live broadcast. No YouTube Data API key is required for this flow.

### YouTube lease monitor
The alert server continuously inspects `data/streamers.json` for YouTube subscriptions and automatically renews them when roughly 5% of the lease window remains. The renewal window is derived from `hubLeaseDate` (last hub confirmation) plus `leaseSeconds`, so keeping those fields current ensures subscriptions are re-upped before the hub expires them.

### Admin authentication
The admin console authenticates via `/api/admin/login`. Configure the allowed credentials in the `admin` block of `config.json`, and adjust `token_ttl_seconds` to control how long issued bearer tokens remain valid. Include the token using an `Authorization: Bearer <token>` header for any admin-only APIs.

## API reference
All HTTP routes are registered in `internal/api/v1/router.go`. Update the table below whenever an endpoint is added or altered so this README remains the single source of truth.

| Method | Path                         | Description |
| ------ | ---------------------------- | ----------- |
| GET    | `/alerts`                    | Responds to YouTube PubSubHubbub verification challenges. |
| POST   | `/api/youtube/subscribe`     | Proxies subscription requests to YouTube's hub after enforcing defaults. |
| POST   | `/api/youtube/unsubscribe`   | Issues unsubscribe calls to YouTube's hub so you stop receiving alerts. |
| POST   | `/api/youtube/channel`       | Resolves a YouTube `@handle` into its canonical channel ID. |
| GET    | `/api/streamers`             | Returns every stored streamer record. |
| GET    | `/api/streamers/watch`       | Streams server-sent events whenever `streamers.json` changes. |
| POST   | `/api/streamers`             | Queues a streamer submission for admin review (written to `data/submissions.json`). |
| PATCH  | `/api/streamers`             | Updates the alias/description/languages of an existing streamer. |
| DELETE | `/api/streamers`             | Removes a stored streamer record. |
| POST   | `/api/youtube/metadata`      | Scrapes a public URL and returns its meta description/title. |
| GET    | `/api/server/config`         | Returns the server runtime information consumed by the UI. |
| POST   | `/api/admin/login`           | Issues a bearer token for administrative API calls. |
| GET    | `/api/admin/submissions`     | Lists pending streamer submissions for review. |
| POST   | `/api/admin/submissions`     | Approves or rejects a pending submission. |
| GET    | `/api/admin/monitor/youtube` | Summarises YouTube lease status for every stored channel. |
| GET    | `/`                          | Returns placeholder text reminding you to host alGUI separately. |
