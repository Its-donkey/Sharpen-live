# Architecture Overview

This document explains how the alert server is structured so new contributors can reason about responsibilities without re-reading the entire codebase.

## Layered flow

```
cmd/alertserver ➜ internal/ui/server ➜ services ➜ stores/platform clients
```

1. **`cmd/alertserver`** wires CLI flags/env vars (templates, assets, listen addr, config path) and delegates to `internal/ui/server`.
2. **`internal/ui/server`** loads configuration, builds dependencies (streamers/submissions stores, admin + streamer services, YouTube lease monitor), registers HTTP routes/templates/assets, and manages process lifecycle with contexts.
3. **Services** (for streamers, admin, YouTube channel/metadata/subscription/alert flows) encapsulate business rules and call downstream dependencies via small interfaces so tests can mock them.
4. **Stores/platform clients** are the only layers allowed to touch disk or make outbound HTTP requests. Stores hide file locking/encoding; platform clients keep PubSubHubbub and YouTube parsing contained.

## Key packages

| Package | Responsibility |
| --- | --- |
| `cmd/alertserver` / `internal/ui/server` | Entry point + HTTP host for SSR UI, admin flows, WebSub callbacks, and SSE watch endpoint. |
| `internal/alert/streamers/service` | Streamer CRUD + submissions queueing. |
| `internal/alert/platforms/youtube/service` | Channel lookup, metadata scraping, subscription proxying, WebSub alert processing. |
| `internal/alert/platforms/youtube/subscriptions` | PubSubHubbub client, lease monitor, renewal helpers. |
| `internal/alert/admin/service` | Auth + submission approval flows. |
| `internal/alert/streamers` & `internal/alert/submissions` | File-backed stores with per-path mutexes. |

## Background workers

- **Lease monitor**: `internal/alert/platforms/youtube/subscriptions.LeaseMonitor` watches stored YouTube records and silently renews subscriptions 5% before expiration. The UI server owns its lifecycle via `StartLeaseMonitor/Stop`.
- **Streamers watch SSE**: `internal/ui/server.streamersWatchHandler` polls `streamers.json` and streams change notifications to clients. The poller is scoped to the HTTP handler request context so it automatically stops when clients disconnect.

## Configuration surfaces

- `config/config.go` loads `config.json`, merging `server`, `youtube`, and `admin` blocks with CLI/env overrides.
- Flags/env vars are declared in `cmd/alertserver/main.go` and passed into `internal/ui/server.Options`. The server builds defaults for stores/services when none are injected, but tests and tools can swap in fakes (stores, services, templates, metadata fetcher, lease monitor factory) for deterministic behaviour.

## Testing philosophy

- Services accept interfaces (stores, HTTP clients, clock/ID generators) so tests can inject determinism.
- Table-driven tests cover validation edge cases (`internal/streamers/service`, `internal/platforms/youtube/service`, stores).
- Handler tests mock services/processors to assert HTTP behavior separately from core logic.
- UI server tests inject fakes via `internal/ui/server.Options` to exercise handlers without touching disk/network. Templates and lease monitors are injectable, keeping background goroutines out of tests.

## Adding new features

1. Decide whether the change belongs in a service or store. Handlers should only parse HTTP inputs and forward to services.
2. Update/add services/interfaces if new business logic is required, keeping dependencies injectable.
3. Document new routes in `README.md` and extend this architecture doc if a significant new subsystem is introduced.
4. Ensure `gofmt`, `go vet`, and `go test ./...` pass locally—the CI workflow enforces all three.
