# Changelog

## Unreleased

### Added
- Logging: comprehensive structured logging system with JSON formatting, log levels (DEBUG, INFO, WARN, ERROR, FATAL), and real-time log streaming via Server-Sent Events.
- Logging: HTTP middleware that captures all request/response details including method, path, query, status, timing, headers (sensitive headers filtered), and body content (truncated to 1000 chars).
- Logging: YouTube WebSub notification parser that automatically detects and parses Atom XML feeds from YouTube PubSubHubbub callbacks, creating structured logs with video_id, channel_id, video_title, channel_name, and timestamps for each video notification.
- Logging: rotating file writer with automatic gzip compression of old log files and configurable size/retention limits (default 50MB per file, 10 files kept).
- Logging: structured event logging for admin actions (login, logout, submission moderation, streamer updates/deletions) and public submissions with contextual fields.
- Logging: web-based log viewer on default-site at /logs with filtering by level and category, real-time updates via SSE, and expandable JSON fields.
- Logging: pub/sub pattern for real-time log subscribers enabling live log streaming to multiple viewers simultaneously.
- Server/UI: run multiple branded sites from a single config (`-site` targets one; default boot spins up every entry) so Sharpen.Live and synth.wave can host their own templates/assets/log/data roots concurrently.
- UI: add site-specific templates, OG images, neon synthwave styling, and SVG brand assets plus a synth.wave brand guide for design handoff.
- UI: add a default-site fallback that surfaces the errors causing a fallback instead of silently rendering Sharpen.Live defaults.
- Admin: add roster “Check online status” action and API to refresh channel state on demand.
- Submit form: detect @handles, prompt for platform, and expand to full channel URLs.
- Submit form: preselect English and add an "Add another language" button consistent with platform controls.
- UI: add SEO-focused canonical/meta tags, JSON-LD, sitemap/robots.txt endpoints, and an Open Graph preview image so pages are fully server-rendered for search and social.
- Docs: add Go engineering guidelines covering file responsibilities, testing, and logging practices.

### Changed
- Server: consolidate alerts, roster, submissions, and admin into a single `cmd/alertserver` binary (no separate proxy) and host YouTube WebSub callbacks + lease monitor in-process.
- UI: remove WASM bundle/static entrypoints; everything is server-rendered.
- API surface: drop public streamers CRUD/config/admin APIs; only SSE watch, metadata, and `/alerts` remain exposed.
- Tooling: replace Postman collection with current endpoints and update one-command runner to start only the consolidated server.
- Config: default server listen/templates/assets directories now come from `config.json` (see `ui` block) so running without flags picks up file settings.
 - Config: renamed `ui` block to `app` and added `data` so streamers/submissions/templates/assets paths are all configurable from config.json.
- Docs: update layout/run commands to reflect the consolidated server entrypoint.
- Forms: split submit helpers into focused files (state, languages, platforms, secrets, description) to simplify maintenance.
- Config/Docs: capture per-site `app.name` plus site-specific server/assets/data roots in config.json and README so multi-site deployments stay isolated.
- UI: drop legacy root templates/assets in favour of per-site (Sharpen.Live, synth.wave) and default-site directories.
- YouTube API: prefer `YOUTUBE_API_KEY`/`YT_API_KEY` environment values for both config loading and the player client default key instead of the baked-in sample key.
- UI: move YouTube-specific helpers/handlers into `internal/ui/platforms/youtube` and reuse them across forms, streamers, and server wiring for clearer ownership.

