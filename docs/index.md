---
layout: default
title: Sharpen Live
---

# Sharpen Live

Sharpen Live is an end-to-end tooling stack for managing streamer onboarding and discovering live events.

## Repository Layout

- **backend/** – Go services, storage layer, and supporting packages.
- **frontend/** – Vite + React frontend that surfaces the admin console and submission forms.
- **scripts/** – Utility scripts for local development and deployment support.

## Local Development

Run the combined dev environment with:

```
make online
```

That script starts the Vite dev server and the Go API concurrently, and configures the necessary environment variables.

## Additional Resources

- Backend server entry point: `backend/cmd/api-server/main.go`
- Frontend entry: `frontend/src/main.tsx`

Feel free to file issues or pull requests if you run into problems or have improvements to share.
