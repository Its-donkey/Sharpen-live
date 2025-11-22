# Changelog

## [Unreleased]
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
- Removed the manual roster refresh button from the public header; the roster already updates via live events and retry controls.
- Dropped the in-console “Add new streamer” form—new entries now flow exclusively through the public submission form before admins approve them.
### Removed
- Deleted the legacy `frontend/` workspace so the WASM UI is now the single source of truth for Sharpens's dashboard bundle.
