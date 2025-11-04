# Sharpen Live

Sharpen Live is an end-to-end platform for curating professional knife sharpening streams, onboarding new partners, and monitoring live event signals. The project contains a Go backend for moderation and automation workflows, a React + Vite frontend for both the public roster and admin console, a small documentation site, and helper scripts for running everything locally.

## Table of Contents
- [Architecture Overview](#architecture-overview)
- [Quick Start](#quick-start)
- [Backend API Service](#backend-api-service)
- [Frontend Web Application](#frontend-web-application)
- [Data Files](#data-files)
- [Automation Scripts](#automation-scripts)
- [Documentation Site](#documentation-site)
- [Contributing](#contributing)

## Architecture Overview
The repository is organized into the following top-level components:

| Path | Description |
| --- | --- |
| [`backend/`](backend/) | Go modules that expose the HTTP API, connect to PostgreSQL for site settings, and persist streamer data to JSON. Entry point: [`cmd/api-server/main.go`](backend/cmd/api-server/main.go). |
| [`frontend/`](frontend/) | React + Vite single-page application that renders the public roster, submission form, and admin console. Entry point: [`src/main.tsx`](frontend/src/main.tsx). |
| [`backend/data/`](backend/data/) | Default JSON fixtures for the streamer roster and pending submissions used by the API JSON store. |
| [`scripts/`](scripts/) | Utility scripts for running the complete stack during development. |
| [`docs/`](docs/) | Jekyll configuration used to publish the project documentation site. |

Supporting packages worth noting:

- [`backend/internal/api`](backend/internal/api) – request handlers, validation helpers, CORS middleware, and YouTube PubSub monitoring utilities. The handler routes are wired in [`server.go`](backend/internal/api/server.go).
- [`backend/internal/config`](backend/internal/config) – loads environment configuration, applies defaults for directories and data files, and validates required settings. [`config.go`](backend/internal/config/config.go)
- [`backend/internal/storage`](backend/internal/storage) – mutex-protected JSON persistence for streamers and submissions, with helper methods for CRUD operations. [`storage.go`](backend/internal/storage/storage.go)
- [`backend/internal/settings`](backend/internal/settings) – abstractions and implementations (PostgreSQL, in-memory) for persisting site settings such as admin credentials and YouTube alert metadata. [`postgres.go`](backend/internal/settings/postgres.go)

## Quick Start
1. **Install prerequisites**
   - Go 1.23+
   - Node.js 18+ (or any modern LTS) and npm
   - PostgreSQL 14+ (or a managed instance) for persisting site settings

2. **Clone the repository**
   ```bash
   git clone https://github.com/Its-donkey/Sharpen-live.git
   cd Sharpen-live
   ```

3. **Run the full development environment**
   ```bash
   make online
   ```

   The [`online` script](scripts/online.sh) installs frontend dependencies (if missing), sets default environment variables (including admin credentials, JSON data paths, and Vite API origin), then launches the Vite dev server and the Go API concurrently. The API defaults to `http://localhost:8880` and the frontend to `http://localhost:5173`.

## Backend API Service
The backend resides in [`backend/`](backend/) and is built as a Go module targeting Go 1.23. The main entry point is [`cmd/api-server/main.go`](backend/cmd/api-server/main.go), which performs the following on startup:

1. Loads configuration from environment variables via [`config.FromEnv`](backend/internal/config/config.go) and validates required values.
2. Establishes a PostgreSQL connection pool and ensures the settings schema exists using [`settings.PostgresStore.EnsureSchema`](backend/internal/settings/postgres.go).
3. Seeds site settings from environment defaults when the table is empty, and keeps database rows synchronized with runtime configuration.
4. Initializes the JSON-backed data store for streamers and submissions [`storage.NewJSONStore`](backend/internal/storage/storage.go).
5. Constructs the HTTP server with API routes and static asset handling through [`api.New`](backend/internal/api/server.go).

### Configuration
Environment variables control runtime behavior. The table below lists the most important values (defaults come from [`config.go`](backend/internal/config/config.go) and [`scripts/online.sh`](scripts/online.sh)).

| Variable | Purpose | Default |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL connection string used by the settings store. | _required_ |
| `LISTEN_ADDR` | Address for the API server to bind. | `:8880`
| `PORT` | Alternative to `LISTEN_ADDR` for platforms that only expose a port number. | _unset_
| `ADMIN_TOKEN` | Shared secret required for admin API requests. | `dev-admin-token`
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | Credentials accepted by the admin login endpoint. | `admin@sharpen.live` / `changeme123`
| `YOUTUBE_API_KEY` | API key for fetching YouTube channel metadata. | _unset_
| `YOUTUBE_ALERTS_CALLBACK` | Public callback URL for PubSub notifications. Enables alert subscriptions when set. | _unset_
| `YOUTUBE_ALERTS_SECRET` | Shared secret used to validate PubSub messages. | _unset_
| `YOUTUBE_ALERTS_VERIFY_PREFIX` / `YOUTUBE_ALERTS_VERIFY_SUFFIX` | Prefix/suffix added to PubSub verify tokens. | _unset_
| `YOUTUBE_ALERTS_HUB_URL` | Overrides the PubSub hub endpoint. | `https://pubsubhubbub.appspot.com/subscribe`
| `SHARPEN_DATA_DIR` | Base directory for JSON data files. | `backend/data`
| `SHARPEN_STREAMERS_FILE` / `SHARPEN_SUBMISSIONS_FILE` | Paths to JSON persistence for streamers and submissions. | `<data dir>/streamers.json`, `<data dir>/submissions.json`
| `SHARPEN_STATIC_DIR` | Directory containing built frontend assets served by the API. | `frontend/dist`

> **Note:** When the API boots it persists the effective file paths and static directory back into the settings store so subsequent restarts reuse the resolved locations. [`main.go`](backend/cmd/api-server/main.go)

### Running the server manually
Once configuration is in place (including `DATABASE_URL`), you can run the API on its own:

```bash
cd backend
LISTEN_ADDR=":8880" \
DATABASE_URL="postgres://user:pass@localhost:5432/sharpen_live?sslmode=disable" \
ADMIN_TOKEN="dev-admin-token" ADMIN_EMAIL="admin@sharpen.live" ADMIN_PASSWORD="changeme123" \
go run ./cmd/api-server
```

Build a production binary with:

```bash
cd backend
go build -o bin/api-server ./cmd/api-server
```

### Testing
Execute all Go unit tests with:

```bash
cd backend
go test ./...
```

Tests exercise configuration loading, storage behavior, and API handlers. Network access is required the first time Go modules are downloaded.

### API Surface
Key routes exposed by [`Server.Handler`](backend/internal/api/server.go) include:

| Route | Method(s) | Description |
| --- | --- | --- |
| `/api/streamers` | `GET` | Public roster of approved streamers. |
| `/api/submit-streamer` | `POST` | Public submission endpoint that enqueues entries for moderation. |
| `/api/admin/login` | `POST` | Exchanges admin email/password for the configured admin token. |
| `/api/admin/streamers` | `GET`, `POST` | List existing streamers and create new ones (requires `Authorization: Bearer <token>`). |
| `/api/admin/streamers/{id}` | `PUT`, `DELETE` | Update or delete a streamer by ID. |
| `/api/admin/submissions` | `GET`, `POST` | Review submissions and approve/reject them. |
| `/api/admin/settings` | `GET`, `PATCH` | Inspect or update persisted site settings. |
| `/api/admin/monitor/youtube` | `GET`, `POST` | View PubSub event logs and (re)subscribe channels to alerts. |

Static assets are served from the configured `StaticDir`, and the SPA handler injects runtime configuration (`LISTEN_ADDR`) before returning `index.html`. [`spaHandler`](backend/cmd/api-server/main.go)

## Frontend Web Application
The frontend lives in [`frontend/`](frontend/) and uses Vite with React 18 and React Router for navigation. The root renderer is defined in [`src/main.tsx`](frontend/src/main.tsx) and mounts the [`App`](frontend/src/App.tsx) component.

### Installing dependencies
```bash
cd frontend
npm install
```

### Available scripts
- `npm run dev` – Start the Vite development server (defaults to port 5173). Configure `VITE_API_BASE_URL` to point at the API when proxying requests. [`package.json`](frontend/package.json)
- `npm run build` – Produce a production-ready bundle in `frontend/dist`, which the Go API can serve as static assets.
- `npm run preview` – Preview the built assets locally.

### Application layout
The SPA provides two primary routes defined in [`App.tsx`](frontend/src/App.tsx):

- `/` – Public homepage that lists streamers via `<StreamerTable>`, includes a submission form (`<SubmitStreamerForm>`), and displays status/CTA components.
- `/admin` – Admin console (`<AdminConsole>`) that allows moderators to authenticate, manage streamers, and review submissions. Admin credentials and tokens persist in local storage via the `useAdminToken` hook.

Shared styling lives in [`styles.css`](frontend/src/styles.css), and API utilities reside in [`src/api.ts`](frontend/src/api.ts).

## Data Files
Default data lives in [`backend/data`](backend/data):

- [`streamers.json`](backend/data/streamers.json) – Seed roster displayed on the public homepage. Each entry contains IDs, descriptions, status labels, languages, and associated streaming platforms. The JSON store appends and persists updates through the admin API. [`storage.go`](backend/internal/storage/storage.go)
- [`submissions.json`](backend/data/submissions.json) – Queue for pending partner submissions awaiting admin approval. Populated via `/api/submit-streamer` and managed through admin endpoints.

When running locally the JSON store creates these files (and their parent directories) if they do not exist and ensures they contain valid arrays. [`ensureFile`](backend/internal/storage/storage.go)

## Automation Scripts
`scripts/online.sh` orchestrates the local developer workflow:

1. Ensures frontend dependencies are installed (`npm --prefix frontend install`).
2. Sets sane defaults for API origin, admin credentials, JSON data locations, and static asset directory.
3. Runs `npm run dev -- --host` inside `frontend/` and `go run ./cmd/api-server` inside `backend/` in the background, cleaning up both processes on exit.

Invoke it directly or through `make online`. [`scripts/online.sh`](scripts/online.sh)

## Documentation Site
The [`docs/`](docs/) directory contains a minimal Jekyll site configured with the `pages-themes/minimal` theme. [`_config.yml`](docs/_config.yml) and [`index.md`](docs/index.md) provide project summaries and quick-start information.

To preview the documentation locally:

```bash
cd docs
bundle install
bundle exec jekyll serve
```

The generated site mirrors key information from this README and is suitable for deployment to GitHub Pages.

## Contributing
1. Fork the repository and create a feature branch.
2. Run linters/tests where available (`go test ./...`, `npm run build`).
3. Open a pull request describing your changes and include relevant screenshots for UI updates when possible.

Issues and feature suggestions are welcome. Before filing, please search existing issues to avoid duplicates.
