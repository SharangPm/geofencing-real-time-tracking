# FleetOps geofencing and real-time tracking

FleetOps is a Go REST service backed by PostgreSQL and a React/Vite fleet dashboard. It records vehicle location history, evaluates closed polygon geofences in Go (including their boundaries), persists matching entry and exit events transactionally, and streams committed alerts over WebSockets.

## Start the full stack

Requirements: Docker Desktop with its Linux engine running and Docker Compose v2.

```sh
docker compose up --build
```

Open http://localhost:5173. The stack starts with an empty PostgreSQL database, applies the migration automatically, and persists data in the `postgres_data` volume. The browser calls the API through the frontend Nginx `/api` proxy, and Nginx proxies `/ws/alerts` as a WebSocket. For a non-local deployment, set a strong `POSTGRES_PASSWORD` in `.env` before starting Compose.

## Local development

Run PostgreSQL 17+ and create a database/user, then set `DATABASE_URL` (the default is `postgres://fleet:fleet@localhost:5432/fleet?sslmode=disable`). Start the API from the repository root:

```sh
go run .
```

In another terminal:

```sh
cd web
npm ci
npm run dev
```

Visit http://localhost:5173. The Vite development build calls `http://localhost:8080`. Configuration: `ADDR`, `DATABASE_URL`, `DB_MIN_CONNS`, `DB_MAX_CONNS`, `CORS_ORIGIN`, and (for a custom browser API origin) `VITE_API_URL`. `.env.example` lists the variables.

## APIs

Every JSON HTTP API response, including errors and CORS preflight responses, contains `time_ns` as a string measured from handler entry. WebSocket upgrade responses use the WebSocket protocol; each WebSocket event retains its `type` and `event` structure.

| Method | Path | Purpose |
| --- | --- | --- |
| POST, GET | `/geofences` | Create/list polygons; list accepts `category` |
| PUT | `/geofences/{geofence_id}` | Update an existing polygon (used by the geofence edit form) |
| POST, GET | `/vehicles` | Register/list vehicles |
| POST | `/vehicles/location` | Record a location, update membership and create matching events |
| GET | `/vehicles/location/{vehicle_id}` | Return vehicle location and current geofence memberships |
| POST | `/alerts/configure` | Add `entry`, `exit` or `both` rule; omit `vehicle_id` for fleet-wide |
| GET | `/alerts` | List rules; accepts `geofence_id`, `vehicle_id` |
| GET | `/alerts/unread` | Return unread persisted alert IDs and their count |
| POST | `/alerts/read` | Mark supplied `alert_ids` as read without creating duplicate alert records |
| GET | `/violations/history` | Filter history by vehicle, geofence, `from`, `to`; paginated via `limit` (max 500) and `offset` |
| GET | `/ws/alerts` | Subscribe to committed geofence events |

Polygons use `[latitude, longitude]` pairs, must contain at least four points, be closed, and use valid coordinate ranges. The geofence page provides Leaflet Draw controls for creating, editing and removing an unsaved polygon; manual coordinate input remains available. Vehicle map clicks submit actual location updates.

## Example requests

```sh
curl -X POST http://localhost:8080/geofences -H 'Content-Type: application/json' \
  -d '{"name":"Depot","description":"Main depot","category":"delivery_zone","coordinates":[[37.77,-122.44],[37.78,-122.44],[37.78,-122.43],[37.77,-122.43],[37.77,-122.44]]}'
curl -X POST http://localhost:8080/vehicles -H 'Content-Type: application/json' \
  -d '{"vehicle_number":"TRK-01","driver_name":"A. Driver","vehicle_type":"truck","phone":"555-0100"}'
```

## Database and tests

`migrations/001_init.sql` creates geofences, vehicles, location history, current vehicle/geofence state, alert configurations and violations, with foreign keys, uniqueness checks, coordinate/category constraints and lookup indexes. `migrations/002_violation_read_state.sql` stores read timestamps on existing violations. The API uses pgx pooling and a database transaction for location history, current positions, membership transitions and violation inserts. Events are broadcast only after commit.

Set `TEST_DATABASE_URL` to a disposable PostgreSQL database to run API, persistence and multi-client WebSocket integration tests against PostgreSQL. The suite checks all nine required endpoint/method contracts, response timing, overlapping zones, polygon boundaries, first/repeated/entry/exit/re-entry location transitions, fleet-wide and vehicle-specific rules, disconnect handling, and persistence through a fresh API instance:

```sh
go test ./...
go vet ./...
cd web && npm run build
docker compose config
```

The dashboard loads its statistics, tables, filters, maps, history and alert rules from the API. The geofence map supports drawing, editing and removing unsaved polygons; manual coordinate entry is also available. Vehicle details, location status filters, and saved geofence editing use persisted API data.