### Fixed
- Home page now renders even when roster loading fails, surfacing the error inline instead of crashing the template.
- Submit form template now binds directly to the submit form state and status badges render with the correct helper signature, preventing template execution errors on the home page.
- Metadata: restrict metadata fetches to an allowlist of hosts and normalise URLs before issuing upstream requests to avoid uncontrolled destinations.
- Admin: Refresh Status now falls back to live YouTube watch-page metadata so live streams get written to `data/streamers.json` even when the player API doesn’t flag them.
- Admin: Status checks query the YouTube search API (`eventType=live`) using the configured API key, clearing status when no live items are returned.
- Roster: serve the SSE watch feed at `/api/streamers/watch` as an alias for legacy clients hitting the old API path.
- Submit form: align channel URL rows to the grid and constrain inputs to half-width for consistent layout.
- Submit form: remove padding/border radius from channel URL inputs so they sit flush in the grid.
- Submit form: always show handle platform picker when an @handle is entered, even before metadata loads.
- Submit form: render handle platform picker markup even before a handle is typed so it can be toggled on.
- Submit form: remove padding/border radius on handle platform selects so they render flush.
- Submit form: accept bracketed language fields (languages[]) so submissions validate instead of returning 422.
- Submit form: keep the handle dropdown markup consistent between SSR and WASM to avoid duplicate platform rows.
- Submit form: place the add-language button before the dropdown for clearer horizontal controls.
- Submit form: show selected language tags above the controls for better visibility.
- UI: remove the redundant “Roster” header button from the home view.
- Submit form: hide the form behind an “Add a Streamer” button and reveal it on demand.
- Submit form: fix the “Add a Streamer” toggle so it opens the hidden form as intended.
- Submit form: stretch the lead help text across the full form grid for consistent alignment.
- Submit form: hide the language dropdown until “Add another language” is clicked, then swap back once a choice is made.
- Submit form: correctly parse platform rows/removals so submissions post successfully again.
- UI: expand the home intro copy and span it across the full width to spotlight live sharpening streams.
- Roster: swap YouTube text labels for the YouTube logo on platform links.
- UI: remove the home page “Live Knife Sharpening Studio” heading to keep the intro concise.
- UI: brand home, submit, streamer, and admin page metadata per site so each brand renders the correct title/description.
- Roster: auto-reload browsers when `streamers.json` changes via the watch endpoint.
- Roster: size the YouTube logo pill to match the text-height badges.
- Roster: show platform links only when a streamer is online and disable profile links for now.
- Roster: map live/busy/offline state from stored status so status pills and platform links reflect reality.
- Guard WebSub hub.challenge and reject malformed values to avoid reflected content.
- Harden YouTube WebSub requests by validating hub, topic, and callback URLs.
- Admin: validate admin tokens for settings/log streams instead of accepting any non-empty value.
- Admin: ensure `/admin/logs` streams valid server-sent events so the Activity tab shows live logs again.
- Admin: wrap long log messages/details so the log feed stays within its card layout.
- Admin: remove the inline log viewer from the dashboard to keep the layout focused on submissions and roster.
- Alert server: encode HTTP request/response log payloads as JSON so downstream log streams stay valid.
- Alert server: emit default SSE message events on `/api/streamers/watch` so EventSource listeners receive updates.
- Admin: populate platform fields when editing streamers so YouTube channels appear even when offline.
- Admin: automatically run the roster “Check online status” action on admin load.
- Alert server: skip rotating log files on restart.
- UI: restore top-aligned layout by removing full-page centering so the header/footer sit in the right place.
- YouTube rate limiter: stop the ticker when adjusting test intervals to avoid leaking goroutines between runs.

## Legacy alertserver entries (merged)

