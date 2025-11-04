# Changelog

## [Unreleased]
- Implement Sharpen Live API service with JSON-backed storage, admin endpoints, and accompanying tests plus local Go module packaging.
- Rebuild the Sharpen Live frontend as a React application with submission workflow, admin dashboard, and Vite toolchain.
- Add `scripts/online.sh`/`make online` dev launcher, default API port 8880, and updated client configuration to target the new endpoints.
- Fix React submit form platform removal to reset rows with `createPlatformRow()` so builds succeed.
- Require admin console authentication via email/password login endpoint and React UI update.
- Move admin tools to dedicated `/admin` route with router-based navigation and dev-prefilled credentials.
- Auto-detect data directory when the API runs from module subdirectory to avoid missing JSON files.
- Extract Sharpen Live landing page styles into `frontend/styles.css` and generate the streamer roster from JSON with live-aware platform links.
- Design Sharpen Live landing page with custom logo and live streamer status table.
- Relocate static site assets from `doc/` to `frontend/`.
- Add YouTube alert listener service that polls live status every five minutes.
- Modularize YouTube alert application with dedicated packages and automated tests.
- Add `frontend/streamers.json` to track streamer metadata.
- Restructure repository into `backend/` and `frontend/` applications with shared tooling paths.
- Rename the Go API binary to `cmd/api-server` and group HTTP handlers under `internal/api` for clarity.
- Add Go unit tests covering configuration, middleware, and SPA handler utilities.
- Provide default admin token in dev launcher so backend starts without extra env vars.
- Refresh submission form to auto-set status and manage languages via curated dropdown with removable chips.
- Display language names as “English / français”-style labels in the submission picker.
- Add admin dashboard tabs for streamers and settings, including editable environment values.
- Support PubSubHubbub verification callbacks for `/alerts`.
- Ensure PubSubHubbub verification replies echo `hub.challenge` with a `200 OK` status and log successful confirmations.
