# Setup

## Docker Compose (recommended)

1. Start Docker Desktop and wait for its Linux engine to report running.
2. From this directory, run `docker compose up --build`.
3. Open http://localhost:5173. The API is available at http://localhost:8080.

Compose starts PostgreSQL, waits for its health check before starting Go, and waits for the API health check before starting the frontend. On first boot, Go applies `migrations/001_init.sql`. The `postgres_data` named volume keeps records between container restarts. Override `POSTGRES_PASSWORD` in `.env` when using a shared environment.

## Local development

Provide PostgreSQL and set a valid `DATABASE_URL`, then run `go run .`. The server retries its PostgreSQL connection for up to one minute on startup. In `web`, run `npm ci` and `npm run dev`; Vite serves at http://localhost:5173 and connects to the API at http://localhost:8080 by default.

## Checks

Set `TEST_DATABASE_URL` to a disposable PostgreSQL database before `go test ./...`. The integration suite applies the migration and writes uniquely named records, then cleans up the geofence and dependent history. Run `go vet ./...`, `cd web && npm run build`, and `docker compose config` for the static and frontend checks.
