## Streamer onboarding comparison

| Step | Postman flow (17:14 UTC) | alGUI flow (17:37 UTC) | Match |
| --- | --- | --- | --- |
| Delete existing record | `DELETE /api/streamers` with body `{"streamer":{"id":"Iwillshavemyeyebrows"}}` → `200 OK` | Same request and result | ✅ |
| Prefill metadata | n/a (alias/description entered manually) | `POST /api/youtube/metadata` for `https://www.youtube.com/@I-will-shave-my-eyebrows` → existing description/title resolved | Extra helper call used only by alGUI UI (expected) |
| Create streamer | `POST /api/streamers` with alias/description/language + `platforms.url` | Identical body after UI update (`platforms.url` matches) | ✅ |
| Hub subscription | Outbound `POST /subscribe` (lease 864000, verify async) using derived channelId/handle; Google hub challenge handled + `200 OK` echo | Same outbound request/verification cycle (new hubSecret/verify_token generated, as expected) | ✅ |
| streamers.json entry | Record updated with YouTube platform metadata containing `handle`, `channelId`, `hubSecret`, `topic`, `callbackUrl`, `hubUrl`, `verifyMode`, `leaseSeconds` | Same fields populated (new secret + timestamps) | ✅ |

**Notes**
- Both flows now rely on the server-managed onboarding: the UI passes only `platforms.url`, so the backend resolves the channel, writes YouTube metadata, and triggers WebSub automatically.
- The only differences in the logs are timestamps, generated secrets (`hubSecret`, `hub.verify_token`), and lease renewal times—these are expected for each run.
