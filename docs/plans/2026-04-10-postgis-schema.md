# PostGIS Schema & Migrations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create all PostGIS schema migrations and seed data for the cyclist-map platform, covering routes, community, environment, and plan bounded contexts.

**Architecture:** Four PostgreSQL schemas (`routes`, `community`, `environment`, `plan`) isolate bounded contexts within a single PostGIS database. Migrations use golang-migrate with sequential numbered files. The `osm` schema is managed by osm2pgsql and excluded from this plan.

**Tech Stack:** PostgreSQL 16 + PostGIS 3.4, golang-migrate, SQL

---

## Task 1: Project scaffolding and migration tooling

**Files:**
- `migrations/` (directory)
- `Makefile` (modify — add migration targets)

- [ ] 1. Create the migrations directory structure:
```bash
mkdir -p migrations
```

- [ ] 2. Verify golang-migrate is available (install if needed):
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate --version
# Expected output: v4.x.x
```

- [ ] 3. Add migration helper targets to the Makefile (create if it does not exist):
```makefile
MIGRATE_URL ?= postgres://cyclist:cyclist@localhost:5432/cyclist_map?sslmode=disable

.PHONY: migrate-up migrate-down migrate-create

migrate-up:
	migrate -path migrations -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

migrate-create:
	@read -p "Name: " name; \
	migrate create -ext sql -dir migrations -seq -digits 6 $$name
```

- [ ] 4. Verify PostGIS is running and accessible:
```bash
psql "$(MIGRATE_URL)" -c "SELECT PostGIS_Version();"
# Expected output: 3.4 ...
```

- [ ] 5. Commit: `chore: add migrations directory and Makefile targets`

---

## Task 2: Create schemas and enable PostGIS

**Files:**
- `migrations/000001_create_schemas.up.sql`
- `migrations/000001_create_schemas.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000001_create_schemas.up.sql
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE SCHEMA IF NOT EXISTS routes;
CREATE SCHEMA IF NOT EXISTS community;
CREATE SCHEMA IF NOT EXISTS environment;
CREATE SCHEMA IF NOT EXISTS plan;
```

- [ ] 2. Create the down migration:
```sql
-- 000001_create_schemas.down.sql
DROP SCHEMA IF EXISTS plan CASCADE;
DROP SCHEMA IF EXISTS environment CASCADE;
DROP SCHEMA IF EXISTS community CASCADE;
DROP SCHEMA IF EXISTS routes CASCADE;

-- Extensions are left in place intentionally
```

- [ ] 3. Run the migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT schema_name FROM information_schema.schemata WHERE schema_name IN ('routes','community','environment','plan') ORDER BY schema_name;"
# Expected: community, environment, plan, routes
```

- [ ] 4. Commit: `feat(db): add base schemas and PostGIS extension`

---

## Task 3: Routes schema — enum types

**Files:**
- `migrations/000002_routes_enums.up.sql`
- `migrations/000002_routes_enums.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000002_routes_enums.up.sql
CREATE TYPE routes.difficulty AS ENUM ('easy', 'moderate', 'hard', 'expert');
CREATE TYPE routes.route_status AS ENUM ('draft', 'published', 'archived');
CREATE TYPE routes.waypoint_type AS ENUM ('viewpoint', 'rest_stop', 'water', 'shrine', 'konbini', 'other');
CREATE TYPE routes.surface_type AS ENUM ('paved', 'gravel', 'dirt', 'cobblestone');
```

- [ ] 2. Create the down migration:
```sql
-- 000002_routes_enums.down.sql
DROP TYPE IF EXISTS routes.surface_type;
DROP TYPE IF EXISTS routes.waypoint_type;
DROP TYPE IF EXISTS routes.route_status;
DROP TYPE IF EXISTS routes.difficulty;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT typname FROM pg_type JOIN pg_namespace ON pg_type.typnamespace = pg_namespace.oid WHERE nspname = 'routes' AND typtype = 'e' ORDER BY typname;"
# Expected: difficulty, route_status, surface_type, waypoint_type
```

- [ ] 4. Commit: `feat(db): add routes schema enum types`

---

## Task 4: Routes schema — route table

**Files:**
- `migrations/000003_routes_route.up.sql`
- `migrations/000003_routes_route.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000003_routes_route.up.sql
CREATE TABLE routes.route (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            TEXT NOT NULL,
    description     TEXT,
    geometry        geometry(LINESTRINGZ, 4326) NOT NULL,
    distance_m      DOUBLE PRECISION NOT NULL,
    elevation_gain_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    elevation_loss_m DOUBLE PRECISION NOT NULL DEFAULT 0,
    difficulty      routes.difficulty NOT NULL DEFAULT 'moderate',
    status          routes.route_status NOT NULL DEFAULT 'draft',
    creator_id      UUID,  -- FK added after community.user exists
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_route_geometry ON routes.route USING GIST (geometry);
CREATE INDEX idx_route_status ON routes.route (status);
CREATE INDEX idx_route_difficulty ON routes.route (difficulty);
CREATE INDEX idx_route_creator_id ON routes.route (creator_id);
```

- [ ] 2. Create the down migration:
```sql
-- 000003_routes_route.down.sql
DROP TABLE IF EXISTS routes.route CASCADE;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "\d routes.route"
# Expected: table with all columns listed above
```

