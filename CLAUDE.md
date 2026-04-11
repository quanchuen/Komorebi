# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Full dev stack (API + Martin + Valhalla + Web)
make dev-run

# Individual services
JWT_SECRET=cyclist-map-dev-secret-do-not-use-in-production \
  DATABASE_URL="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go run ./cmd/api                          # Go API on :8080
martin --config martin.yaml                 # Vector tiles on :3000
docker compose up valhalla                  # Routing engine on :8002
cd web && npm run dev                       # SvelteKit on :5173

# Build
go build ./cmd/api
cd web && npm run build
```

## Testing

```bash
go test ./...                               # All Go tests
go test ./internal/app -run TestAuth        # Single test pattern
go test -v ./internal/infra/postgres        # Verbose, one package
cd web && npm run check                     # Svelte type checking
```

Integration tests in `infra/postgres/` connect to the real database. They use `TEST_DB_DSN` or the default connection string and skip gracefully if unreachable. Test stubs are hand-written (no mocking library) — see `testutil_test.go` files for patterns.

## Database

PostgreSQL 18 + PostGIS 3.6. Connection: `postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev`

```bash
make migrate-up                             # Apply pending migrations
make migrate-down                           # Rollback one migration
make migrate-create                         # Create new migration pair
```

Migrations use golang-migrate, numbered `000001`–`000021`. Four schemas: `routes`, `community`, `environment`, `plan`. The `osm` schema is managed by osm2pgsql.

## Architecture

**Hexagonal DDD** with four layers:

- **`internal/domain/`** — Pure Go types, interfaces, business rules. Zero external imports. Five bounded contexts: `route`, `community`, `environment`, `plan`, `discovery`.
- **`internal/app/`** — Application services orchestrating domain objects. Each context has a service (e.g., `RouteService`, `RoutingService`, `AuthService`).
- **`internal/infra/`** — Adapter implementations: `postgres/` (repositories), `valhalla/` (routing client), `openmeteo/` (weather client), `tomorrowio/`, `openweathermap/`.
- **`internal/api/`** — HTTP handlers + chi router. Thin layer: parse request, call service, write JSON.

**Key domain concepts:**
- `Route` (aggregate root) has `Waypoints`, `Segments`, `Tags`. States: Draft → Published → Archived.
- `RoutePlan` holds ordered `StopPoints` + `PlanTasks` with hashtag venue resolution (#konbini, #cafe).
- `EnvironmentService` computes time-projected conditions (shade, wind, rain, UV, greenery, signals) per segment using the speed model to project arrival times.
- `RoutingService.GetDirections()` calls Valhalla with 3 profiles in parallel (Suggested, Fast, Avoid Main Roads).

**Wiring:** `cmd/api/main.go` constructs all repos → services → handlers → chi router. No DI framework.

## Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `DATABASE_URL` | Yes | — | PostGIS connection |
| `JWT_SECRET` | Yes | — | HS256 JWT signing key |
| `PORT` | No | `8080` | API listen port |
| `VALHALLA_URL` | No | `http://localhost:8002` | Routing engine |
| `WEATHER_PROVIDER` | No | `open-meteo` | `open-meteo`, `tomorrow-io`, `openweathermap` |
| `WEATHER_API_KEY` | For paid providers | — | Tomorrow.io or OWM key |

## Data Pipelines

```bash
make osm-all          # Download Kanto PBF + import + extract venues (~15 min)
make greenery         # Compute greenery scores for 2.3M road edges (~10 min)
make weather          # Fetch hourly weather grid from Open-Meteo
make plateau-shadow   # Precompute building shadows (Docker, Python)
```

## Frontend (web/)

SvelteKit 2 + Svelte 5 (runes syntax: `$state`, `$derived`, `$effect`), TypeScript, Tailwind CSS 4, MapLibre GL JS. Vite proxies `/api` → `:8080`, `/tiles` → `:3000`, `/nominatim` → Nominatim geocoder.

Key stores in `src/lib/stores/`: `map.ts` (map instance, overlay, departure time, route displays), `discovery.ts` (route list, filters).

Map uses CARTO Dark raster basemap + Martin vector tile overlays. Route condition gradients via MapLibre `line-gradient` with color LUT functions in `src/lib/utils/conditionColors.ts`.

## API Endpoints (internal/api/router.go)

**Public:** GET routes, GET discover/nearby|viewport|suggested, GET venues/tags, POST auth/register|login|refresh, GET weather/point, POST routing/directions, GET routes/:id/conditions

**Authenticated:** POST contributions, POST reviews, POST ride-logs, POST plans, POST plans/:id/stops|tasks

## Design Spec

Full design document at `docs/specs/2026-04-10-cyclist-map-design.md` — covers all bounded contexts, data model, API design, speed model, environment-aware routing, and frontend architecture.