### Added
- Added POST `/api/admin/streamers/status` and an admin console trigger that refreshes the online status of every stored channel on demand.
- Added the `internal/app` bootstrap package (with dedicated logging helpers and unit tests) so `cmd/alertserver/main.go` only wires its context and delegates to a single entrypoint.
- Added regression tests for the config loader to verify default resolution/override precedence now that `config.Load` returns structured errors instead of terminating the process.
- Added a typed config loader plus JSON schema that accepts a nested `server` block (with `addr`/`port`) and `youtube` overrides inside `config.json`, falling back to the historic flat keys so operators can retarget the HTTP listener without recompiling.
- Parse incoming YouTube WebSub notifications, fetch the related watch pages (no API key required), and persist streamer `status` details whenever a live broadcast starts so downstream tooling can react instantly.
- Introduced the v1 HTTP router with request-dump logging so every inbound request is captured alongside the YouTube alert verification endpoint.
- Added the `/api/youtube/subscribe` proxy that forwards JSON payloads to the YouTube PubSubHubbub hub while applying the required defaults.
- Added the `/api/youtube/unsubscribe` endpoint so operators can stop receiving hub callbacks for a topic without editing configs manually.
- Added the `/api/youtube/channel` lookup endpoint to convert @handles into canonical UC channel IDs.
- Added POST `/api/streamers` to persist streamer metadata into `data/streamers.json` for multi-platform support.
- Added GET `/api/streamers` so clients can list every stored streamer record.
- Added PATCH `/api/streamers` so existing streamer aliases/descriptions/languages can be updated without recreating the record.
- Added `streamer.description` to the schema and storage model so submissions can describe what makes each streamer unique.
- Derived `streamer.id` from the alias by stripping whitespace/punctuation and tightened the schema to enforce alphanumeric IDs.
- Reject duplicate streamer aliases by enforcing unique cleaned IDs during persistence and documenting the resulting `409 Conflict` behavior.
- Added `/api/youtube/metadata` so tooling can fetch channel summaries and auto-fill the description/name/YouTube handle fields when a URL is entered.
- Stored every YouTube PubSubHubbub request field (topic/callback/hub/verify mode/lease duration) in streamer records and documented the schema so the alert server can persist and inspect future subscription attempts without losing context.
- Added `streamer.languages` to the schema/storage plus validation so submissions only include supported language codes.
- Automatically subscribes YouTube channels (via PubSubHubbub) whenever a newly created streamer includes YouTube platform data, resolving channel IDs from handles when needed.
- Added a JSON schema (`schema/streamers.schema.json`) and typed storage layer for streamers so data persists with server-managed IDs and timestamps.
- Stubbed platform folders (`internal/platforms/{youtube,facebook,twitch}`) plus shared logging utilities to support future providers.
- Added a root `.gitignore` to drop editor/OS cruft, `cmd/alertserver/out.bin`, and other generated artifacts (including generated WebAssembly binaries).
- Added a root `README.md` with setup instructions and a canonical list of every HTTP endpoint so future additions stay documented.
- Added `/api/streamers/watch`, a server-sent events endpoint clients can subscribe to so browser dashboards reload when `streamers.json` changes.
- Once a WebSub notification confirms a YouTube livestream is online, persist the streamer’s `status` with the active video ID, start timestamp, and platform list so downstream tooling can display who’s live.
- Inspect POST `/alerts` WebSub notifications, parse the feed payload, and query YouTube to confirm whether the referenced video is a livestream that's currently online.
- Rotated `data/alertserver.log` into timestamped archives under `data/logs/` on startup so each run writes to a clean file without losing history.
- Added POST `/api/admin/login` plus an `admin` config block so the console can request bearer tokens tied to configured credentials, alongside GET/POST `/api/admin/submissions` for listing and approving/rejecting pending streamer submissions.
- Added GET `/api/admin/monitor/youtube` and the backing monitor service so the admin console can inspect lease health for every stored YouTube channel.
- Updated POST `/api/streamers` to queue submissions in `data/submissions.json` until an admin approves them, keeping `data/streamers.json` limited to vetted entries.
- Added a background YouTube lease monitor that renews subscriptions once ~95% of the current `leaseSeconds` window has elapsed so WebSub callbacks keep flowing without manual intervention.