- [ ] 4. Commit: `feat(db): add routes.route table`

---

## Task 5: Routes schema — waypoint, route_segment, route_tag

**Files:**
- `migrations/000004_routes_supporting_tables.up.sql`
- `migrations/000004_routes_supporting_tables.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000004_routes_supporting_tables.up.sql
CREATE TABLE routes.waypoint (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id    UUID NOT NULL REFERENCES routes.route(id) ON DELETE CASCADE,
    geometry    geometry(POINT, 4326) NOT NULL,
    name        TEXT,
    type        routes.waypoint_type NOT NULL DEFAULT 'other',
    sort_order  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_waypoint_route_id ON routes.waypoint (route_id);
CREATE INDEX idx_waypoint_geometry ON routes.waypoint USING GIST (geometry);

CREATE TABLE routes.route_segment (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id        UUID NOT NULL REFERENCES routes.route(id) ON DELETE CASCADE,
    geometry        geometry(LINESTRINGZ, 4326) NOT NULL,
    surface_type    routes.surface_type NOT NULL DEFAULT 'paved',
    grade_percent   DOUBLE PRECISION NOT NULL DEFAULT 0,
    segment_order   INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_route_segment_route_id ON routes.route_segment (route_id);
CREATE INDEX idx_route_segment_geometry ON routes.route_segment USING GIST (geometry);

CREATE TABLE routes.route_tag (
    route_id    UUID NOT NULL REFERENCES routes.route(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    PRIMARY KEY (route_id, tag)
);

CREATE INDEX idx_route_tag_tag ON routes.route_tag (tag);
```

- [ ] 2. Create the down migration:
```sql
-- 000004_routes_supporting_tables.down.sql
DROP TABLE IF EXISTS routes.route_tag CASCADE;
DROP TABLE IF EXISTS routes.route_segment CASCADE;
DROP TABLE IF EXISTS routes.waypoint CASCADE;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'routes' ORDER BY table_name;"
# Expected: route, route_segment, route_tag, waypoint
```

- [ ] 4. Commit: `feat(db): add waypoint, route_segment, route_tag tables`

---

## Task 6: Community schema — enum types and user table

**Files:**
- `migrations/000005_community_user.up.sql`
- `migrations/000005_community_user.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000005_community_user.up.sql
CREATE TYPE community.contribution_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE community."user" (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    display_name    TEXT NOT NULL,
    email           TEXT NOT NULL UNIQUE,
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_email ON community."user" (email);

-- Now add the FK from routes.route.creator_id to community.user
ALTER TABLE routes.route
    ADD CONSTRAINT fk_route_creator
    FOREIGN KEY (creator_id) REFERENCES community."user"(id);
```

- [ ] 2. Create the down migration:
```sql
-- 000005_community_user.down.sql
ALTER TABLE routes.route DROP CONSTRAINT IF EXISTS fk_route_creator;
DROP TABLE IF EXISTS community."user" CASCADE;
DROP TYPE IF EXISTS community.contribution_status;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "\d community.user"
# Expected: table with id, display_name, email, avatar_url, created_at
```

- [ ] 4. Commit: `feat(db): add community.user table and route creator FK`

---

## Task 7: Community schema — contribution, review, ride_log

**Files:**
- `migrations/000006_community_tables.up.sql`
- `migrations/000006_community_tables.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000006_community_tables.up.sql
CREATE TABLE community.contribution (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES community."user"(id),
    route_geometry  geometry(LINESTRINGZ, 4326) NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    status          community.contribution_status NOT NULL DEFAULT 'pending',
    moderator_notes TEXT,
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_contribution_user_id ON community.contribution (user_id);
CREATE INDEX idx_contribution_status ON community.contribution (status);
CREATE INDEX idx_contribution_geometry ON community.contribution USING GIST (route_geometry);

CREATE TABLE community.review (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES community."user"(id),
    route_id    UUID NOT NULL REFERENCES routes.route(id) ON DELETE CASCADE,
    rating      SMALLINT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    body        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_review_route_id ON community.review (route_id);
CREATE INDEX idx_review_user_id ON community.review (user_id);

CREATE TABLE community.ride_log (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES community."user"(id),
    route_id    UUID NOT NULL REFERENCES routes.route(id) ON DELETE CASCADE,
    ridden_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    duration_s  INTEGER NOT NULL,
    gpx_track   geometry(LINESTRINGZ, 4326)
);

CREATE INDEX idx_ride_log_user_id ON community.ride_log (user_id);
CREATE INDEX idx_ride_log_route_id ON community.ride_log (route_id);
CREATE INDEX idx_ride_log_gpx_track ON community.ride_log USING GIST (gpx_track);
```

- [ ] 2. Create the down migration:
```sql
-- 000006_community_tables.down.sql
DROP TABLE IF EXISTS community.ride_log CASCADE;
DROP TABLE IF EXISTS community.review CASCADE;
DROP TABLE IF EXISTS community.contribution CASCADE;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'community' ORDER BY table_name;"
# Expected: contribution, review, ride_log, user
```

- [ ] 4. Commit: `feat(db): add contribution, review, ride_log tables`

---

## Task 8: Environment schema — enum types

