# Komorebi

## A word

This is a weekend learning project born from pain points I shared with my friends. From A->B there are so many routes, we have some favorite stops, quiet backstreets, green wave lights, our favorite bridge, and completely subjective personal energy cost, but they are often overlooked by mainstream map apps, even the best & paid ones doesn't give the flexibility in routing.

I started this project because I suspected the building blocks were already there, and an LLM could stitch them together   into a *basic* PoC of this quite complex project. What I want is a proof that an LLM is able to deal with complex engineering problems. And what I got is something with potential but still rough.

Although it only took about two days, prompting exhausting, I still had to steer the model in the right direction which took constant watching, reviewing, and intervening. Domain Driven Design patterns did help a lot, but working with the details can still be a very frustrating process if you cannot settle for the crude vibe coding quality.

Please see my upcoming blog post on the whole process of taming AI into doing this.

## Disclaimer

This is a weekend hack, not a vetted navigation product. Routes come out of an automated router on top of OSM data and may send you down roads that are private, restricted, closed, unsafe, illegal for bicycles, or simply don't exist on the ground anymore. Greenery, shade, weather, and green-wave scoring are all best-effort estimates — treat them as hints, not facts.

Use your own judgment. Obey local traffic laws, posted signs, and physical reality over anything this app tells you. If a route looks wrong, it probably is — turn around. I take no responsibility for where you end up.

## Introduction

Environment-aware cycling route platform — routes you through shade, greenery, and good weather windows. Tokyo seed.

> *Komorebi* (木漏れ日) — the play of sunlight filtering through leaves.

An open-source, self-hosted route discovery and planning service for recreational and touring cyclists. Users browse curated routes, contribute their own, and plan rides with routing that scores segments by **shade, greenery, wind, rain, UV, traffic signals, and green-wave (グリーンウェーブ) corridors** at the rider's projected arrival time — not just the current weather at the start.

Initial coverage is the Kanto / Tokyo region (OSM data + PLATEAU 3D building shadows). The architecture is designed to extend to any city with OSM coverage and an hourly weather feed.

> **Status:** pre-launch / greenfield. No public deployment. The full design is in [`docs/specs/2026-04-10-cyclist-map-design.md`](docs/specs/2026-04-10-cyclist-map-design.md).

## Tech stack

