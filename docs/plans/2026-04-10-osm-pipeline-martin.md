# OSM Data Pipeline and Martin Tile Server

**Goal:** Import Kanto region OSM data into PostGIS using osm2pgsql, extract venue records into `environment.venue`, and configure Martin to serve vector tiles for roads, cycling infrastructure, venues, and routes.

**Architecture:**

```
Geofabrik PBF download
        │
        ▼
  osm2pgsql import
  (osm schema: planet_osm_point / planet_osm_line / planet_osm_polygon)
        │
        ├─── venue extraction SQL ──▶ environment.venue
        │
        └─── Martin tile server reads all sources
                   │
                   ▼
           MapLibre GL JS (web client)
```

**Tech stack:**

| Component | Tool |
|-----------|------|
| OSM data source | Geofabrik — kanto-latest.osm.pbf |
| OSM import | osm2pgsql 1.x (flex output, Lua style file) |
| Database | PostGIS on localhost:5432 — cyclist_map_dev |
| Tile server | Martin v0.14 (ghcr.io/maplibre/martin) |
| Config | martin.yaml mounted into Martin container |
| Orchestration | docker-compose.yml, Makefile targets |

**Database URL:** `postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable`

---

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute the tasks below. Each task is independent unless a dependency is noted. Run tasks that have no cross-dependencies in parallel.

---

## Task 1: Create osm schema and osm2pgsql Lua style file

**Files:**
- `migrations/000014_osm_schema.up.sql`
- `migrations/000014_osm_schema.down.sql`
- `pipelines/osm_import/kanto.lua`

### Steps

- [ ] Create migration for the `osm` schema (osm2pgsql will create tables within it):

  **migrations/000014_osm_schema.up.sql**
  ```sql
  CREATE SCHEMA IF NOT EXISTS osm;
  ```

  **migrations/000014_osm_schema.down.sql**
  ```sql
  DROP SCHEMA IF EXISTS osm CASCADE;
  ```