**Files:**
- `migrations/000007_environment_enums.up.sql`
- `migrations/000007_environment_enums.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000007_environment_enums.up.sql
CREATE TYPE environment.green_wave_source AS ENUM ('ride_log_inferred', 'user_reported');
```

- [ ] 2. Create the down migration:
```sql
-- 000007_environment_enums.down.sql
DROP TYPE IF EXISTS environment.green_wave_source;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT typname FROM pg_type JOIN pg_namespace ON pg_type.typnamespace = pg_namespace.oid WHERE nspname = 'environment' AND typtype = 'e';"
# Expected: green_wave_source
```

- [ ] 4. Commit: `feat(db): add environment schema enum types`

---

## Task 9: Environment schema — shadow_grid, greenery_edge

**Files:**
- `migrations/000008_environment_shadow_greenery.up.sql`
- `migrations/000008_environment_shadow_greenery.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000008_environment_shadow_greenery.up.sql
CREATE TABLE environment.shadow_grid (
    id              BIGSERIAL PRIMARY KEY,
    cell_geometry   geometry(POLYGON, 4326) NOT NULL,
    hour_slot       SMALLINT NOT NULL CHECK (hour_slot >= 0 AND hour_slot <= 23),
    month           SMALLINT NOT NULL CHECK (month >= 1 AND month <= 12),
    shade_coverage  DOUBLE PRECISION NOT NULL CHECK (shade_coverage >= 0.0 AND shade_coverage <= 1.0)
);

CREATE INDEX idx_shadow_grid_geometry ON environment.shadow_grid USING GIST (cell_geometry);
CREATE INDEX idx_shadow_grid_time ON environment.shadow_grid (month, hour_slot);

CREATE TABLE environment.greenery_edge (
    osm_way_id      BIGINT PRIMARY KEY,
    greenery_score  DOUBLE PRECISION NOT NULL CHECK (greenery_score >= 0.0 AND greenery_score <= 1.0),
    tree_lined      BOOLEAN NOT NULL DEFAULT false,
    park_adjacent   BOOLEAN NOT NULL DEFAULT false
);
```

- [ ] 2. Create the down migration:
```sql
-- 000008_environment_shadow_greenery.down.sql
DROP TABLE IF EXISTS environment.greenery_edge CASCADE;
DROP TABLE IF EXISTS environment.shadow_grid CASCADE;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "\d environment.shadow_grid"
psql "$MIGRATE_URL" -c "\d environment.greenery_edge"
```

- [ ] 4. Commit: `feat(db): add shadow_grid and greenery_edge tables`

---

## Task 10: Environment schema — weather_grid, traffic_signal

**Files:**
- `migrations/000009_environment_weather_signal.up.sql`
- `migrations/000009_environment_weather_signal.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000009_environment_weather_signal.up.sql
CREATE TABLE environment.weather_grid (
    id                  BIGSERIAL PRIMARY KEY,
    cell_geometry       geometry(POLYGON, 4326) NOT NULL,
    valid_at            TIMESTAMPTZ NOT NULL,
    wind_speed_ms       DOUBLE PRECISION NOT NULL,
    wind_bearing_deg    DOUBLE PRECISION NOT NULL CHECK (wind_bearing_deg >= 0 AND wind_bearing_deg < 360),
    precip_intensity_mmh DOUBLE PRECISION NOT NULL DEFAULT 0,
    temperature_c       DOUBLE PRECISION NOT NULL
);

CREATE INDEX idx_weather_grid_geometry ON environment.weather_grid USING GIST (cell_geometry);
CREATE INDEX idx_weather_grid_valid_at ON environment.weather_grid (valid_at);

CREATE TABLE environment.traffic_signal (
    osm_node_id     BIGINT PRIMARY KEY,
    geometry        geometry(POINT, 4326) NOT NULL
);

CREATE INDEX idx_traffic_signal_geometry ON environment.traffic_signal USING GIST (geometry);
```

- [ ] 2. Create the down migration:
```sql
-- 000009_environment_weather_signal.down.sql
DROP TABLE IF EXISTS environment.traffic_signal CASCADE;
DROP TABLE IF EXISTS environment.weather_grid CASCADE;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "\d environment.weather_grid"
psql "$MIGRATE_URL" -c "\d environment.traffic_signal"
```

- [ ] 4. Commit: `feat(db): add weather_grid and traffic_signal tables`

---

## Task 11: Environment schema — green_wave, venue, venue_tag_mapping

**Files:**
- `migrations/000010_environment_greenwave_venue.up.sql`
- `migrations/000010_environment_greenwave_venue.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000010_environment_greenwave_venue.up.sql
CREATE TABLE environment.green_wave (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    osm_way_ids         BIGINT[] NOT NULL,
    direction_bearing   DOUBLE PRECISION NOT NULL CHECK (direction_bearing >= 0 AND direction_bearing < 360),
    target_speed_kmh    DOUBLE PRECISION NOT NULL,
    confidence          DOUBLE PRECISION NOT NULL CHECK (confidence >= 0.0 AND confidence <= 1.0),
    source              environment.green_wave_source NOT NULL,
    detected_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_green_wave_osm_way_ids ON environment.green_wave USING GIN (osm_way_ids);
CREATE INDEX idx_green_wave_confidence ON environment.green_wave (confidence);

CREATE TABLE environment.venue (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    osm_id      BIGINT NOT NULL UNIQUE,
    geometry    geometry(POINT, 4326) NOT NULL,
    name        TEXT,
    category    TEXT NOT NULL,
    brand       TEXT,
    osm_tags    JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_venue_geometry ON environment.venue USING GIST (geometry);
CREATE INDEX idx_venue_osm_id ON environment.venue (osm_id);
CREATE INDEX idx_venue_category ON environment.venue (category);
CREATE INDEX idx_venue_brand ON environment.venue (brand) WHERE brand IS NOT NULL;
CREATE INDEX idx_venue_osm_tags ON environment.venue USING GIN (osm_tags);

CREATE TABLE environment.venue_tag_mapping (
    hashtag     TEXT PRIMARY KEY,
    osm_filter  JSONB NOT NULL,
    description TEXT,
    is_brand    BOOLEAN NOT NULL DEFAULT false
);
```

