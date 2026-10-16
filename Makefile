.DEFAULT_GOAL := help

# One database variable for everything: migrations, pipelines, and the API.
DATABASE_URL ?= postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable
MIGRATE_URL  ?= $(DATABASE_URL)

OSM_PBF     := pipelines/osm_import/kanto-latest.osm.pbf
OSM_URL     := https://download.geofabrik.de/asia/japan/kanto-latest.osm.pbf
OSM_LUA     := pipelines/osm_import/kanto.lua
OSM_DB      := $(DATABASE_URL)

DEV_JWT_SECRET := cyclist-map-dev-secret-do-not-use-in-production
API_PORT       ?= 8080
COMPOSE        ?= docker compose

# Published images (docker-bake.hcl). The branch picks the channel: main/master
# publish to <registry>/release/* (cosign-signed), everything else to
# <registry>/develop/*. Nothing is pushed automatically — `make image-push`.
IMAGE_REGISTRY ?= ghcr.io/quanchuen/komorebi
GIT_BRANCH     ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_COMMIT     := $(shell git rev-parse HEAD 2>/dev/null)
GIT_SHORT      := $(shell git rev-parse --short=12 HEAD 2>/dev/null)
GIT_DIRTY      := $(if $(shell git status --porcelain 2>/dev/null),-dirty)
GIT_CREATED    := $(shell git log -1 --format=%cI 2>/dev/null)
IMAGE_VERSION  := $(shell git describe --tags --always --abbrev=12 2>/dev/null)$(GIT_DIRTY)
IMAGE_CHANNEL  := $(if $(filter main master,$(GIT_BRANCH)),release,develop)
BRANCH_SLUG    := $(shell printf '%s' '$(GIT_BRANCH)' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9._-]/-/g')
# Immutable per-commit tag plus one moving tag (latest / branch name).
IMAGE_TAGS     := $(GIT_SHORT)$(GIT_DIRTY),$(if $(filter release,$(IMAGE_CHANNEL)),latest,$(BRANCH_SLUG))
IMAGE_METADATA := .watch/image-metadata.json
BAKE = REGISTRY=$(IMAGE_REGISTRY) CHANNEL=$(IMAGE_CHANNEL) TAGS=$(IMAGE_TAGS) \
	VERSION=$(IMAGE_VERSION) COMMIT=$(GIT_COMMIT) CREATED=$(GIT_CREATED) \
	REF_NAME=$(GIT_BRANCH) docker buildx bake -f docker-bake.hcl
# Keyless (Sigstore OIDC) by default; set COSIGN_KEY to sign with a key instead.
COSIGN     ?= cosign
COSIGN_KEY ?=

# Secret scanner used by the pre-commit hook and `make secrets-audit`.
# Pinned; bump deliberately. `go install` verifies the source against
# sum.golang.org, so no opaque release binary is trusted.
GITLEAKS_VERSION := v8.30.1
GITLEAKS         ?= $(shell command -v gitleaks 2>/dev/null || echo $${GOPATH:-$$HOME/go}/bin/gitleaks)

.PHONY: hooks gitleaks-install secrets-audit
.PHONY: watch api-rebuild image-print image-build image-push
.PHONY: migrate-up migrate-down migrate-create osm-download osm-import osm-venues osm-all greenery plateau-shadow weather support-up support-stop project-up project-stop stack-up stack-down compose-config dev-run dev-api dev-martin dev-valhalla dev-web test test-unit test-integration test-web test-all help

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
	DATABASE_URL="$(DATABASE_URL)" go -C api run ./pipelines/weather_fetch/

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

## Run the full stack under `docker compose watch`: web syncs into a Vite dev server (HMR), api rebuilds on commit
watch:
	WEB_TARGET=dev $(COMPOSE) watch

## Rebuild the api container of a running `make watch` now (the git hooks do this on commit)
api-rebuild:
	@date > .watch/api.rebuild; echo 'stamped .watch/api.rebuild'

## Show the resolved image build (channel, tags, labels) without building
image-print:
	$(BAKE) --print

## Build linux/amd64 + linux/arm64 api and web images into the local image store (needs the containerd image store)
image-build:
	$(BAKE) --load

## Build and push the images to ghcr.io; main/master go to release/ and are cosign-signed, other branches to develop/
image-push:
ifeq ($(IMAGE_CHANNEL),release)
	@test -z "$(GIT_DIRTY)" || { echo 'refusing to publish a release image from a dirty working tree'; exit 1; }
	@command -v $(COSIGN) >/dev/null || { echo 'cosign is required to publish release images'; exit 1; }
endif
	$(BAKE) --push --metadata-file $(IMAGE_METADATA)
ifeq ($(IMAGE_CHANNEL),release)
	@set -eu; for svc in api web; do \
		digest=$$(jq -er --arg t $$svc '.[$$t]["containerimage.digest"]' $(IMAGE_METADATA)); \
		echo "cosign sign $(IMAGE_REGISTRY)/release/$$svc@$$digest"; \
		$(COSIGN) sign --yes $(if $(COSIGN_KEY),--key $(COSIGN_KEY)) \
			-a commit=$(GIT_COMMIT) -a version=$(IMAGE_VERSION) \
			"$(IMAGE_REGISTRY)/release/$$svc@$$digest"; \
	done
endif

## Validate the Compose file (including the pipelines profile) without starting containers
compose-config:
	$(COMPOSE) --profile pipelines config --quiet

## API and web both on the host; Martin and Valhalla run as containers. Ctrl-C stops the host processes
dev-run: support-up
	@set -eu; \
	api_pid=''; web_pid=''; \
	trap 'test -z "$$api_pid" || kill "$$api_pid" 2>/dev/null || true; test -z "$$web_pid" || kill "$$web_pid" 2>/dev/null || true' INT TERM EXIT; \
	JWT_SECRET=$(DEV_JWT_SECRET) DATABASE_URL="$(DATABASE_URL)" PORT=$(API_PORT) go -C api run ./cmd/api & api_pid=$$!; \
	(cd web && npm run dev) & web_pid=$$!; \
	wait

## API on the host (go run); web, Martin and Valhalla run as containers
dev-api: support-up
	WEB_API_URL=http://host.docker.internal:$(API_PORT) $(COMPOSE) up -d --build --no-deps web
	@echo 'web container is on http://localhost:3001 (proxying /api to this host API)'
	JWT_SECRET=$(DEV_JWT_SECRET) DATABASE_URL="$(DATABASE_URL)" PORT=$(API_PORT) go -C api run ./cmd/api

## Web on the host (Vite + HMR); API, Martin and Valhalla run as containers
dev-web: support-up
	$(COMPOSE) up -d --build api
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
	go -C api test ./cmd/... ./internal/domain/... ./internal/app/... ./internal/api/... \
		./internal/infra/valhalla/... ./internal/infra/openmeteo/... \
		./internal/infra/openweathermap/... ./internal/infra/tomorrowio/...

## PostGIS repository tests; requires an explicit TEST_DB_DSN
test-integration:
	@test -n "$(TEST_DB_DSN)" || { echo 'TEST_DB_DSN is required'; exit 2; }
	TEST_DB_DSN="$(TEST_DB_DSN)" go -C api test -v ./internal/infra/postgres/...

## Frontend type, lint, formatting, token, and production-build checks
test-web:
	cd web && npm run check && npm run lint && npm run format:check && npm run check:tokens && npm run test:unit && npm run build

## Run unit, integration, and web checks
test-all: test-unit test-integration test-web

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