### Changed
- Encapsulated the streamer and submissions file stores behind `streamers.Store`/`submissions.Store` so handlers, admins, and WebSub flows share path-scoped locks instead of package-level globals.
- Split the admin login/submission endpoints into dedicated services so handlers just authorize/encode responses while the new service layer covers approval/onboarding workflows with targeted tests.
- Documented the admin auth manager, HTTP handlers, and router exports so every public type/function ships with GoDoc coverage.
- Added package/type documentation across config, logging, store, and YouTube platform packages so exported APIs pass linting.
- Moved the YouTube channel lookup, metadata, and subscription HTTP handlers onto dedicated services so transport code only validates HTTP details while services manage upstream calls and defaults with new unit tests.
- Added regression tests for the submissions store so append/list/remove behaviors (including ID/SubmittedAt defaults) stay covered.
- Sanitized the WebSub notification handler logging so Atom parse errors are not logged (clients just get 400) and upstream fetch errors are logged only when informative, preventing noisy logs.
- Lease monitor now exposes `Stop()` and waits for renewal goroutines before exiting, letting the app tie the background YouTube subscription refresh loop to the server lifecycle cleanly.
- Submissions store now accepts injected clocks/ID generators so tests can deterministically assert `SubmittedAt` and ID values without relying on real time.
- Split the WebSub notification handler into a dedicated `service.AlertProcessor`, keeping the HTTP layer focused on method/path/response mapping while business logic (feed parsing, lookups, live-status updates) lives in the service with targeted tests.
- Added `.github/workflows/ci.yml` so gofmt/vet/test run on every push/PR, and documented the workflow in the README to keep config/transport/core/CI responsibilities clear.
- Added `docs/ARCHITECTURE.md` (linked from the README) documenting the layered design, key packages, background workers, and testing conventions so future contributors understand the configuration/transport/core separation.
- Reworked configuration/state wiring so YouTube hub/callback/verify/lease settings are injected through `internal/api/v1`, onboarding, admin submissions, and subscription clients instead of relying on the old `config.YT` globals.
- Removed the embedded alGUI assets/handler so the alert server stays API-only, returning a placeholder at `/` and keeping the UI’s traffic out of alert-server logs.
- Allowed `streamer.firstName`, `streamer.lastName`, and `streamer.email` fields to be blank in the JSON schema so optional contact details no longer trigger validation errors.
- Moved the HTTP router under `internal/api/v1` and updated docs/CLI tooling so future endpoints live under their API versioned package.
- Relocated metadata scraping into the YouTube platform tree and corralled all YouTube handlers/clients/subscribers beneath `internal/platforms/youtube/{api,metadata,store,subscriptions}` for clearer ownership.
- Simplified `POST /api/streamers` to accept only alias/description/languages plus a single YouTube channel URL, deriving the streamer ID, resolving channel metadata, generating a hub secret, updating the store, and triggering subscriptions automatically.
- Updated `DELETE /api/streamers/{id}` to require both the matching path parameter and a JSON body containing the `streamer.id` and original `createdAt` timestamp, ensuring accidental deletions are caught before records are removed.
- Added dedicated GET/POST/DELETE handler coverage for `/api/streamers` and now advertise all supported methods via the `Allow` header (including `DELETE`) so clients can reliably introspect the endpoint.
- Dropped the `/v1` segment from every public API path (for example, `/api/v1/streamers` is now `/api/streamers`) to simplify client integrations.
- Documented the shared `/api/streamers` handler (README + Postman collection) so clients understand DELETE lives on the same base path as GET/POST and can rely on the `Allow` header.
- Embedded the WebAssembly UI directly into the alert server binary so a single process now serves both the dashboard and the APIs.
- Renamed the metadata scraping endpoint to `/api/youtube/metadata` (including handler types) so the path and code align with what the endpoint returns.
- Removed the `createdAt` requirement from `DELETE /api/streamers/{id}` so operators only need to provide the streamer ID when deleting records.
- The subscribe handler now mirrors the hub's HTTP response (body/status) to the API client and falls back to the upstream status text when the hub omits a body.
- Consolidated the YouTube subscribe/unsubscribe handlers into a single JSON proxy, defaulting hub settings consistently and relocating lease tracking into the subscriptions package.
- Validation errors for `/api/youtube/subscribe` and `/api/youtube/unsubscribe` now surface as `400 Bad Request` responses so clients can correct their payloads instead of seeing `502 Bad Gateway`.
- `/api/youtube/unsubscribe` no longer requests a lease duration, allowing hub callbacks that omit `hub.lease_seconds` to complete successfully.
- YouTube hub verification now skips lease-duration comparisons (and lease writes) for unsubscribe callbacks so removing a subscription no longer fails or records a false renewal.
- Programmatic YouTube subscriptions now reuse the configured verify mode and lease duration so hub requests keep honoring `config.json` overrides.
- Normalized all YouTube WebSub defaults (callback URL, lease duration, verification mode) inside the handler so clients can omit them safely.
- Alert verification logging now includes the exact challenge response body so the terminal reflects what was sent back to YouTube.
- Server output and logs are now mirrored into `data/alertserver.log` so operational history persists across restarts.
- Accepts `/alert` as an alias for `/alerts` so PubSubHubBub callbacks from older reverse-proxy configs are handled correctly.
- Fixed the router and verification handler so both `/alert` and `/alerts` paths are actually registered, preventing 404s when Google hits the legacy plural route.
- Expanded YouTube hub verification logging to include the full HTTP dump and planned response so challenges can be reviewed before they’re sent.
- Issued unique `hub.verify_token` values for every subscription and reject hub challenges whose topic/token/lease don’t match what was registered (mirroring the configured HMAC secret).
- Consolidated all logging through the internal logger package so runtime output shares consistent formatting regardless of entry point, including a blank spacer line before every timestamped entry for readability.
- Added explicit logging after sending the hub challenge reply so the status/body echoed back to YouTube are captured.
- Made the YouTube WebSub defaults configurable through environment variables or CLI flags so deployments are not tied to baked-in hub/callback values.
- DELETE `/api/streamers` now unsubscribes the corresponding YouTube WebSub feed before removing the record so PubSubHubbub callbacks stop immediately.
- Restricted `/alerts` to GET requests from FeedFetcher-Google, logging the raw verification data and rejecting suspicious traffic instead of processing every request blindly.
- Hardened the YouTube HTTP handlers by sharing JSON/body validation, trimming whitespace, and surfacing meaningful `400` responses whenever subscribe/unsubscribe, metadata, or channel lookup payloads are malformed.
- WebSub subscriptions now dump the full hub response and log when Google accepts a request so operators can trace every step from the API proxy through confirmation.
- The companion alGUI now listens to `/api/streamers/watch` so the roster refreshes automatically whenever streamer data changes.