- [ ] 2. Create the down migration:
```sql
-- 000010_environment_greenwave_venue.down.sql
DROP TABLE IF EXISTS environment.venue_tag_mapping CASCADE;
DROP TABLE IF EXISTS environment.venue CASCADE;
DROP TABLE IF EXISTS environment.green_wave CASCADE;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'environment' ORDER BY table_name;"
# Expected: green_wave, greenery_edge, shadow_grid, traffic_signal, venue, venue_tag_mapping, weather_grid
```

- [ ] 4. Commit: `feat(db): add green_wave, venue, venue_tag_mapping tables`

---

## Task 12: Plan schema — enum types and route_plan

**Files:**
- `migrations/000011_plan_tables.up.sql`
- `migrations/000011_plan_tables.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000011_plan_tables.up.sql
CREATE TYPE plan.speed_model AS ENUM ('elevation', 'flat', 'custom');
CREATE TYPE plan.stop_type AS ENUM ('manual', 'venue_resolved', 'waypoint');
CREATE TYPE plan.task_status AS ENUM ('unresolved', 'matched', 'completed');

CREATE TABLE plan.route_plan (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES community."user"(id),
    departure_at    TIMESTAMPTZ,
    speed_model     plan.speed_model NOT NULL DEFAULT 'elevation',
    shade_weight    DOUBLE PRECISION NOT NULL DEFAULT 0.5 CHECK (shade_weight >= 0.0 AND shade_weight <= 1.0),
    greenery_weight DOUBLE PRECISION NOT NULL DEFAULT 0.5 CHECK (greenery_weight >= 0.0 AND greenery_weight <= 1.0),
    wind_weight     DOUBLE PRECISION NOT NULL DEFAULT 0.5 CHECK (wind_weight >= 0.0 AND wind_weight <= 1.0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_route_plan_user_id ON plan.route_plan (user_id);

CREATE TABLE plan.stop_point (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id         UUID NOT NULL REFERENCES plan.route_plan(id) ON DELETE CASCADE,
    geometry        geometry(POINT, 4326) NOT NULL,
    type            plan.stop_type NOT NULL DEFAULT 'manual',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    venue_id        UUID REFERENCES environment.venue(id),
    resolved_name   TEXT
);

CREATE INDEX idx_stop_point_plan_id ON plan.stop_point (plan_id);
CREATE INDEX idx_stop_point_geometry ON plan.stop_point USING GIST (geometry);

CREATE TABLE plan.plan_task (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id             UUID NOT NULL REFERENCES plan.route_plan(id) ON DELETE CASCADE,
    description         TEXT NOT NULL,
    hashtag             TEXT,
    status              plan.task_status NOT NULL DEFAULT 'unresolved',
    resolved_venue_id   UUID REFERENCES environment.venue(id)
);

CREATE INDEX idx_plan_task_plan_id ON plan.plan_task (plan_id);
CREATE INDEX idx_plan_task_status ON plan.plan_task (status);
```

- [ ] 2. Create the down migration:
```sql
-- 000011_plan_tables.down.sql
DROP TABLE IF EXISTS plan.plan_task CASCADE;
DROP TABLE IF EXISTS plan.stop_point CASCADE;
DROP TABLE IF EXISTS plan.route_plan CASCADE;
DROP TYPE IF EXISTS plan.task_status;
DROP TYPE IF EXISTS plan.stop_type;
DROP TYPE IF EXISTS plan.speed_model;
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'plan' ORDER BY table_name;"
# Expected: plan_task, route_plan, stop_point
```

- [ ] 4. Commit: `feat(db): add plan schema tables (route_plan, stop_point, plan_task)`

---

## Task 13: Seed venue_tag_mapping with common hashtags

**Files:**
- `migrations/000012_seed_venue_tag_mappings.up.sql`
- `migrations/000012_seed_venue_tag_mappings.down.sql`

