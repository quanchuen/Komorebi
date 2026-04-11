MIGRATE_URL ?= postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable

OSM_PBF     := pipelines/osm_import/kanto-latest.osm.pbf
OSM_URL     := https://download.geofabrik.de/asia/japan/kanto-latest.osm.pbf
OSM_LUA     := pipelines/osm_import/kanto.lua
OSM_DB      := $(MIGRATE_URL)

DEV_JWT_SECRET := cyclist-map-dev-secret-do-not-use-in-production
DATABASE_URL   ?= $(MIGRATE_URL)
API_PORT       ?= 8080

.PHONY: migrate-up migrate-down migrate-create osm-download osm-import osm-update osm-venues osm-all greenery plateau-shadow weather dev-run dev-api dev-martin dev-valhalla dev-web help

migrate-up:
	migrate -path migrations -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

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
plateau-shadow:
	docker compose --profile pipelines run --rm plateau_shadow \
	    --db-url "$(MIGRATE_URL)" \
	    --wards chiyoda,minato,shibuya \
	    --months 1,4,7,10

## Start all dev services (API + Martin + Valhalla + Web) — Ctrl-C stops all
dev-run:
	docker compose up valhalla -d
	martin --config martin.yaml &
	JWT_SECRET=$(DEV_JWT_SECRET) DATABASE_URL="$(DATABASE_URL)" PORT=$(API_PORT) go run ./cmd/api &
	cd web && npm run dev

help:
	@grep -E '^## ' Makefile | sed 's/^## //'
