# Changelog

## [Unreleased]
- Design Sharpen Live landing page with custom logo and live streamer status table.
- Relocate static site assets from `doc/` to `web/`.
- Add YouTube alert listener service that polls live status every five minutes.
- Modularize YouTube alert application with dedicated packages and automated tests.
- Add `web/streamers.json` to track streamer metadata.
- Support PubSubHubbub verification callbacks for `/alerts`.
- Ensure PubSubHubbub verification replies echo `hub.challenge` with a `200 OK` status.
