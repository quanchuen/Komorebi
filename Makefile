.DEFAULT_GOAL := help

MIGRATE_URL ?= postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable

OSM_PBF     := pipelines/osm_import/kanto-latest.osm.pbf
OSM_URL     := https://download.geofabrik.de/asia/japan/kanto-latest.osm.pbf
OSM_LUA     := pipelines/osm_import/kanto.lua
OSM_DB      := $(MIGRATE_URL)

DEV_JWT_SECRET := cyclist-map-dev-secret-do-not-use-in-production
DATABASE_URL   ?= $(MIGRATE_URL)
API_PORT       ?= 8080
COMPOSE        ?= docker compose

# Secret scanner used by the pre-commit hook and `make secrets-audit`.
# Pinned; bump deliberately. `go install` verifies the source against
# sum.golang.org, so no opaque release binary is trusted.
GITLEAKS_VERSION := v8.30.1
GITLEAKS         ?= $(shell command -v gitleaks 2>/dev/null || echo $${GOPATH:-$$HOME/go}/bin/gitleaks)

.PHONY: demo-routes
.PHONY: hooks gitleaks-install secrets-audit
.PHONY: migrate-up migrate-down migrate-create osm-download osm-import osm-update osm-venues osm-all greenery plateau-shadow weather support-up support-stop project-up project-stop stack-up stack-down compose-config dev-run dev-api dev-martin dev-valhalla dev-web test test-unit test-integration test-web test-all web-lint web-lighthouse help

## Apply pending database migrations
migrate-up:
	migrate -path migrations -database "$(MIGRATE_URL)" up

## Roll back one migration
migrate-down:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

## Create a new numbered migration pair (prompts for a name)
migrate-create:
	@read -p "Name: " name; \
	migrate create -ext sql -dir migrations -seq -digits 6 $$name

## Re-seed demo routes via the reroute tool (requires DB + Valhalla)
demo-routes:
	DATABASE_URL="$(DATABASE_URL)" VALHALLA_URL="$${VALHALLA_URL:-http://localhost:8002}" go run ./cmd/reroute-demo

## Download Kanto PBF from Geofabrik
osm-download:
	@mkdir -p pipelines/osm_import
	wget -c -O $(OSM_PBF) $(OSM_URL)

## Full import (drop and recreate osm.* tables)
osm-import: osm-download
	osm2pgsql \
	    --output=flex \
	    --style=$(OSM_LUA) \
	    --database="$(OSM_DB)" \
	    --schema=osm \
	    --slim \
	    --drop \
	    --number-processes=4 \
	    $(OSM_PBF)

## Incremental update using an existing slim database
osm-update: osm-download
	osm2pgsql \
	    --output=flex \
	    --style=$(OSM_LUA) \
	    --database="$(OSM_DB)" \
	    --schema=osm \
	    --slim \
	    --number-processes=4 \
	    $(OSM_PBF)

## Extract venues from osm.pois into environment.venue
osm-venues:
	psql "$(OSM_DB)" -f pipelines/osm_import/extract_venues.sql

## Run full OSM pipeline: download → import → venues
osm-all: osm-import osm-venues

## Populate environment.greenery_edge from osm.roads + osm.landuse (idempotent, ~5–15 min)
greenery:
	psql "$(OSM_DB)" -f pipelines/greenery/compute_greenery.sql

## Fetch hourly weather from Open-Meteo for Tokyo area grid
weather:
	DATABASE_URL="$(MIGRATE_URL)" go run ./pipelines/weather_fetch/

## Run PLATEAU shadow precompute pipeline (requires Docker; uses pipelines profile)
WARDS ?= chiyoda,minato,shibuya
plateau-shadow:
	$(COMPOSE) --profile pipelines run --rm --build plateau_shadow \
	    --wards $(WARDS) \
	    --months 1,4,7,10

## Start supporting services only (Martin + Valhalla); runs pending migrations first
support-up:
	$(COMPOSE) up -d martin valhalla

## Stop supporting services without touching project containers or data
support-stop:
	$(COMPOSE) stop martin valhalla

## Build and start project-owned containers only (API + Web)
project-up:
	$(COMPOSE) up -d --build api web

## Stop project-owned containers without touching supporting services
project-stop:
	$(COMPOSE) stop api web

