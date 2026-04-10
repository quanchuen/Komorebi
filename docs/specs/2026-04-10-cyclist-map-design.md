# Cyclist Map — Design Spec

## Overview

An open-source, self-hosted cycling route discovery platform for recreational and touring cyclists. Users browse curated routes, contribute their own, and plan rides with environment-aware routing that accounts for shade, greenery, wind, rain, traffic signals, and green wave corridors.

Initial target: Tokyo area. Designed for global expansion.

## Architecture

**Two-service split** with shared PostGIS database:

1. **Go API Server** — business logic, routing orchestration, REST API
2. **Martin Tile Server** — serves vector tiles directly from PostGIS to map clients

Supporting services (all self-hosted via docker-compose):

3. **Valhalla** — open-source routing engine with custom bicycle costing
4. **PostGIS** — spatial database, single instance, schema-per-context

### Deployment

- `docker-compose.yml` as the primary deployment method
- All services self-hosted, no managed cloud dependencies
- Single VPS capable of running the Tokyo region
- Four offline data pipelines run on schedule

## Bounded Contexts (DDD)

### 1. Routes

The core domain. Manages curated and user-contributed cycling routes.

**Aggregates & entities:**
- `Route` (aggregate root) — geometry (LINESTRING Z), name, description, distance_m, elevation_gain_m, elevation_loss_m, difficulty (easy/moderate/hard/expert), status (draft/published/archived), creator_id, tags
- `Waypoint` (value object) — point on a route (viewpoint, rest stop, water, shrine, konbini, other), sort order
- `RouteSegment` (value object) — section between waypoints, carrying surface_type (paved/gravel/dirt/cobblestone), grade_percent

**States:** Draft → Published → Archived

### 2. Discovery

How users find routes. Reads from Routes, owns search and ranking logic.

- Search/filter by area, distance, elevation, surface, difficulty, tags
- Spatial queries: routes near a point, routes within map viewport
- `GET /api/v1/discover/suggested` — ranks routes by current conditions (shade + weather + greenery) for a given location and departure time

### 3. Community

User-generated content and social features.

- `User` (aggregate root) — auth identity, profile, preferences
- `Contribution` — user-submitted route, goes through moderation before becoming a Route
- `Review` — rating (1-5) + text on a Route
- `RideLog` — "I rode this" record with optional GPS track (LINESTRING Z). GPS tracks feed into the green wave inference pipeline.

### 4. Environment

External and precomputed environmental data for route scoring.

**Sub-domains:**

#### Shadow/Shade
- Source: **PLATEAU** (MLIT CityGML LOD2 3D building models for Tokyo)
- Precomputed shadow masks per cell, per hour slot (0-23), per month
- `ShadowGrid` — cell geometry + shade_coverage (0.0–1.0)
- Solar position calculated via SPA algorithm (lat/lon + datetime)

#### Greenery
- Source: OSM tags — `landuse=forest/grass`, `leisure=park`, `natural=tree_row`, tree-lined highway attributes
- `GreeneryIndex` — per-edge greenery_score (0.0–1.0), tree_lined (bool), park_adjacent (bool)

#### Weather
- Source: **Open-Meteo** (free, open source, grid-level forecasts)
- Hourly wind speed/direction, precipitation intensity, temperature
- `WeatherGrid` — cell geometry + conditions at valid_at timestamp
- Wind direction relative to route bearing determines headwind/tailwind/crosswind penalty/bonus

#### Traffic Signals
- Source: OSM `highway=traffic_signals` nodes (good coverage in Tokyo)
- Signal density per segment used for speed model penalty (~30s per signal average)
- Displayed as markers on route map

#### Green Wave (グリーンウェーブ)
- Coordinated traffic signals that allow continuous green lights at a target speed
- Signal timing data not publicly available — detected via **crowdsourced inference** from ride log GPS tracks
- Detection: multiple riders pass through signals on a segment without stopping at a consistent speed → infer green wave
- `GreenWave` — osm_way_ids (ordered), direction (bearing), target_speed_kmh, confidence, source (ride_log_inferred/user_reported)
- Community annotation: users can also manually report green wave corridors
- Route display: "ride at 22 km/h for continuous greens on Meiji-dori"

