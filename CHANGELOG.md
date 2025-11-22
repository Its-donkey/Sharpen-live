# Changelog

## Unreleased

### Added
- Admin: add roster “Check online status” action and API to refresh channel state on demand.

### Fixed
- Submit form: align channel URL fields to the same grid sizing as other inputs.
- Submit form: render channel URL inputs full width within the platform fieldset.
- Guard WebSub hub.challenge and reject malformed values to avoid reflected content.
- Harden YouTube WebSub requests by validating hub, topic, and callback URLs.
- Admin: validate admin tokens for settings/log streams instead of accepting any non-empty value.
- Admin: ensure `/admin/logs` streams valid server-sent events so the Activity tab shows live logs again.
