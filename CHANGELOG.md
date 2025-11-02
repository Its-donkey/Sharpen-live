# Changelog

## [Unreleased]
- Move streamer roster data to `api/data/streamers.json` and update the frontend/backend references.
- Route streamer submissions through `/api/submit-streamer` backend using a server-held GitHub token and update the landing page to consume it.
- Ensure streamer submission GitHub requests set `Content-Type: application/json` when sending payloads.
- Add public submission flow that opens GitHub pull requests for new streamers from the landing page.
- Extract Sharpen Live landing page styles into `web/styles.css` and generate the streamer roster from JSON with live-aware platform links.
- Design Sharpen Live landing page with custom logo and live streamer status table.
- Relocate static site assets from `doc/` to `web/`.
- Add YouTube alert listener service that polls live status every five minutes.
- Modularize YouTube alert application with dedicated packages and automated tests.
- Add `web/streamers.json` to track streamer metadata.
- Support PubSubHubbub verification callbacks for `/alerts`.
- Ensure PubSubHubbub verification replies echo `hub.challenge` with a `200 OK` status and log successful confirmations.
