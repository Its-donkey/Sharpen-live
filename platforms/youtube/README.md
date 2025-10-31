# YouTube Alert Listener

This service listens for live alerts for Sharpen Live creators on YouTube and polls every five minutes to ensure the stream is still running.

## Running locally

```bash
go run ./platforms/youtube
```

### Environment variables

- `YOUTUBE_API_KEY` – required to call the YouTube Data API v3. You can create an API key under a Google Cloud project with the YouTube Data API enabled.

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