#### Venues
- Source: OSM `amenity`, `shop`, `brand` tags (already in PostGIS from osm2pgsql)
- `Venue` — osm_id, geometry, name, category, brand, osm_tags (JSONB)
- **Hashtag resolution system:**
  - `VenueTagMapping` — hashtag → OSM filter expression
  - Brand-level: `#7-eleven` → `shop=convenience AND brand ILIKE '%seven%eleven%'`
  - Category-level: `#konbini` → `shop=convenience` (any convenience store)
  - Other examples: `#cafe`, `#bike-shop`, `#vending`, `#park`, `#toilet`
  - Unknown hashtags return unresolved for user refinement
  - Community can propose new tag mappings over time

### 5. RoutePlan

The primary ride planning object. Users build plans with ordered stops and tasks.

- `RoutePlan` (aggregate root) — ordered list of `StopPoint`s, departure_at, speed_model preference
- `StopPoint` — one of:
  - **Manual** — user-placed pin or address
  - **Venue-resolved** — from a #hashtag task, snapped to nearest matching venue in the route corridor
  - **Waypoint** — from a curated Route's existing waypoints
- `PlanTask` — text description + optional hashtag venue filter + status (unresolved/matched/completed)
- Minimum 2 stops (origin + destination), no maximum

**Planning flow:**
1. User sets origin + destination + departure time
2. Initial route computed via Valhalla with environment overlay
3. User adds stops — map taps or plan tasks like `"coffee at #cafe"`
4. Each new stop triggers re-route via Valhalla with updated via-points
5. Venue-resolved stops snap to nearest match along current route corridor (200m buffer)
6. Route line updates with new color-coded conditions reflecting adjusted ETAs

**Curated routes as templates:** Browsing a curated route → "Plan this ride" creates a RoutePlan pre-populated with the route's stops.

### Cross-context communication

In-process domain event bus (Go channels). Examples:
- Contribution approved → Route created (Community → Routes)
- RideLog with GPS track saved → Green wave inference triggered (Community → Environment)

No external message broker needed at this scale.

## Routing Engine

**Valhalla** (open source, Docker-friendly, reads OSM PBF directly).

- Dedicated bicycle costing model with surface/grade awareness
- Custom costing plugins for environment overlays
- Supports multi-stop via-points for RoutePlan stops
- Runs as a sidecar container

**Routing request flow:**
1. RoutePlan provides ordered stops + departure time + preference weights
2. Environment context produces cost overlay for the departure time window:
   - Shade map (from ShadowGrid at projected arrival times)
   - Greenery scores (from GreeneryIndex)
   - Wind penalty/bonus (from WeatherGrid, relative to route bearing)
   - Green wave bonus (from detected corridors)
   - Signal density penalty
3. Go API sends overlay + stops to Valhalla as custom costing weights
4. Valhalla returns optimized multi-leg route
5. Go API annotates result with time-projected conditions per segment

**User-tunable preference weights** (all 0.0–1.0):
- `shade` — how much to prefer shaded segments
- `greenery` — how much to prefer green/park-adjacent segments
- `wind` — how much to penalize headwind / reward tailwind

## Speed Model

Elevation-adjusted with signal and green wave corrections:

- **Base speed:** 15 km/h on flat
- **Uphill:** `base - (grade% × 1.5)` km/h, clamped at 4 km/h minimum
- **Downhill:** `base + (grade% × 1.0)` km/h, capped at 35 km/h
- **Signal penalty:** −30 seconds per traffic signal (average wait)
- **Green wave override:** on detected corridors, use the green wave target speed instead of elevation-adjusted speed

This produces per-segment ETAs. Weather/shade conditions are evaluated at each segment's projected arrival time, not a single departure-time snapshot.

## Time-Projected Route Conditions

The route is a journey through time. As the rider moves, conditions change.

**Per-segment response payload:**
```json
{
  "segments": [
    {
      "km": 0, "eta": "14:00",
      "shade": 0.8, "wind_benefit": 0.6, "precip": 0.0,
      "green_wave": null, "signals": 0
    },
    {
      "km": 5.1, "eta": "14:22",
      "shade": 0.3, "wind_benefit": -0.4, "precip": 0.3,
      "green_wave": { "speed_kmh": 20, "length_km": 1.8 }, "signals": 2
    }
  ]
}
```

**Color LUT rendering on map** (user toggles active overlay):

| Overlay | Color gradient | Meaning |
|---------|---------------|---------|
| Shade | yellow → deep blue | full sun → full shade at projected arrival |
| Wind | green → red | tailwind → headwind at projected arrival |
| Rain | white → dark purple | dry → heavy rain at projected arrival |

