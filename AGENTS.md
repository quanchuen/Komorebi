# AGENTS.md

Shared instructions for coding agents working in this repository. More specific
instructions are scoped by directory and override this file where applicable.

## Scoped instructions

- `web/AGENTS.md` — SvelteKit, MapLibre, browser storage, and foreground PWA work.
- `web/tests/AGENTS.md` — mandatory UI and layout-test rules. Keep browser UI
  layout tests under `web/tests/` so this policy applies automatically.
- `ios/AGENTS.md` — future native SwiftUI/Core Location application work.

## Build & Run

```bash
# Full dev stack (API + Martin + Valhalla + Web)
make dev-run

# Operationally separated Compose services
make support-up       # external adapters: Martin + Valhalla
make project-up       # repository-owned containers: API + Web
make stack-up         # both profiles

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
make test                                   # Fast tests; no external services
make test-unit                              # Explicit unit/HTTP-adapter boundary
TEST_DB_DSN="$DATABASE_URL" make test-integration
make test-web                               # Frontend checks + build
go test ./internal/app -run TestAuth        # Single test pattern
go test -v ./internal/infra/postgres        # Verbose, one package
cd web && npm run check                     # Svelte type checking
```

Integration tests in `infra/postgres/` connect to the real database. They use `TEST_DB_DSN` or the default connection string and skip gracefully if unreachable. Test stubs are hand-written (no mocking library) — see `testutil_test.go` files for patterns.

Docker Compose has three profiles: `project` (`api`, `web`), `support`
(`martin`, `valhalla`), and `pipelines` (`plateau_shadow`). Project services do
not declare dependencies on support services so they can be developed and tested
independently; use `make stack-up` for the integrated container stack.

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

- **`internal/domain/`** — Pure Go types, interfaces, business rules. Zero external imports. Existing bounded contexts: `route`, `community`, `environment`, `plan`, `discovery`; `exploration` is proposed for user-specific explored-road knowledge. Route Planning owns personalized routing policy. See `docs/adr/`.
- **`internal/app/`** — Application services orchestrating domain objects. Each context has a service (e.g., `RouteService`, `RoutingService`, `AuthService`).
- **`internal/infra/`** — Adapter implementations: `postgres/` (repositories), `valhalla/` (routing client), `openmeteo/` (weather client), `tomorrowio/`, `openweathermap/`, `anthropic/` (route-intent LLM adapter, ADR 0003).
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
| `WEATHER_PROVIDER` | No | `open-meteo` | Live point + minutely provider: `open-meteo`, `tomorrow-io`, `openweathermap` |
| `WEATHER_GRID_PROVIDER` | No | `open-meteo` | Bulk-grid provider for `make weather`; deliberately independent of `WEATHER_PROVIDER` (the ~100-call grid run exceeds paid-tier rate limits) |
| `WEATHER_API_KEY` | For paid providers | — | Tomorrow.io or OWM key |
| `ANTHROPIC_API_KEY` | No | — | Enables natural-language route intent (`POST /routing/intent`); without it the endpoint returns 503 |
| `INTENT_MODEL` | No | `claude-opus-5` | Claude model for the route-intent adapter |

## Data Pipelines

```bash
make osm-all          # Download Kanto PBF + import + extract venues (~15 min)
make greenery         # Compute greenery scores for 2.3M road edges (~10 min)
make weather          # Fetch hourly weather grid from Open-Meteo (recurring; cron-worthy)
make plateau-shadow   # Precompute building shadows (Docker, Python)
```

`plateau-shadow` is a one-time bootstrap, not a recurring job: buildings are
static, so re-run it only for a new PLATEAU edition, additional wards
(`WARDS=kita,toshima make plateau-shadow`), or different representative
months. Downloads are cached under `pipelines/plateau_shadow/data/`. Expect
~30 min per ward plus a ~2 GB download the first time. `weather` is the
opposite: forecast data goes stale within hours, so it should run frequently.

Scheduling and orchestration of these targets (hourly weather, manual
bootstrap runs) live in the separate `~/src/komorebi-workflows` repository
(Temporal); this repo's Makefile remains the single owner of what each
pipeline does.

## API Endpoints (internal/api/router.go)

**Public:** GET routes, GET discover/nearby|viewport|suggested, GET venues/tags, POST auth/register|login|refresh, GET weather/point, POST routing/directions, POST routing/intent, POST routing/conditions, GET routes/:id/conditions

**Authenticated:** POST contributions, POST reviews, POST ride-logs, POST plans, POST plans/:id/stops|tasks

## Design Spec

Full design document at `docs/specs/2026-04-10-cyclist-map-design.md` — covers all bounded contexts, data model, API design, speed model, environment-aware routing, and frontend architecture.