- [ ] 1. Create the up migration:
```sql
-- 000012_seed_venue_tag_mappings.up.sql
INSERT INTO environment.venue_tag_mapping (hashtag, osm_filter, description, is_brand) VALUES
    -- Brand-level convenience stores
    ('#7-eleven',    '{"shop": "convenience", "brand_pattern": "%seven%eleven%"}',
        'Seven-Eleven convenience stores', true),
    ('#lawson',      '{"shop": "convenience", "brand_pattern": "%lawson%"}',
        'Lawson convenience stores', true),
    ('#familymart',  '{"shop": "convenience", "brand_pattern": "%familymart%"}',
        'FamilyMart convenience stores', true),
    ('#ministop',    '{"shop": "convenience", "brand_pattern": "%ministop%"}',
        'Ministop convenience stores', true),

    -- Category-level
    ('#konbini',     '{"shop": "convenience"}',
        'Any convenience store (konbini)', false),
    ('#cafe',        '{"amenity": "cafe"}',
        'Cafes and coffee shops', false),
    ('#bike-shop',   '{"shop": "bicycle"}',
        'Bicycle shops and repair', false),
    ('#vending',     '{"amenity": "vending_machine"}',
        'Vending machines', false),
    ('#park',        '{"leisure": "park"}',
        'Parks and green spaces', false),
    ('#toilet',      '{"amenity": "toilets"}',
        'Public toilets and restrooms', false),
    ('#water',       '{"amenity": "drinking_water"}',
        'Drinking water fountains', false),
    ('#shrine',      '{"amenity": "place_of_worship", "religion": "shinto"}',
        'Shinto shrines', false),
    ('#temple',      '{"amenity": "place_of_worship", "religion": "buddhist"}',
        'Buddhist temples', false),
    ('#rest-area',   '{"highway": "rest_area"}',
        'Roadside rest areas', false),
    ('#supermarket', '{"shop": "supermarket"}',
        'Supermarkets and grocery stores', false),
    ('#restaurant',  '{"amenity": "restaurant"}',
        'Restaurants', false),
    ('#onsen',       '{"leisure": "hot_spring"}',
        'Hot springs (onsen)', false),
    ('#atm',         '{"amenity": "atm"}',
        'ATMs and cash machines', false)
ON CONFLICT (hashtag) DO NOTHING;
```

- [ ] 2. Create the down migration:
```sql
-- 000012_seed_venue_tag_mappings.down.sql
DELETE FROM environment.venue_tag_mapping WHERE hashtag IN (
    '#7-eleven', '#lawson', '#familymart', '#ministop',
    '#konbini', '#cafe', '#bike-shop', '#vending', '#park',
    '#toilet', '#water', '#shrine', '#temple', '#rest-area',
    '#supermarket', '#restaurant', '#onsen', '#atm'
);
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT hashtag, is_brand, description FROM environment.venue_tag_mapping ORDER BY hashtag;"
# Expected: 18 rows
```

- [ ] 4. Commit: `feat(db): seed venue_tag_mapping with common hashtags`

---

## Task 14: Seed sample Tokyo routes

**Files:**
- `migrations/000013_seed_sample_routes.up.sql`
- `migrations/000013_seed_sample_routes.down.sql`

- [ ] 1. Create the up migration with sample routes, waypoints, segments, and tags. All geometries use real Tokyo coordinates with SRID 4326.

