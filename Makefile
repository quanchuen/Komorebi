MIGRATE_URL ?= postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable

OSM_PBF     := pipelines/osm_import/kanto-latest.osm.pbf
OSM_URL     := https://download.geofabrik.de/asia/japan/kanto-latest.osm.pbf
OSM_LUA     := pipelines/osm_import/kanto.lua
OSM_DB      := $(MIGRATE_URL)

.PHONY: migrate-up migrate-down migrate-create osm-download osm-import osm-update osm-venues osm-all help

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
	    --slim \
	    --number-processes=4 \
	    $(OSM_PBF)

## Extract venues from osm.pois into environment.venue
osm-venues:
	psql "$(OSM_DB)" -f pipelines/osm_import/extract_venues.sql

## Run full OSM pipeline: download → import → venues
osm-all: osm-import osm-venues

help:
	@grep -E '^## ' Makefile | sed 's/^## //'