Rendered as MapLibre line-gradient on GeoJSON with interpolated color properties.

**Condition sparklines on route cards:** Mini horizontal bars summarizing shade/wind/rain distribution across the full route at departure time.

## Data Model (PostGIS)

### `routes` schema
- `route` — id (UUID), name, description, geometry (LINESTRING Z), distance_m, elevation_gain_m, elevation_loss_m, difficulty (enum), status (enum), creator_id, created_at, updated_at
- `waypoint` — id, route_id (FK), geometry (POINT), name, type (enum), sort_order
- `route_segment` — id, route_id (FK), geometry (LINESTRING Z), surface_type (enum), grade_percent, segment_order
- `route_tag` — route_id, tag (text)

### `community` schema
- `user` — id (UUID), display_name, email, avatar_url, created_at
- `contribution` — id, user_id, route_geometry (LINESTRING Z), metadata (JSONB), status (enum), moderator_notes, submitted_at
- `review` — id, user_id, route_id, rating (1-5), body, created_at
- `ride_log` — id, user_id, route_id, ridden_at, duration_s, gpx_track (LINESTRING Z, nullable)

### `environment` schema
- `shadow_grid` — id, cell_geometry (POLYGON), hour_slot (0-23), month, shade_coverage (0.0–1.0)
- `greenery_edge` — osm_way_id, greenery_score (0.0–1.0), tree_lined (bool), park_adjacent (bool)
- `weather_grid` — id, cell_geometry (POLYGON), valid_at (TIMESTAMPTZ), wind_speed_ms, wind_bearing_deg, precip_intensity_mmh, temperature_c
- `traffic_signal` — osm_node_id, geometry (POINT)
- `green_wave` — id, osm_way_ids (BIGINT[]), direction_bearing (FLOAT), target_speed_kmh (FLOAT), confidence (FLOAT), source (enum), detected_at
- `venue` — id, osm_id, geometry (POINT), name, category, brand (nullable), osm_tags (JSONB)
- `venue_tag_mapping` — hashtag (text PK), osm_filter (JSONB), description, is_brand (bool)

### `plan` schema
- `route_plan` — id (UUID), user_id, departure_at, speed_model (enum), shade_weight, greenery_weight, wind_weight, created_at
- `stop_point` — id, plan_id (FK), geometry (POINT), type (manual/venue_resolved/waypoint), sort_order, venue_id (FK nullable), resolved_name
- `plan_task` — id, plan_id (FK), description, hashtag (nullable), status (unresolved/matched/completed), resolved_venue_id (FK nullable)

### `osm` schema
- Managed by osm2pgsql — `planet_osm_line`, `planet_osm_point`, `planet_osm_polygon`
- Read-only for the Go API, used by Martin for tile serving

Spatial indexes (GiST) on all geometry columns.

## API Design

All responses JSON. All geometries GeoJSON. Cursor-based pagination on list endpoints.

### Routes
- `GET /api/v1/routes` — list/search (bbox, distance, difficulty, surface, tags)
- `GET /api/v1/routes/:id` — single route with waypoints, segments
- `GET /api/v1/routes/:id/conditions?departure_at=&speed_model=` — time-projected conditions
- `POST /api/v1/routes` — create (authenticated, curators)
- `PATCH /api/v1/routes/:id` — update metadata
- `DELETE /api/v1/routes/:id` — archive (soft delete)

### Discovery
- `GET /api/v1/discover/nearby?lat=&lon=&radius_km=` — routes near a point
- `GET /api/v1/discover/viewport?bbox=` — routes in map viewport
- `GET /api/v1/discover/suggested?lat=&lon=&departure_at=` — best routes now (conditions-ranked)

### Routing
- `POST /api/v1/routing/directions` — multi-stop environment-aware routing
  ```json
  {
    "stops": [
      { "type": "manual", "lat": 35.61, "lon": 139.67 },
      { "type": "venue", "hashtag": "#konbini" },
      { "type": "manual", "lat": 35.66, "lon": 139.54 }
    ],
    "departure_at": "2026-04-10T14:00:00+09:00",
    "speed_model": "elevation",
    "preferences": { "shade": 0.8, "greenery": 0.5, "wind": 0.6 }
  }
  ```
- `GET /api/v1/routing/conditions/preview?bbox=&departure_at=` — heatmap data for map overlays