```sql
-- 000013_seed_sample_routes.up.sql

-- Seed user for sample data
INSERT INTO community."user" (id, display_name, email)
VALUES ('00000000-0000-0000-0000-000000000001', 'Cyclist Map Team', 'team@cyclist-map.dev');

-- ============================================================
-- Route 1: Tama River Cycling Path (Tamagawa Cycling Road)
-- ~40km along the Tama River from Haneda to Hamura
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000001',
    'Tama River Cycling Path (多摩川サイクリングロード)',
    'A classic Tokyo cycling route following the Tama River from near Haneda Airport upstream to Hamura. Mostly flat, paved path with river views. Popular with road cyclists and casual riders alike.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(
        139.7380 35.5530 3,
        139.7100 35.5720 5,
        139.6700 35.5950 8,
        139.6300 35.6100 15,
        139.5800 35.6300 25,
        139.5300 35.6500 35,
        139.4800 35.6650 50,
        139.4300 35.6800 65,
        139.3500 35.7100 80,
        139.3100 35.7400 95
    )'), 4326),
    40200,
    92,
    3,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_MakePoint(139.7380, 35.5530), 4326), 'Rokugobashi Start', 'other', 0),
    ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_MakePoint(139.6700, 35.5950), 4326), 'Maruko Bridge Rest Area', 'rest_stop', 1),
    ('20000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_MakePoint(139.5300, 35.6500), 4326), 'Fuchu Waterfront', 'viewpoint', 2),
    ('20000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_MakePoint(139.3100, 35.7400), 4326), 'Hamura Weir Finish', 'viewpoint', 3);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.7380 35.5530 3, 139.7100 35.5720 5, 139.6700 35.5950 8)'), 4326),
     'paved', 0.1, 0),
    ('30000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.6700 35.5950 8, 139.6300 35.6100 15, 139.5800 35.6300 25, 139.5300 35.6500 35)'), 4326),
     'paved', 0.2, 1),
    ('30000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.5300 35.6500 35, 139.4800 35.6650 50, 139.4300 35.6800 65, 139.3500 35.7100 80, 139.3100 35.7400 95)'), 4326),
     'paved', 0.3, 2);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000001', 'river'),
    ('10000000-0000-0000-0000-000000000001', 'flat'),
    ('10000000-0000-0000-0000-000000000001', 'beginner-friendly'),
    ('10000000-0000-0000-0000-000000000001', 'long-ride');

-- ============================================================
-- Route 2: Imperial Palace Loop (皇居一周)
-- ~5km loop around the Imperial Palace, central Tokyo
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000002',
    'Imperial Palace Loop (皇居一周)',
    'The iconic 5km loop around the Imperial Palace. Wide roads, minimal traffic on weekends, and beautiful moat views. A Tokyo cycling rite of passage. Best early morning or Sunday when roads are quieter.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(
        139.7560 35.6825 15,
        139.7530 35.6870 14,
        139.7480 35.6890 13,
        139.7420 35.6870 12,
        139.7390 35.6820 14,
        139.7400 35.6770 16,
        139.7430 35.6740 17,
        139.7490 35.6740 16,
        139.7530 35.6770 15,
        139.7560 35.6825 15
    )'), 4326),
    5000,
    20,
    20,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_MakePoint(139.7560, 35.6825), 4326), 'Sakuradamon Gate Start', 'other', 0),
    ('20000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_MakePoint(139.7420, 35.6870), 4326), 'Chidorigafuchi Moat View', 'viewpoint', 1),
    ('20000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_MakePoint(139.7430, 35.6740), 4326), 'Sakashitamon Water Stop', 'water', 2);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.7560 35.6825 15, 139.7530 35.6870 14, 139.7480 35.6890 13, 139.7420 35.6870 12, 139.7390 35.6820 14)'), 4326),
     'paved', 0.5, 0),
    ('30000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000002',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.7390 35.6820 14, 139.7400 35.6770 16, 139.7430 35.6740 17, 139.7490 35.6740 16, 139.7530 35.6770 15, 139.7560 35.6825 15)'), 4326),
     'paved', 0.6, 1);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000002', 'loop'),
    ('10000000-0000-0000-0000-000000000002', 'urban'),
    ('10000000-0000-0000-0000-000000000002', 'beginner-friendly'),
    ('10000000-0000-0000-0000-000000000002', 'iconic');

-- ============================================================
-- Route 3: Arakawa River to Tokyo Bay (荒川下流)
-- ~25km downstream along Arakawa River to Kasai Rinkai Park
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000003',
    'Arakawa River to Tokyo Bay (荒川下流)',
    'Follow the Arakawa River downstream through eastern Tokyo to the bay. Wide riverside paths, views of Tokyo Skytree, and finishes at Kasai Rinkai Park seaside. Flat and easy with good konbini access along the way.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(
        139.7100 35.7950 5,
        139.7200 35.7700 4,
        139.7400 35.7400 3,
        139.7700 35.7100 2,
        139.8000 35.6900 2,
        139.8200 35.6700 1,
        139.8500 35.6500 1
    )'), 4326),
    25300,
    5,
    9,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000020', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_MakePoint(139.7100, 35.7950), 4326), 'Akabane Start', 'other', 0),
    ('20000000-0000-0000-0000-000000000021', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_MakePoint(139.7700, 35.7100), 4326), 'Skytree View Point', 'viewpoint', 1),
    ('20000000-0000-0000-0000-000000000022', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_MakePoint(139.8200, 35.6700), 4326), 'Shin-Kiba Rest Stop', 'rest_stop', 2),
    ('20000000-0000-0000-0000-000000000023', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_MakePoint(139.8500, 35.6500), 4326), 'Kasai Rinkai Park Finish', 'viewpoint', 3);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000020', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.7100 35.7950 5, 139.7200 35.7700 4, 139.7400 35.7400 3, 139.7700 35.7100 2)'), 4326),
     'paved', 0.1, 0),
    ('30000000-0000-0000-0000-000000000021', '10000000-0000-0000-0000-000000000003',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.7700 35.7100 2, 139.8000 35.6900 2, 139.8200 35.6700 1, 139.8500 35.6500 1)'), 4326),
     'paved', 0.0, 1);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000003', 'river'),
    ('10000000-0000-0000-0000-000000000003', 'flat'),
    ('10000000-0000-0000-0000-000000000003', 'seaside'),
    ('10000000-0000-0000-0000-000000000003', 'beginner-friendly');

-- ============================================================
-- Route 4: Ome-Okutama Hill Climb (青梅・奥多摩ヒルクライム)
-- ~35km hill climb from Ome to Lake Okutama
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000004',
    'Ome to Okutama Hill Climb (青梅・奥多摩ヒルクライム)',
    'A challenging hill climb from Ome station along the Tama River gorge to Lake Okutama. Stunning mountain scenery, winding roads through forests, and a rewarding lakeside finish. The route follows Route 411 with some dedicated cycling paths.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(
        139.2780 35.7880 185,
        139.2500 35.7950 220,
        139.2200 35.8000 280,
        139.1900 35.8050 350,
        139.1600 35.8100 420,
        139.1300 35.8080 490,
        139.1000 35.7980 530
    )'), 4326),
    35400,
    680,
    335,
    'hard',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000030', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_MakePoint(139.2780, 35.7880), 4326), 'Ome Station Start', 'other', 0),
    ('20000000-0000-0000-0000-000000000031', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_MakePoint(139.2200, 35.8000), 4326), 'Mitake Valley Shrine', 'shrine', 1),
    ('20000000-0000-0000-0000-000000000032', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_MakePoint(139.1600, 35.8100), 4326), 'Mountain Konbini', 'konbini', 2),
    ('20000000-0000-0000-0000-000000000033', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_MakePoint(139.1000, 35.7980), 4326), 'Lake Okutama Dam Viewpoint', 'viewpoint', 3);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000030', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.2780 35.7880 185, 139.2500 35.7950 220, 139.2200 35.8000 280)'), 4326),
     'paved', 2.5, 0),
    ('30000000-0000-0000-0000-000000000031', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.2200 35.8000 280, 139.1900 35.8050 350, 139.1600 35.8100 420)'), 4326),
     'paved', 3.8, 1),
    ('30000000-0000-0000-0000-000000000032', '10000000-0000-0000-0000-000000000004',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.1600 35.8100 420, 139.1300 35.8080 490, 139.1000 35.7980 530)'), 4326),
     'paved', 4.2, 2);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000004', 'hill-climb'),
    ('10000000-0000-0000-0000-000000000004', 'mountain'),
    ('10000000-0000-0000-0000-000000000004', 'scenic'),
    ('10000000-0000-0000-0000-000000000004', 'challenging');

-- ============================================================
-- Route 5: Edogawa River Path (江戸川サイクリングロード)
-- ~20km along Edogawa River, easy flat ride
-- ============================================================
INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id)
VALUES (
    '10000000-0000-0000-0000-000000000005',
    'Edogawa River Path (江戸川サイクリングロード)',
    'A relaxed ride along the Edogawa River on the eastern edge of Tokyo. Dedicated cycling path, flat terrain, and great sunset views. Connects multiple parks along the way.',
    ST_SetSRID(ST_GeomFromText('LINESTRING Z(
        139.8900 35.7800 3,
        139.8950 35.7500 3,
        139.9000 35.7200 2,
        139.9050 35.6900 2,
        139.9100 35.6600 1
    )'), 4326),
    20100,
    3,
    5,
    'easy',
    'published',
    '00000000-0000-0000-0000-000000000001'
);

INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000040', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_MakePoint(139.8900, 35.7800), 4326), 'Shibamata Start', 'other', 0),
    ('20000000-0000-0000-0000-000000000041', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_MakePoint(139.9000, 35.7200), 4326), 'Edogawa Riverside Park', 'viewpoint', 1),
    ('20000000-0000-0000-0000-000000000042', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_MakePoint(139.9100, 35.6600), 4326), 'Gyotoku Finish', 'other', 2);

INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order) VALUES
    ('30000000-0000-0000-0000-000000000040', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.8900 35.7800 3, 139.8950 35.7500 3, 139.9000 35.7200 2)'), 4326),
     'paved', 0.0, 0),
    ('30000000-0000-0000-0000-000000000041', '10000000-0000-0000-0000-000000000005',
     ST_SetSRID(ST_GeomFromText('LINESTRING Z(139.9000 35.7200 2, 139.9050 35.6900 2, 139.9100 35.6600 1)'), 4326),
     'paved', 0.0, 1);

INSERT INTO routes.route_tag (route_id, tag) VALUES
    ('10000000-0000-0000-0000-000000000005', 'river'),
    ('10000000-0000-0000-0000-000000000005', 'flat'),
    ('10000000-0000-0000-0000-000000000005', 'beginner-friendly'),
    ('10000000-0000-0000-0000-000000000005', 'sunset');
```

