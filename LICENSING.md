# Licensing

Komorebi is **open-core**. Two licenses apply, split by directory.

## Apache License 2.0 — the "Komorebi Engine"

The reusable backend library: domain model, application services, infrastructure
adapters, and data pipelines. You may embed these in your own products,
including proprietary ones, under the terms of [`LICENSE-APACHE`](LICENSE-APACHE).

| Path | Contents |
|------|----------|
| `internal/domain/` | Pure domain types and interfaces (route, community, environment, plan, discovery, events) |
| `internal/app/` | Application services (RouteService, RoutingService, EnvironmentService, AuthService, …) |
| `internal/infra/` | Adapters: `postgres/`, `valhalla/`, `openmeteo/`, `openweathermap/`, `tomorrowio/` |
| `pipelines/` | OSM import, greenery scoring, weather fetch, PLATEAU shadow precompute |
| `migrations/` | SQL schema migrations backing the postgres adapter |

Each of these directories also carries a short `LICENSE` file pointing here.

## GNU Affero General Public License v3.0 — the "Komorebi App"

The deployable application: HTTP API server and the web frontend. If you run a
modified version as a network service, AGPL §13 requires you to offer users the
corresponding source. See [`LICENSE`](LICENSE).

| Path | Contents |
|------|----------|
| `cmd/` | API server entrypoint |
| `internal/api/` | chi router and HTTP handlers |
| `web/` | SvelteKit 2 + MapLibre frontend |
| everything else at the repo root | build scripts, configs, docs |

## The dependency rule

The hexagonal architecture keeps the boundary clean and **it must stay that way**:

- AGPL code (`cmd/`, `internal/api/`, `web/`) may import Apache code freely.
- Apache code (`internal/domain/`, `internal/app/`, `internal/infra/`, `pipelines/`)
  **must not** import anything under `cmd/` or `internal/api/`. Doing so would
  pull AGPL obligations into the engine.

`internal/domain` imports nothing external; `internal/app` and `internal/infra`
depend only on the domain layer and third-party libraries.

## Third-party dependencies

All bundled dependencies are under permissive licenses (BSD/MIT) and are
compatible with both Apache-2.0 and AGPL-3.0. Valhalla (routing) and Martin
(tiles) run as separate processes invoked over HTTP — they are not linked into
this codebase and impose no licensing obligations on it.

## Commercial licensing

Because all contributions are made under the [CLA](CLA.md), the copyright holder
can offer the AGPL portions under separate commercial terms. Contact the
maintainer if AGPL §13 is incompatible with your deployment.

## Contributing

By contributing you agree to the [Contributor License Agreement](CLA.md). See
[`CONTRIBUTING.md`](CONTRIBUTING.md).
