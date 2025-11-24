# Changelog

## Unreleased

### Added
- Admin: add roster “Check online status” action and API to refresh channel state on demand.
- Submit form: detect @handles, prompt for platform, and expand to full channel URLs.
- Submit form: preselect English and add an “Add another language” button consistent with platform controls.

### Fixed
- Alert server: rotate the log file after 24 hours of uptime, matching the restart rotation behavior.
- Submit form: align channel URL rows to the grid and constrain inputs to half-width for consistent layout.
- Submit form: always show handle platform picker when an @handle is entered, even before metadata loads.
- Submit form: keep the handle dropdown markup consistent between SSR and WASM to avoid duplicate platform rows.
- Submit form: place the add-language button before the dropdown for clearer horizontal controls.
- Guard WebSub hub.challenge and reject malformed values to avoid reflected content.
- Harden YouTube WebSub requests by validating hub, topic, and callback URLs.
- Admin: validate admin tokens for settings/log streams instead of accepting any non-empty value.
- Admin: ensure `/admin/logs` streams valid server-sent events so the Activity tab shows live logs again.
- Admin: populate platform fields when editing streamers so YouTube channels appear even when offline.
- Admin: automatically run the roster “Check online status” action on admin load.
- Alert server: skip rotating log files on restart.
