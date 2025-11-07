# Sharpen Live

Sharpen Live is an end-to-end tooling stack for discovering live streamer events and managing onboarding workflows. The repository bundles a Go backend (API plus YouTube PubSub alert listener), a Vite/React frontend for submissions and admin tooling, and helper scripts for running everything locally.

## Features

- Admin dashboard for curating streamer profiles, submissions, and site settings.
- YouTube PubSub alert server that queues notifications, validates channels, and logs events for later review.
- PostgreSQL-backed monitor logs plus JSON seed data for local development.
- React/Vite frontend that surfaces submission forms and the admin console.
- Make/shell scripts that spin up the full stack with sensible defaults.

## Repository Layout

| Path      | Description                                                              |
|-----------|--------------------------------------------------------------------------|
| `backend/`| Go services, including the API (`cmd/api-server`) and YouTube alerts app |
| `frontend/`| Vite + React client, admin UI, and submission forms                      |
| `scripts/`| Utility helpers (`make online`, stream resolver helpers, etc.)            |
| `docs/`   | GitHub Pages marketing/docs site                                          |

## Requirements

- Go 1.21+ (module-mode build)
- Node.js 18+ / npm (for the frontend)
- PostgreSQL 14+ (YouTube monitor + settings)
- Make + bash/zsh for the provided scripts

## Quick Start

1. **Install dependencies**
   ```bash
   npm install --prefix frontend
   go mod download ./...
   ```
2. **Provision configuration**
   - Copy `.env.example` (if present) or export the needed variables (`ADMIN_TOKEN`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `DATABASE_URL`, `YOUTUBE_API_KEY`, etc.).
   - For local JSON storage, ensure `backend/data` contains `streamers.json`/`submissions.json`.
3. **Run the stack**
   ```bash
   make online
   ```
   The target launches the API (port 8880 by default) and the Vite dev server, wiring cross-origin access automatically.
4. **YouTube alerts listener (optional)**
   ```bash
   go run ./backend/platforms/youtube/cmd/alerts-server
   ```
   When run interactively, the binary prompts for the listen port, API key, and database URL if the environment variables are missing.

## Testing

- Backend: `go test ./...`
- Frontend: `npm test --prefix frontend` or `npm run lint --prefix frontend`

## Deployment Notes

- Build the frontend bundle with `npm run build --prefix frontend`; assets land in `frontend/dist`.
- Run the API server via `go run ./backend/internal/api` or build an executable and deploy with the required environment (see `backend/internal/config/config.go` for the full list).
- The YouTube alert listener requires access to PostgreSQL (for logging) and the YouTube Data API key.

## Contributing

Issues and pull requests are welcome. Please:

1. Fork and branch from `main` or the relevant feature branch.
2. Add/update tests when touching backend logic.
3. Update `CHANGELOG.md` for user-visible changes.
4. Run `make lint` / `go test ./...` / `npm test --prefix frontend` before submitting.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