- [ ] Create `pipelines/osm_import/` directory and write the Lua style file. The flex output approach gives full control over which tags land in which tables.

  **pipelines/osm_import/kanto.lua**
  ```lua
  -- osm2pgsql flex style for cyclist-map
  -- Targets: Tokyo/Kanto region
  -- Captures cycling-relevant tags for roads, infrastructure, and venues

  local tables = {}

  -- Roads and paths (line geometry)
  tables.roads = osm2pgsql.define_way_table('osm.roads', {
      { column = 'osm_id',       type = 'bigint' },
      { column = 'name',         type = 'text' },
      { column = 'name_en',      type = 'text' },
      { column = 'highway',      type = 'text' },
      { column = 'cycleway',     type = 'text' },
      { column = 'cycleway_left',  type = 'text' },
      { column = 'cycleway_right', type = 'text' },
      { column = 'bicycle',      type = 'text' },
      { column = 'surface',      type = 'text' },
      { column = 'smoothness',   type = 'text' },
      { column = 'maxspeed',     type = 'text' },
      { column = 'oneway',       type = 'text' },
      { column = 'lit',          type = 'text' },
      { column = 'width',        type = 'text' },
      { column = 'lanes',        type = 'text' },
      { column = 'tags',         type = 'jsonb' },
      { column = 'geom',         type = 'linestring', projection = 4326 },
  })

  -- Points of interest: venues, signals, etc. (node geometry)
  tables.pois = osm2pgsql.define_node_table('osm.pois', {
      { column = 'osm_id',    type = 'bigint' },
      { column = 'name',      type = 'text' },
      { column = 'name_en',   type = 'text' },
      { column = 'amenity',   type = 'text' },
      { column = 'shop',      type = 'shop' },
      { column = 'leisure',   type = 'text' },
      { column = 'highway',   type = 'text' },
      { column = 'tourism',   type = 'text' },
      { column = 'brand',     type = 'text' },
      { column = 'operator',  type = 'text' },
      { column = 'opening_hours', type = 'text' },
      { column = 'phone',     type = 'text' },
      { column = 'website',   type = 'text' },
      { column = 'tags',      type = 'jsonb' },
      { column = 'geom',      type = 'point', projection = 4326 },
  })

  -- Land use and green areas (polygon geometry)
  tables.landuse = osm2pgsql.define_area_table('osm.landuse', {
      { column = 'osm_id',    type = 'bigint' },
      { column = 'name',      type = 'text' },
      { column = 'landuse',   type = 'text' },
      { column = 'leisure',   type = 'text' },
      { column = 'natural_',  type = 'text' },
      { column = 'tags',      type = 'jsonb' },
      { column = 'geom',      type = 'geometry', projection = 4326 },
  })

  -- Highway-class filter: only import road types relevant to cycling
  local highway_keep = {
      motorway = false,
      motorway_link = false,
      trunk = true,
      trunk_link = true,
      primary = true,
      primary_link = true,
      secondary = true,
      secondary_link = true,
      tertiary = true,
      tertiary_link = true,
      unclassified = true,
      residential = true,
      service = true,
      living_street = true,
      pedestrian = true,
      track = true,
      path = true,
      cycleway = true,
      footway = true,
      steps = false,
      construction = false,
  }

  -- Amenity/shop types to capture as POIs
  local poi_amenity = {
      cafe = true,
      restaurant = true,
      fast_food = true,
      toilets = true,
      drinking_water = true,
      vending_machine = true,
      atm = true,
      bicycle_parking = true,
      bicycle_repair_station = true,
      place_of_worship = true,
      rest_area = true,
  }

  local poi_shop = {
      convenience = true,
      supermarket = true,
      bicycle = true,
  }

  local poi_leisure = {
      park = true,
      hot_spring = true,
  }

  local poi_highway = {
      traffic_signals = true,
      rest_area = true,
  }

  -- Ways → roads table
  function osm2pgsql.process_way(object)
      local hw = object.tags.highway
      if hw == nil then return end
      if highway_keep[hw] == false then return end
      if highway_keep[hw] == nil then return end

      local row = {
          osm_id         = object.id,
          name           = object.tags.name,
          name_en        = object.tags['name:en'],
          highway        = hw,
          cycleway       = object.tags.cycleway,
          cycleway_left  = object.tags['cycleway:left'],
          cycleway_right = object.tags['cycleway:right'],
          bicycle        = object.tags.bicycle,
          surface        = object.tags.surface,
          smoothness     = object.tags.smoothness,
          maxspeed       = object.tags.maxspeed,
          oneway         = object.tags.oneway,
          lit            = object.tags.lit,
          width          = object.tags.width,
          lanes          = object.tags.lanes,
          tags           = object.tags,
          geom           = { create = 'line' },
      }
      tables.roads:insert(row)
  end

  -- Nodes → pois table
  function osm2pgsql.process_node(object)
      local t = object.tags
      local keep = false

      if t.amenity   and poi_amenity[t.amenity]   then keep = true end
      if t.shop      and poi_shop[t.shop]          then keep = true end
      if t.leisure   and poi_leisure[t.leisure]    then keep = true end
      if t.highway   and poi_highway[t.highway]    then keep = true end

      if not keep then return end

      local row = {
          osm_id        = object.id,
          name          = t.name,
          name_en       = t['name:en'],
          amenity       = t.amenity,
          shop          = t.shop,
          leisure       = t.leisure,
          highway       = t.highway,
          tourism       = t.tourism,
          brand         = t.brand,
          operator      = t.operator,
          opening_hours = t.opening_hours,
          phone         = t.phone,
          website       = t.website,
          tags          = t,
          geom          = { create = 'point' },
      }
      tables.pois:insert(row)
  end

  -- Relations/areas → landuse table
  function osm2pgsql.process_relation(object)
      local t = object.tags
      if t.type ~= 'multipolygon' then return end

      local keep = false
      if t.landuse and (t.landuse == 'forest' or t.landuse == 'grass'
                        or t.landuse == 'meadow' or t.landuse == 'farmland') then
          keep = true
      end
      if t.leisure == 'park' or t.leisure == 'nature_reserve' then keep = true end
      if t.natural then keep = true end

      if not keep then return end

      tables.landuse:insert({
          osm_id  = object.id,
          name    = t.name,
          landuse = t.landuse,
          leisure = t.leisure,
          natural_ = t.natural,
          tags    = t,
          geom    = { create = 'area' },
      })
  end
  ```

  Note on the `shop` column type — the column name in the table definition uses `'shop'` as the column name; the Lua type for text columns is `'text'`. Fix the table definition so `shop` is `type = 'text'` (not `'shop'`).

  **Corrected pois table definition** (replace the shop line):
  ```lua
      { column = 'shop',      type = 'text' },
  ```

