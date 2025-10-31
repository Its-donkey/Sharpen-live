# YouTube Alert Listener

This service listens for live alerts for Sharpen Live creators on YouTube and polls every five minutes to ensure the stream is still running.

## Running locally

1. `cd platforms/youtube`
2. `go run ./cmd/alerts-server`

### Running tests

```bash
go test ./...
```

### Environment variables

- `YOUTUBE_API_KEY` – required to call the YouTube Data API v3. You can create an API key under a Google Cloud project with the YouTube Data API enabled.
- `LISTEN_ADDR` – optional custom address (defaults to `:8080`). A `PORT` variable will also be honoured if `LISTEN_ADDR` is unset.
- `POLL_INTERVAL` – optional duration (e.g. `2m`) controlling re-check cadence. Defaults to `5m`.
- `SHUTDOWN_GRACE_PERIOD` – optional duration (e.g. `5s`) for graceful HTTP shutdown. Defaults to `10s`.

Without an API key the service will start, but polling attempts will fail until the variable is provided.

## HTTP interface

- `POST /alerts`

```json
{
  "channelId": "<youtube-channel-id>",
  "streamId": "<optional-youtube-stream-id>",
  "status": "online"
}
```

Any status other than `online` will cancel polling for the supplied `channelId`.