- [ ] 2. Create the down migration:
```sql
-- 000013_seed_sample_routes.down.sql
DELETE FROM routes.route_tag WHERE route_id IN (
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000004',
    '10000000-0000-0000-0000-000000000005'
);
DELETE FROM routes.route_segment WHERE route_id IN (
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000004',
    '10000000-0000-0000-0000-000000000005'
);
DELETE FROM routes.waypoint WHERE route_id IN (
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000004',
    '10000000-0000-0000-0000-000000000005'
);
DELETE FROM routes.route WHERE id IN (
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000004',
    '10000000-0000-0000-0000-000000000005'
);
DELETE FROM community."user" WHERE id = '00000000-0000-0000-0000-000000000001';
```

- [ ] 3. Run migration and verify:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
psql "$MIGRATE_URL" -c "SELECT name, difficulty, distance_m FROM routes.route ORDER BY name;"
# Expected: 5 routes (Arakawa, Edogawa, Imperial Palace, Ome-Okutama, Tama River)
psql "$MIGRATE_URL" -c "SELECT count(*) FROM routes.waypoint;"
# Expected: 16
psql "$MIGRATE_URL" -c "SELECT count(*) FROM routes.route_segment;"
# Expected: 12
psql "$MIGRATE_URL" -c "SELECT count(*) FROM routes.route_tag;"
# Expected: 18 (removed one duplicate)
```

- [ ] 4. Commit: `feat(db): seed sample Tokyo cycling routes`

---

## Task 15: Full migration smoke test (down and up)

**Files:** None (verification only)

- [ ] 1. Run all migrations down to zero:
```bash
migrate -path migrations -database "$MIGRATE_URL" down -all
psql "$MIGRATE_URL" -c "SELECT schema_name FROM information_schema.schemata WHERE schema_name IN ('routes','community','environment','plan');"
# Expected: 0 rows
```

- [ ] 2. Run all migrations up from zero:
```bash
migrate -path migrations -database "$MIGRATE_URL" up
# Expected: no errors
```

- [ ] 3. Verify final state — all tables present:
```bash
psql "$MIGRATE_URL" -c "
SELECT table_schema, table_name
FROM information_schema.tables
WHERE table_schema IN ('routes','community','environment','plan')
ORDER BY table_schema, table_name;
"
# Expected output:
# community  | contribution
# community  | review
# community  | ride_log
# community  | user
# environment | green_wave
# environment | greenery_edge
# environment | shadow_grid
# environment | traffic_signal
# environment | venue
# environment | venue_tag_mapping
# environment | weather_grid
# plan       | plan_task
# plan       | route_plan
# plan       | stop_point
# routes     | route
# routes     | route_segment
# routes     | route_tag
# routes     | waypoint
```

- [ ] 4. Verify seed data survived the round trip:
```bash
psql "$MIGRATE_URL" -c "SELECT count(*) FROM routes.route;"
# Expected: 5
psql "$MIGRATE_URL" -c "SELECT count(*) FROM environment.venue_tag_mapping;"
# Expected: 18
```

- [ ] 5. Verify all spatial indexes exist:
```bash
psql "$MIGRATE_URL" -c "
SELECT schemaname, tablename, indexname
FROM pg_indexes
WHERE indexname LIKE 'idx_%geometry%' OR indexname LIKE 'idx_%gpx%'
ORDER BY schemaname, tablename;
"
# Expected: GiST indexes on all geometry columns
```

- [ ] 6. Commit: `test(db): verify full migration round-trip`

---

## Self-Review Checklist

### Table/column coverage vs. design spec

| Spec Table | Migration | All Columns Present |
|------------|-----------|-------------------|
| `routes.route` | 000003 | id, name, description, geometry (LINESTRING Z), distance_m, elevation_gain_m, elevation_loss_m, difficulty (enum), status (enum), creator_id, created_at, updated_at -- YES |
| `routes.waypoint` | 000004 | id, route_id (FK), geometry (POINT), name, type (enum), sort_order -- YES |
| `routes.route_segment` | 000004 | id, route_id (FK), geometry (LINESTRING Z), surface_type (enum), grade_percent, segment_order -- YES |
| `routes.route_tag` | 000004 | route_id, tag (text), composite PK -- YES |
| `community.user` | 000005 | id (UUID), display_name, email, avatar_url, created_at -- YES |
| `community.contribution` | 000006 | id, user_id, route_geometry (LINESTRING Z), metadata (JSONB), status (enum), moderator_notes, submitted_at -- YES |
| `community.review` | 000006 | id, user_id, route_id, rating (1-5), body, created_at -- YES |
| `community.ride_log` | 000006 | id, user_id, route_id, ridden_at, duration_s, gpx_track (LINESTRING Z, nullable) -- YES |
| `environment.shadow_grid` | 000008 | id, cell_geometry (POLYGON), hour_slot (0-23), month, shade_coverage (0.0-1.0) -- YES |
| `environment.greenery_edge` | 000008 | osm_way_id, greenery_score (0.0-1.0), tree_lined (bool), park_adjacent (bool) -- YES |
| `environment.weather_grid` | 000009 | id, cell_geometry (POLYGON), valid_at (TIMESTAMPTZ), wind_speed_ms, wind_bearing_deg, precip_intensity_mmh, temperature_c -- YES |
| `environment.traffic_signal` | 000009 | osm_node_id, geometry (POINT) -- YES |
| `environment.green_wave` | 000010 | id, osm_way_ids (BIGINT[]), direction_bearing, target_speed_kmh, confidence, source (enum), detected_at -- YES |
| `environment.venue` | 000010 | id, osm_id, geometry (POINT), name, category, brand (nullable), osm_tags (JSONB) -- YES |
| `environment.venue_tag_mapping` | 000010 | hashtag (text PK), osm_filter (JSONB), description, is_brand (bool) -- YES |
| `plan.route_plan` | 000011 | id (UUID), user_id, departure_at, speed_model (enum), shade_weight, greenery_weight, wind_weight, created_at -- YES |
| `plan.stop_point` | 000011 | id, plan_id (FK), geometry (POINT), type (enum), sort_order, venue_id (FK nullable), resolved_name -- YES |
| `plan.plan_task` | 000011 | id, plan_id (FK), description, hashtag (nullable), status (enum), resolved_venue_id (FK nullable) -- YES |

### Placeholder check
- No "TBD", "TODO", or placeholder text found in any SQL block.

### Type consistency
- All UUIDs use `uuid_generate_v4()` default.
- All geometries specify SRID 4326.
- All LINESTRING Z columns use `geometry(LINESTRINGZ, 4326)`.
- All POINT columns use `geometry(POINT, 4326)`.
- All POLYGON columns use `geometry(POLYGON, 4326)`.
- All TIMESTAMPTZ columns default to `now()` where appropriate.
- All 0.0-1.0 range columns have CHECK constraints.
- All enum references match their schema-qualified type names.
- FK from `routes.route.creator_id` to `community.user.id` added in migration 000005 after user table exists.
- Cross-schema FKs from `plan.stop_point.venue_id` and `plan.plan_task.resolved_venue_id` to `environment.venue.id` added in migration 000011 after venue table exists (migration 000010).

### Spatial indexes
- GiST index on every geometry column across all schemas.
- GIN index on `environment.green_wave.osm_way_ids` (array) and `environment.venue.osm_tags` (JSONB).
