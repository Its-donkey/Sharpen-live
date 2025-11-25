# Changelog

## Unreleased

### Added
- Admin: add roster “Check online status” action and API to refresh channel state on demand.
- Submit form: detect @handles, prompt for platform, and expand to full channel URLs.
- Submit form: preselect English and add an “Add another language” button consistent with platform controls.

### Fixed
- Alert server: rotate the log file after 24 hours of uptime, matching the restart rotation behavior.
- Submit form: align channel URL rows to the grid and constrain inputs to half-width for consistent layout.
- Submit form: remove padding/border radius from channel URL inputs so they sit flush in the grid.
- Submit form: always show handle platform picker when an @handle is entered, even before metadata loads.
- Submit form: render handle platform picker markup even before a handle is typed so it can be toggled on.
- Submit form: remove padding/border radius on handle platform selects so they render flush.
- Submit form: keep the handle dropdown markup consistent between SSR and WASM to avoid duplicate platform rows.
- Submit form: place the add-language button before the dropdown for clearer horizontal controls.
- Submit form: show selected language tags above the controls for better visibility.
- UI: remove the redundant “Roster” header button from the home view.
- Submit form: hide the form behind an “Add a Streamer” button and reveal it on demand.
- Submit form: fix the “Add a Streamer” toggle so it opens the hidden form as intended.
- Submit form: stretch the lead help text across the full form grid for consistent alignment.
- Submit form: hide the language dropdown until “Add another language” is clicked, then swap back once a choice is made.
- Guard WebSub hub.challenge and reject malformed values to avoid reflected content.
- Harden YouTube WebSub requests by validating hub, topic, and callback URLs.
- Admin: validate admin tokens for settings/log streams instead of accepting any non-empty value.
- Admin: ensure `/admin/logs` streams valid server-sent events so the Activity tab shows live logs again.
- Alert server: encode HTTP request/response log payloads as JSON so downstream log streams stay valid.
- Alert server: emit default SSE message events on `/api/streamers/watch` so EventSource listeners receive updates.
- Admin: populate platform fields when editing streamers so YouTube channels appear even when offline.
- Admin: automatically run the roster “Check online status” action on admin load.
- Alert server: skip rotating log files on restart.