## Build and start the complete container stack (same as `docker compose up -d --build`)
stack-up:
	$(COMPOSE) up -d --build

## Stop the complete stack; named data volumes are preserved
stack-down:
	$(COMPOSE) down

## Validate the Compose file (including the pipelines profile) without starting containers
compose-config:
	$(COMPOSE) --profile pipelines config --quiet

## Start support containers plus local API and Vite dev servers; Ctrl-C stops local processes
dev-run: support-up
	@set -eu; \
	api_pid=''; web_pid=''; \
	trap 'test -z "$$api_pid" || kill "$$api_pid" 2>/dev/null || true; test -z "$$web_pid" || kill "$$web_pid" 2>/dev/null || true' INT TERM EXIT; \
	JWT_SECRET=$(DEV_JWT_SECRET) DATABASE_URL="$(DATABASE_URL)" PORT=$(API_PORT) go run ./cmd/api & api_pid=$$!; \
	(cd web && npm run dev) & web_pid=$$!; \
	wait

## Start only the local Go API (support services optional)
dev-api:
	JWT_SECRET=$(DEV_JWT_SECRET) DATABASE_URL="$(DATABASE_URL)" PORT=$(API_PORT) go run ./cmd/api

## Start only Vite; API-backed screens require make dev-api in another terminal
dev-web:
	@curl -fsS http://127.0.0.1:$(API_PORT)/api/v1/routes >/dev/null 2>&1 || \
		echo 'warning: API is not reachable on :$(API_PORT); run "make dev-api" or use "make dev-run"'
	cd web && npm run dev

## Start only Martin (plus the one-shot migrate service it depends on)
dev-martin:
	$(COMPOSE) up martin

## Start only Valhalla
dev-valhalla:
	$(COMPOSE) up valhalla

## Fast default: Go tests that do not require Postgres, Martin, or Valhalla processes
test: test-unit

## Go tests for domain/app/api and HTTP adapters only
test-unit:
	go test ./cmd/... ./internal/domain/... ./internal/app/... ./internal/api/... \
		./internal/infra/valhalla/... ./internal/infra/openmeteo/... \
		./internal/infra/openweathermap/... ./internal/infra/tomorrowio/...

## PostGIS repository tests; requires an explicit TEST_DB_DSN
test-integration:
	@test -n "$(TEST_DB_DSN)" || { echo 'TEST_DB_DSN is required'; exit 2; }
	TEST_DB_DSN="$(TEST_DB_DSN)" go test -v ./internal/infra/postgres/...

## Frontend type, lint, formatting, token, and production-build checks
test-web:
	cd web && npm run check && npm run lint && npm run format:check && npm run check:tokens && npm run build

## Run unit, integration, and web checks
test-all: test-unit test-integration test-web

## Lint + format-check + design-token guard for the web app
web-lint:
	cd web && npm run lint && npm run format:check && npm run check:tokens

## Run Lighthouse budget against a production preview of the web app
web-lighthouse:
	cd web && npm run build && npm run lighthouse

## Install the pre-commit hook (secret scan + web lint-staged) and gitleaks
hooks: gitleaks-install
	git config core.hooksPath web/.husky
	@echo "core.hooksPath -> web/.husky (pre-commit: gitleaks + lint-staged)"

## Build the pinned gitleaks version from source if it is not already on PATH
gitleaks-install:
	@if [ -x "$(GITLEAKS)" ]; then echo "gitleaks present: $(GITLEAKS)"; \
	else echo "installing gitleaks $(GITLEAKS_VERSION) via go install"; \
	     go install github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION); fi

## Scan the entire git history for secrets (all refs), using .gitleaks.toml
secrets-audit: gitleaks-install
	"$(GITLEAKS)" git . --config .gitleaks.toml --redact --no-banner

## Show this help
help:
	@printf 'Usage: make <target>\n\nTargets:\n'
	@awk '/^## /{d=substr($$0,4); next} /^[a-zA-Z0-9][a-zA-Z0-9_.-]*:/{if(d){t=$$1; sub(/:.*/,"",t); printf "  \033[36m%-18s\033[0m %s\n", t, d; d=""}} /^$$/{d=""}' Makefile