### Fixed
- Streamer service deletion tests now supply the required YouTube callback URL so subscription management validations mirror production behavior and `go test ./...` stays green.
- Reject admin and API updates that try to reuse an existing streamer alias so roster edits cannot accidentally duplicate names.
- Split the UI server handlers into dedicated admin/public files (login, submissions, streamers, status, home, streamer, sitemap) to reduce coupling and make future changes easier to navigate.
- Persist `streamer.alias` when creating records and require it as the primary identifier so requests without names no longer lose the alias field.
- Removed references to the deprecated `/api/youtube/new/subscribe` alias so the README only lists active endpoints.
- Allow `DELETE /api/streamers/{id}` to accept RFC3339 timestamps with or without fractional seconds so clients can resend the stored `createdAt` value without losing precision.
- Registered the consolidated `/api/streamers` handler in the router so DELETE requests (and the correct Allow header) are available to clients.
- Added the missing list/delete handler implementations so the `/api/streamers` handler actually builds with GET/POST/DELETE support.
- Restored the YouTube metadata handler import so `/api/youtube/metadata` compiles and keeps using the dedicated scraping package.
- Registered `/api/streamers/` alongside `/api/streamers` so DELETE requests to `/api/streamers/{id}` reach the handler instead of 404ing.
- Restored the subscribe/unsubscribe defaulting behavior so `NormaliseSubscribeRequest` and `NormaliseUnsubscribeRequest` only fill in blank fields, allowing clients to override callback/hub/verify/lease values.
- Ensured `ManageSubscription` forwards the configured or stored lease duration so YouTube hub calls keep the intended 10-day renewal window instead of falling back to the hub default.
- DELETE `/api/streamers/{id}` now validates `streamer.createdAt` locally so malformed timestamps return `400 Bad Request` instead of surfacing as `500` errors.

## Legacy UI entries (merged)

### Added
- Embedded the alert-server admin console directly into the WASM UI so operators can log in, moderate submissions, manage streamers, edit environment settings, and review monitor events without switching to a separate frontend bundle.
- Introduced an Admin Activity tab with Website/API sub-tabs so future telemetry streams have a dedicated home inside the console.
- Streamed the helper server's stdout/stderr into the Admin ▸ Activity ▸ Website tab so operators can monitor live JSON log output without leaving the UI.

### Changed
- Moved the admin Log out control to the dashboard header and slimmed it down to match the Monitor/Settings tabs so it stays visible without overpowering the UI.
- Documented the README with a fuller description plus the GitHub repo summary so the local history now aligns with the remote initial commit.
- Retooled the admin console to reuse the public `/api/streamers` endpoints (POST/PATCH/DELETE) so roster edits/removals work with the actual API surface while new entries still queue submissions for approval.
- Shifted the streamer edit/delete actions into the card header so controls stay aligned with each roster entry’s title/status row.
- Promoted the admin console to its own `/admin` page so the public roster no longer embeds the console; the home page now links out via a single Admin button.

### Fixed
- Ensured the admin roster renders fallback data with a warning if the streamers API cannot be reached, instead of leaving the section empty.
- Let the admin refresh continue populating the roster even when submissions/settings/monitor endpoints fail, surfacing warnings rather than aborting the update.
- Updated the admin submissions fetcher to support wrapped responses and treat missing settings/monitor endpoints as optional so refreshes succeed without warnings.
- Rebuilt the WebAssembly bundle so the roster resiliency fixes are reflected in the published UI.
- Cache the public roster inside the WASM app so the admin console can display the latest public data when the streamers API returns no records.
- Guarded the admin settings refresh from nil responses so missing `/api/admin/settings` endpoints no longer crash the WASM runtime.
- Added autocomplete hints to the admin login inputs to satisfy browser accessibility recommendations.
- Triggered an automatic admin logout when authenticated API calls return unauthorized so expired tokens immediately redirect operators back to the login form.
- Detect unauthorized responses from the admin log EventSource and log out automatically when the stream reports expired credentials.

### Removed
- Removed the custom alert-server logging pipeline and related YouTube handler log hooks to simplify runtime dependencies.
- Removed the manual roster refresh button from the public header; the roster already updates via live events and retry controls.
- Dropped the in-console “Add new streamer” form—new entries now flow exclusively through the public submission form before admins approve them.
- Deleted the legacy `frontend/` workspace so the WASM UI is now the single source of truth for Sharpens's dashboard bundle.