### Plans
- `POST /api/v1/routes/:id/plans` — create plan from curated route
- `POST /api/v1/plans` — create plan from scratch
- `GET /api/v1/plans/:id` — get plan with resolved stops/tasks
- `POST /api/v1/plans/:id/stops` — add stop (triggers re-route)
- `POST /api/v1/plans/:id/tasks` — add task (triggers venue resolution)
- `DELETE /api/v1/plans/:id/stops/:stop_id` — remove stop (triggers re-route)

### Venues
- `GET /api/v1/venues/along-route?route_id=&type=&buffer_m=` — venues near a route
- `GET /api/v1/venues/tags` — list available hashtags

### Community
- `POST /api/v1/contributions` — submit route for moderation
- `GET /api/v1/routes/:id/reviews` — list reviews
- `POST /api/v1/routes/:id/reviews` — add review
- `POST /api/v1/routes/:id/ride-logs` — log a ride
- `GET /api/v1/users/:id/ride-logs` — user's ride history

### Auth
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login` — returns JWT
- `POST /api/v1/auth/refresh`

## Frontend

### Web — SvelteKit + MapLibre GL JS

**Map-first layout:** Map is the primary interface. Route list in left panel (desktop) / bottom sheet (mobile).

**Key pages:**
1. **Discovery** — map + route list, filters, departure time picker, environment overlay toggles (shade/wind/rain), color-coded route lines, condition sparklines on route cards
2. **Route Detail** — full route on map, elevation profile, segment conditions, waypoints, reviews, plan builder with #hashtag tasks
3. **Route Planner** — multi-stop point-to-point routing, preference sliders (shade/greenery/wind), live preview with color-coded conditions
4. **Profile / Ride Log** — user's rides, contributions, reviews, GPX upload

**Technical:**
- MapLibre GL JS for vector tile map rendering
- Shared map component with reactive Svelte stores
- SSR for discovery/route detail (SEO), CSR for interactive planner
- Line-gradient rendering for condition-colored routes

### iOS — SwiftUI + MapKit/MapLibre Native

Same four screens, consuming the identical Go API. Native map experience.

## Data Pipelines

Four offline jobs:

1. **OSM Import** — osm2pgsql, Tokyo region PBF, scheduled weekly
2. **PLATEAU Shadow Precompute** — CityGML → shadow grid per hour/month, runs on data update
3. **Weather Fetch** — Open-Meteo grid data → PostGIS, hourly
4. **Green Wave Inference** — analyze ride log GPS tracks for signal-pass patterns, runs periodically over new logs

## Go Project Structure

```
cyclist-map/
├── cmd/api/main.go
├── internal/
│   ├── domain/           # Pure domain logic, zero external imports
│   │   ├── route/        # Route, Segment, Waypoint, repository interface
│   │   ├── plan/         # RoutePlan, StopPoint, PlanTask
│   │   ├── community/    # User, Contribution, Review, RideLog
│   │   ├── environment/  # Shadow, Greenery, Weather, Venue, GreenWave, Signal, Conditions
│   │   ├── discovery/    # Search, ranking logic
│   │   └── events/       # Domain event definitions
│   ├── infra/            # Adapter implementations
│   │   ├── postgres/     # Repository implementations
│   │   ├── valhalla/     # Routing engine client
│   │   ├── openmeteo/    # Weather data fetcher
│   │   └── eventbus/     # In-process event bus
│   ├── app/              # Application services (use cases)
│   └── api/              # HTTP handlers + middleware
├── migrations/           # SQL migrations per schema
├── pipelines/            # Offline data jobs
│   ├── osm_import/
│   ├── plateau_shadow/
│   ├── weather_fetch/
│   └── green_wave/
├── web/                  # SvelteKit frontend
├── docker-compose.yml    # PostGIS + Martin + Valhalla + Go API
├── go.mod
└── README.md
```

## Technology Summary

| Component | Technology |
|-----------|-----------|
| API server | Go |
| Database | PostgreSQL + PostGIS |
| Tile server | Martin |
| Routing engine | Valhalla |
| 3D city data | PLATEAU (MLIT CityGML) |
| Weather data | Open-Meteo |
| Venue/map data | OpenStreetMap (via osm2pgsql) |
| Web frontend | SvelteKit + MapLibre GL JS |
| iOS frontend | SwiftUI + MapKit/MapLibre Native |
| Deployment | Docker Compose (self-hosted) |
| License | Open source |
