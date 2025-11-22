# Changelog

## Unreleased

### Added
- Admin: add roster “Check online status” action and API to refresh channel state on demand.
- Submit form: detect @handles, prompt for platform, and expand to full channel URLs.

### Fixed
- Submit form: always show handle platform picker when an @handle is entered, even before metadata loads.
- Submit form: constrain channel URL inputs to half-width for consistent layout.
- Guard WebSub hub.challenge and reject malformed values to avoid reflected content.
- Harden YouTube WebSub requests by validating hub, topic, and callback URLs.
- Admin: validate admin tokens for settings/log streams instead of accepting any non-empty value.
- Admin: ensure `/admin/logs` streams valid server-sent events so the Activity tab shows live logs again.
- Admin: populate platform fields when editing streamers so YouTube channels appear even when offline.