- [ ] Apply the migration:
  ```bash
  make migrate-up
  ```

- [ ] Commit:
  ```bash
  git add migrations/000014_osm_schema.up.sql migrations/000014_osm_schema.down.sql pipelines/osm_import/kanto.lua
  git commit -m "Add osm schema migration and osm2pgsql Lua style file for Kanto import"
  ```

---

## Task 2: Add Makefile targets for OSM download and import

**Files:**
- `Makefile`

**Dependency:** Task 1 (schema migration must exist before import runs)

### Steps

- [ ] Add the following targets to `Makefile`. The PBF is downloaded to `pipelines/osm_import/` which is gitignored.

  ```makefile
  OSM_PBF     := pipelines/osm_import/kanto-latest.osm.pbf
  OSM_URL     := https://download.geofabrik.de/asia/japan/kanto-latest.osm.pbf
  OSM_LUA     := pipelines/osm_import/kanto.lua
  OSM_DB      := $(MIGRATE_URL)

  .PHONY: osm-download osm-import osm-update osm-venues

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
  ```

- [ ] Add `pipelines/osm_import/*.osm.pbf` to `.gitignore` (create `.gitignore` if it doesn't exist):
  ```
  pipelines/osm_import/*.osm.pbf
  pipelines/osm_import/*.osm.pbf.md5
  ```

- [ ] Commit:
  ```bash
  git add Makefile .gitignore
  git commit -m "Add Makefile targets for OSM download, import, incremental update, and venue extraction"
  ```

---

## Task 3: Write venue extraction SQL

**Files:**
- `pipelines/osm_import/extract_venues.sql`

**Dependency:** Task 1 (osm.pois table must exist), Task 2 (Makefile target references this file)

### Steps

- [ ] Create `pipelines/osm_import/extract_venues.sql`. This populates `environment.venue` from `osm.pois` using a deterministic upsert keyed on `osm_id`.

  ```sql
  -- extract_venues.sql
  -- Populates environment.venue from osm.pois after an osm2pgsql import.
  -- Safe to re-run: uses INSERT ... ON CONFLICT DO UPDATE.

  INSERT INTO environment.venue (osm_id, geometry, name, category, brand, osm_tags)
  SELECT
      p.osm_id,
      p.geom                                          AS geometry,

      COALESCE(p.name, p.brand, p.operator, 'unnamed') AS name,

      -- Derive a single category string from OSM tags
      CASE
          WHEN p.shop      = 'convenience'        THEN 'konbini'
          WHEN p.shop      = 'supermarket'        THEN 'supermarket'
          WHEN p.shop      = 'bicycle'            THEN 'bike-shop'
          WHEN p.amenity   = 'cafe'               THEN 'cafe'
          WHEN p.amenity   = 'restaurant'         THEN 'restaurant'
          WHEN p.amenity   = 'fast_food'          THEN 'fast-food'
          WHEN p.amenity   = 'toilets'            THEN 'toilet'
          WHEN p.amenity   = 'drinking_water'     THEN 'water'
          WHEN p.amenity   = 'vending_machine'    THEN 'vending'
          WHEN p.amenity   = 'atm'                THEN 'atm'
          WHEN p.amenity   = 'bicycle_parking'    THEN 'bicycle-parking'
          WHEN p.amenity   = 'bicycle_repair_station' THEN 'bike-repair'
          WHEN p.amenity   = 'place_of_worship'   THEN 'worship'
          WHEN p.amenity   = 'rest_area'          THEN 'rest-area'
          WHEN p.leisure   = 'park'               THEN 'park'
          WHEN p.leisure   = 'hot_spring'         THEN 'onsen'
          WHEN p.highway   = 'traffic_signals'    THEN 'traffic-signal'
          ELSE 'other'
      END                                             AS category,

      p.brand                                         AS brand,
      p.tags                                          AS osm_tags

  FROM osm.pois p
  WHERE
      -- Exclude traffic signals from venue table (handled by environment.traffic_signal)
      p.highway IS DISTINCT FROM 'traffic_signals'

  ON CONFLICT (osm_id) DO UPDATE
      SET
          geometry  = EXCLUDED.geometry,
          name      = EXCLUDED.name,
          category  = EXCLUDED.category,
          brand     = EXCLUDED.brand,
          osm_tags  = EXCLUDED.osm_tags;

  -- Also populate environment.traffic_signal from osm.pois
  INSERT INTO environment.traffic_signal (osm_node_id, geometry)
  SELECT
      p.osm_id,
      p.geom
  FROM osm.pois p
  WHERE p.highway = 'traffic_signals'
  ON CONFLICT (osm_node_id) DO UPDATE
      SET geometry = EXCLUDED.geometry;

  -- Report counts
  SELECT
      (SELECT COUNT(*) FROM environment.venue)          AS venue_count,
      (SELECT COUNT(*) FROM environment.traffic_signal) AS signal_count;
  ```

  Note: `environment.venue` needs a unique constraint on `osm_id` for the upsert. Check migration 000010 to confirm it exists; if not, add a migration:

  **migrations/000015_venue_osm_id_unique.up.sql**
  ```sql
  ALTER TABLE environment.venue
      ADD CONSTRAINT venue_osm_id_unique UNIQUE (osm_id);
  ```

  **migrations/000015_venue_osm_id_unique.down.sql**
  ```sql
  ALTER TABLE environment.venue
      DROP CONSTRAINT IF EXISTS venue_osm_id_unique;
  ```

  Similarly for `environment.traffic_signal` on `osm_node_id`:

  **migrations/000016_traffic_signal_pk.up.sql**
  ```sql
  -- osm_node_id should be the primary key; add unique if not already enforced
  ALTER TABLE environment.traffic_signal
      ADD CONSTRAINT traffic_signal_osm_node_id_unique UNIQUE (osm_node_id);
  ```

  **migrations/000016_traffic_signal_pk.down.sql**
  ```sql
  ALTER TABLE environment.traffic_signal
      DROP CONSTRAINT IF EXISTS traffic_signal_osm_node_id_unique;
  ```

  Apply migrations before testing:
  ```bash
  make migrate-up
  ```

- [ ] Commit:
  ```bash
  git add pipelines/osm_import/extract_venues.sql \
      migrations/000015_venue_osm_id_unique.up.sql \
      migrations/000015_venue_osm_id_unique.down.sql \
      migrations/000016_traffic_signal_pk.up.sql \
      migrations/000016_traffic_signal_pk.down.sql
  git commit -m "Add venue extraction SQL and unique constraint migrations"
  ```

---

## Task 4: Write Martin configuration file

**Files:**
- `martin.yaml`

**Dependency:** Tasks 1–3 must be complete so the tables Martin references actually exist.

### Steps

- [ ] Create `martin.yaml` at the project root. Martin reads this at startup and serves each entry as a vector tile source.

  ```yaml
  # martin.yaml — Vector tile sources for cyclist-map
  # Served by Martin v0.14 at http://localhost:3000

  listen_addresses:
    - "0.0.0.0:3000"

  # Postgres connection (overridden by DATABASE_URL env var in docker-compose)
  postgres:
    connection_string: "postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable"
    pool_size: 10

    # ---- OSM sources ----

    tables:

      # All roads (zoom 8–18)
      osm_roads:
        schema: osm
        table: roads
        srid: 4326
        geometry_column: geom
        geometry_type: LINESTRING
        minzoom: 8
        maxzoom: 18
        properties:
          - osm_id
          - name
          - name_en
          - highway
          - cycleway
          - cycleway_left
          - cycleway_right
          - bicycle
          - surface
          - smoothness
          - maxspeed
          - oneway
          - lit

      # Dedicated cycling infrastructure layer (cycleway, path, track — zoom 10+)
      osm_cycling:
        schema: osm
        table: roads
        srid: 4326
        geometry_column: geom
        geometry_type: LINESTRING
        minzoom: 10
        maxzoom: 18
        # Filter to cycling-specific highway types and ways with cycleway tags
        # Martin table sources don't support SQL WHERE; use a view instead — see below.
        # This source is replaced by the osm_cycling_view function source below.

      # Venues / POIs (zoom 12+)
      osm_pois:
        schema: osm
        table: pois
        srid: 4326
        geometry_column: geom
        geometry_type: POINT
        minzoom: 12
        maxzoom: 18
        properties:
          - osm_id
          - name
          - name_en
          - amenity
          - shop
          - leisure
          - highway
          - brand
          - opening_hours

      # Landuse / greenery polygons (zoom 10+)
      osm_landuse:
        schema: osm
        table: landuse
        srid: 4326
        geometry_column: geom
        geometry_type: GEOMETRY
        minzoom: 10
        maxzoom: 18
        properties:
          - osm_id
          - name
          - landuse
          - leisure
          - natural_

      # ---- Application sources ----

      # Curated routes (zoom 8+)
      routes:
        schema: routes
        table: route
        srid: 4326
        geometry_column: geometry
        geometry_type: LINESTRING
        minzoom: 8
        maxzoom: 18
        properties:
          - id
          - name
          - distance_m
          - elevation_gain_m
          - difficulty
          - status

      # Enriched venues from environment schema (zoom 12+)
      venues:
        schema: environment
        table: venue
        srid: 4326
        geometry_column: geometry
        geometry_type: POINT
        minzoom: 12
        maxzoom: 18
        properties:
          - id
          - osm_id
          - name
          - category
          - brand

      # Traffic signals (zoom 14+)
      traffic_signals:
        schema: environment
        table: traffic_signal
        srid: 4326
        geometry_column: geometry
        geometry_type: POINT
        minzoom: 14
        maxzoom: 18
        properties:
          - osm_node_id

    # ---- Function sources (filtered views) ----
    # These replace the osm_cycling placeholder above.

    functions:

      osm_cycling:
        schema: public
        function: osm_cycling_tiles
  ```

- [ ] Create the `osm_cycling_tiles` function that Martin will call for filtered cycling infrastructure tiles. Add a migration:

  **migrations/000017_osm_cycling_tiles_fn.up.sql**
  ```sql
  CREATE OR REPLACE FUNCTION public.osm_cycling_tiles(z integer, x integer, y integer)
  RETURNS bytea AS $$
  DECLARE
      tile_bounds geometry;
      result      bytea;
  BEGIN
      tile_bounds := ST_TileEnvelope(z, x, y);

      SELECT INTO result
          ST_AsMVT(q, 'osm_cycling', 4096, 'mvt_geom')
      FROM (
          SELECT
              osm_id,
              name,
              name_en,
              highway,
              cycleway,
              cycleway_left,
              cycleway_right,
              bicycle,
              surface,
              smoothness,
              oneway,
              lit,
              ST_AsMVTGeom(
                  ST_Transform(geom, 3857),
                  ST_Transform(tile_bounds, 3857),
                  4096, 64, true
              ) AS mvt_geom
          FROM osm.roads
          WHERE
              geom && tile_bounds
              AND (
                  highway IN ('cycleway', 'path', 'track', 'footway', 'pedestrian')
                  OR cycleway IS NOT NULL
                  OR cycleway_left IS NOT NULL
                  OR cycleway_right IS NOT NULL
                  OR bicycle IN ('yes', 'designated', 'permissive')
              )
      ) q
      WHERE mvt_geom IS NOT NULL;

      RETURN result;
  END;
  $$ LANGUAGE plpgsql STABLE;
  ```

  **migrations/000017_osm_cycling_tiles_fn.down.sql**
  ```sql
  DROP FUNCTION IF EXISTS public.osm_cycling_tiles(integer, integer, integer);
  ```

  Apply:
  ```bash
  make migrate-up
  ```

- [ ] Commit:
  ```bash
  git add martin.yaml \
      migrations/000017_osm_cycling_tiles_fn.up.sql \
      migrations/000017_osm_cycling_tiles_fn.down.sql
  git commit -m "Add Martin config and osm_cycling_tiles MVT function"
  ```

---

## Task 5: Update docker-compose.yml to mount martin.yaml

**Files:**
- `docker-compose.yml`

**Dependency:** Task 4 (martin.yaml must exist)

### Steps

- [ ] Update the `martin` service in `docker-compose.yml` to mount `martin.yaml` and pass it as the config argument. The `DATABASE_URL` env var is kept so Martin can override the connection string if needed, but the config file takes precedence for source definitions.

  Current martin service:
  ```yaml
    martin:
      image: ghcr.io/maplibre/martin:v0.14
      ports:
        - "3000:3000"
      environment:
        DATABASE_URL: postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable
  ```

  Replace with:
  ```yaml
    martin:
      image: ghcr.io/maplibre/martin:v0.14
      ports:
        - "3000:3000"
      environment:
        DATABASE_URL: postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable
      volumes:
        - ./martin.yaml:/usr/local/share/martin/martin.yaml:ro
      command: ["--config", "/usr/local/share/martin/martin.yaml"]
  ```

- [ ] Restart Martin to pick up the new config:
  ```bash
  docker compose up -d martin
  ```

- [ ] Commit:
  ```bash
  git add docker-compose.yml
  git commit -m "Mount martin.yaml config file into Martin container"
  ```

---

## Task 6: Run OSM import and venue extraction

**Dependency:** Tasks 1–5 complete.

This task has no code to write — it is a run-and-verify task.

### Steps

- [ ] Run the full import pipeline:
  ```bash
  make osm-import
  ```
  Expected output: osm2pgsql reports rows written to `osm.roads`, `osm.pois`, `osm.landuse`. For the Kanto region, expect on the order of:
  - `osm.roads`: 1–3 million rows
  - `osm.pois`: 200k–600k rows
  - `osm.landuse`: 50k–200k rows

- [ ] Extract venues:
  ```bash
  make osm-venues
  ```
  Expected: the final SELECT reports non-zero `venue_count` and `signal_count`.

- [ ] Spot-check via psql:
  ```bash
  psql "postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
    -c "SELECT highway, COUNT(*) FROM osm.roads GROUP BY highway ORDER BY count DESC LIMIT 15;"

  psql "postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
    -c "SELECT category, COUNT(*) FROM environment.venue GROUP BY category ORDER BY count DESC LIMIT 20;"

  psql "postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
    -c "SELECT COUNT(*) FROM environment.traffic_signal;"
  ```

---

## Task 7: Verify Martin serves tiles

**Dependency:** Task 5 (docker-compose updated), Task 6 (data imported).

### Steps

- [ ] Start Martin:
  ```bash
  docker compose up -d martin
  docker compose logs -f martin
  ```
  Martin should log "Listening on 0.0.0.0:3000" and list discovered sources.

- [ ] Fetch the tile catalog:
  ```bash
  curl -s http://localhost:3000/catalog | python3 -m json.tool | head -60
  ```
  Expected: JSON array listing source IDs including `osm_roads`, `osm_cycling`, `osm_pois`, `osm_landuse`, `routes`, `venues`, `traffic_signals`.

- [ ] Fetch a sample tile for each source (Tokyo tile at zoom 12, x=3632, y=1616):
  ```bash
  # Roads
  curl -o /tmp/tile_roads.mvt -w "%{http_code}" \
      http://localhost:3000/osm_roads/12/3632/1616

  # Cycling infrastructure
  curl -o /tmp/tile_cycling.mvt -w "%{http_code}" \
      http://localhost:3000/osm_cycling/12/3632/1616

  # Venues
  curl -o /tmp/tile_venues.mvt -w "%{http_code}" \
      http://localhost:3000/venues/14/14529/6464

  # Traffic signals
  curl -o /tmp/tile_signals.mvt -w "%{http_code}" \
      http://localhost:3000/traffic_signals/14/14529/6464
  ```
  All responses should return HTTP 200 and non-empty files (file size > 0 bytes).

- [ ] Verify tile content with `tippecanoe`'s `tile-join` or simply check that the MVT binary is non-empty:
  ```bash
  wc -c /tmp/tile_roads.mvt /tmp/tile_cycling.mvt /tmp/tile_venues.mvt /tmp/tile_signals.mvt
  ```

- [ ] Check TileJSON metadata:
  ```bash
  curl -s http://localhost:3000/osm_roads | python3 -m json.tool
  ```
  Should return TileJSON with `minzoom`, `maxzoom`, `bounds` populated.

- [ ] If any source fails to appear:
  1. Check `docker compose logs martin` for errors
  2. Confirm the table/schema exists: `\dt osm.*` in psql
  3. Confirm the function exists: `\df public.osm_cycling_tiles` in psql

---

## Task 8: Add osm-import to Makefile help and document the full pipeline

**Files:**
- `Makefile` (add a `help` target and `osm-all` convenience target)

### Steps

- [ ] Add a convenience `osm-all` target that runs the complete pipeline:
  ```makefile
  ## Run full OSM pipeline: download → import → venues
  osm-all: osm-import osm-venues
  ```

- [ ] Add a `help` target to `Makefile`:
  ```makefile
  .PHONY: help

  help:
  	@grep -E '^## ' Makefile | sed 's/^## //'
  ```

- [ ] Commit:
  ```bash
  git add Makefile
  git commit -m "Add osm-all convenience target and help target to Makefile"
  ```

---

## Self-Review

### Correctness checks

1. **osm2pgsql Lua style** — uses the flex output API (`osm2pgsql.define_way_table`, `define_node_table`, `define_area_table`). The `shop` column type is `'text'` not `'shop'` — verify the Lua file has no type typos before running import.

2. **Venue upsert** — `ON CONFLICT (osm_id)` requires a unique constraint. Migrations 000015 and 000016 add these. Apply `make migrate-up` before running `make osm-venues`.

3. **osm_cycling_tiles function** — uses `ST_TileEnvelope` (PostGIS 3.1+) and `ST_AsMVTGeom` with `ST_Transform`. Confirm the PostGIS version on the target database supports these:
   ```sql
   SELECT PostGIS_Full_Version();
   ```
   PostGIS 3.1+ required. If older, replace `ST_TileEnvelope` with the manual formula:
   ```sql
   -- Manual tile envelope for PostGIS < 3.1
   ST_MakeEnvelope(
       -20037508.3428 + x * (20037508.3428 * 2 / 2^z),
       20037508.3428 - (y + 1) * (20037508.3428 * 2 / 2^z),
       -20037508.3428 + (x + 1) * (20037508.3428 * 2 / 2^z),
       20037508.3428 - y * (20037508.3428 * 2 / 2^z),
       3857
   )
   ```

4. **Martin config schema** — Martin v0.14 uses a YAML structure with `postgres.tables` and `postgres.functions`. Confirm the exact key names against the Martin v0.14 changelog if the server fails to start; the API changed between 0.13 and 0.14.

5. **`routes.route` geometry column name** — the design spec uses `geometry` (LINESTRING Z). Verify the actual column name in migration 000003 before Martin starts. If it is named `geom` instead, update `martin.yaml` accordingly.

6. **`environment.venue` geometry column name** — design spec says `geometry (POINT)`. Verify in migration 000010. Update `martin.yaml` if the column is named `geom`.

7. **Traffic signal tile zoom** — signals are captured at zoom 14+. At zoom 12 the tile fetched in Task 7 will return an empty MVT (correct behavior, not an error).

8. **Memory for import** — the Kanto PBF is ~1.8 GB. `osm2pgsql --slim` with 4 processes will use ~4–8 GB RAM. The `--cache` flag defaults to 800 MB; on low-memory machines add `--cache=200` to reduce memory usage at the cost of slower import speed.

### What this plan does NOT cover

- osm2pgsql replication / diff updates (osmium or pyosmium for incremental diffs rather than full re-download)
- Greenery index population from `osm.landuse` into `environment.greenery_edge` (separate pipeline task)
- Martin CORS configuration (needed before the web frontend can fetch tiles cross-origin — add `cors_allow_origin: "*"` to `martin.yaml` under a `http` key)
- Authentication/rate-limiting on the Martin tile endpoint
- Valhalla PBF rebuild after OSM update (separate Makefile target)