| Layer | Stack |
|-------|-------|
| Backend | Go 1.25, [chi](https://github.com/go-chi/chi) v5, [pgx](https://github.com/jackc/pgx) v5, [paulmach/orb](https://github.com/paulmach/orb), `golang-jwt` v5 |
| Frontend | SvelteKit 2 + Svelte 5 (runes), TypeScript, Tailwind CSS 4, Vite, [MapLibre GL JS](https://maplibre.org/) 5 |
| Database | PostgreSQL 18 + PostGIS 3.6 (schemas: `routes`, `community`, `environment`, `plan`, `osm`) |
| Routing | [Valhalla](https://github.com/valhalla/valhalla) with custom bicycle costing |
| Tiles | [Martin](https://martin.maplibre.org/) v0.14 vector tile server |
| Data | [osm2pgsql](https://osm2pgsql.org/) flex (Lua), [Open-Meteo](https://open-meteo.com/), Geofabrik OSM PBF, [PLATEAU](https://www.mlit.go.jp/plateau/) CityGML |

## Repo layout

| Path | Purpose |
|------|---------|
| `cmd/api/` | Go API server entrypoint |
| `internal/api/` | Chi router + HTTP handlers |
| `internal/app/` | Application services (RouteService, RoutingService, AuthService, …) |
| `internal/domain/` | Pure domain types and interfaces — no external imports |
| `internal/infra/` | Adapters: `postgres/`, `valhalla/`, `openmeteo/`, `tomorrowio/`, `openweathermap/` |
| `web/` | SvelteKit 2 + MapLibre frontend |
| `migrations/` | golang-migrate SQL files (`000001`–`000021`) |
| `pipelines/osm_import/` | OSM PBF download and `osm2pgsql` flex import (`kanto.lua`) |
| `pipelines/greenery/` | Per-edge greenery scoring SQL |
| `pipelines/weather_fetch/` | Open-Meteo poller (Go) |
| `pipelines/plateau_shadow/` | PLATEAU 3D shadow precompute (Python, Docker) |
| `docs/specs/` | Design documents |

Hexagonal DDD with five bounded contexts: `route`, `community`, `environment`, `plan`, `discovery`. See [`CLAUDE.md`](CLAUDE.md) for architecture notes.

## Prerequisites

Install before bootstrapping:

- Go 1.25+
- Node.js 20+
- Docker + `docker compose`
- PostgreSQL 18 with PostGIS 3.6, reachable on `localhost:5432` (Postgres is **not** in `docker-compose.yml` — bring your own, e.g. via Homebrew or a separate container)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI
- `osm2pgsql` 1.10+
- [`martin`](https://martin.maplibre.org/installation.html) CLI on `$PATH`
- `psql`, `wget`

## Bootstrap

A fresh clone to a running stack. The Makefile is the source of truth for every command below.

**1. Create the database and role**

```bash
createuser -s osm_dev
createdb -O osm_dev cyclist_map_dev
psql -d cyclist_map_dev -c 'CREATE EXTENSION postgis;'
```

The default connection string used everywhere is `postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable`. Override with `MIGRATE_URL` / `DATABASE_URL` if your local setup differs.

**2. Apply migrations**

```bash
make migrate-up
```

**3. Import OSM data (Kanto, ~15 min)**

```bash
make osm-all          # download Kanto PBF → osm2pgsql import → extract venues
```

Subtargets if you want them individually: `make osm-download`, `make osm-import`, `make osm-update` (incremental), `make osm-venues`.

**4. Compute greenery scores (~5–15 min)**

```bash
make greenery
```

**5. Seed weather (optional, repeatable)**

```bash
make weather          # hourly Open-Meteo grid for the Tokyo area
```

**6. Precompute building shadows (optional, requires Docker)**

```bash
make plateau-shadow   # PLATEAU CityGML → shadow masks for chiyoda/minato/shibuya, months 1/4/7/10
```

**7. Install web dependencies**

```bash
cd web && npm install && cd ..
```

**8. Run the full dev stack**

```bash
make dev-run
```

This starts Valhalla via `docker compose`, then `martin`, the Go API, and `vite dev` in the foreground. Ctrl-C tears them all down.

| Service | URL |
|---------|-----|
| Web (Vite) | http://localhost:5173 |
| API | http://localhost:8080 |
| Martin tiles | http://localhost:3000 |
| Valhalla | http://localhost:8002 |

## Environment variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `DATABASE_URL` | yes | — | PostGIS connection string |
| `JWT_SECRET` | yes | — | HS256 JWT signing key |
| `PORT` | no | `8080` | API listen port |
| `VALHALLA_URL` | no | `http://localhost:8002` | Routing engine endpoint |
| `WEATHER_PROVIDER` | no | `open-meteo` | `open-meteo`, `tomorrow-io`, or `openweathermap` |
| `WEATHER_API_KEY` | for paid providers | — | Tomorrow.io / OpenWeatherMap key |

`make dev-run` injects a hardcoded dev `JWT_SECRET` (`cyclist-map-dev-secret-do-not-use-in-production`). Do not reuse it in any deployed environment.

## Running services individually

```bash
JWT_SECRET=cyclist-map-dev-secret-do-not-use-in-production \
  DATABASE_URL="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go run ./cmd/api                          # API on :8080

martin --config martin.yaml                 # Vector tiles on :3000
docker compose up valhalla                  # Routing on :8002
cd web && npm run dev                       # SvelteKit on :5173
```

## Testing

```bash
go test ./...                               # All Go tests
go test -v ./internal/infra/postgres        # Postgres integration tests
cd web && npm run check                     # Svelte type checking
```

The `internal/infra/postgres` integration tests connect to the real DB via `TEST_DB_DSN` (falling back to the default DSN) and skip gracefully when no database is reachable. Test stubs are hand-written — no mocking library.

## Build

```bash
go build ./cmd/api
cd web && npm run build
```

## Further reading

- [`docs/specs/2026-04-10-cyclist-map-design.md`](docs/specs/2026-04-10-cyclist-map-design.md) — full design: bounded contexts, data model, API surface, speed model, environment-aware routing, frontend architecture
- [`CLAUDE.md`](CLAUDE.md) — architecture summary and conventions
- [`web/README.md`](web/README.md) — SvelteKit defaults

## License

Released into the public domain under [The Unlicense](LICENSE).
