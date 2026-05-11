# Environment-Aware Routing — Implementation Plan

**Date:** 2026-04-10
**Author:** Claude Code
**Status:** Draft

---

## Context

The cyclist-map API can already route via Valhalla and return per-leg ETAs. This plan adds the environment layer: shade, weather, greenery, and traffic signals are queried at each segment's *projected arrival time*, and the results are returned as annotated segment conditions with hex color values for map overlay rendering. A new `EnvironmentService` orchestrates the four data sources; two new HTTP endpoints expose the data to clients.

All data sources are optional — missing rows (sparse shade or weather grids, no greenery edges on a given OSM way) return zero/default values and never hard-fail a request.

---

## Scope

1. `internal/app/environment_service.go` — orchestration + time projection
2. `internal/infra/postgres/environment_repo.go` — four spatial queries against the `environment` schema
3. `internal/domain/environment/color.go` — Color LUT computation
4. `internal/api/routing_handler.go` — two new endpoint methods
5. `internal/api/router.go` — wire the two new routes
6. Tests for each layer (table-driven, TDD)

**Out of scope (V2):** Valhalla cost overlay (feeding shade/wind weights back into Valhalla costing). For V1 we annotate the returned route; Valhalla always routes on its own bicycle cost model.

---

## Prerequisites

- `environment.shadow_grid`, `environment.weather_grid`, `environment.greenery_edge`, `environment.traffic_signal`, `environment.green_wave` tables already exist (schema defined in design spec, managed by migrations).
- Existing `route.Repository` can `GetByID` and return `Segments` with `GradePercent` and `Geometry`.
- `go-chi/chi/v5` and `jackc/pgx/v5` already in `go.mod`.

---

## Step Checklist

- [ ] Step 1 — Domain: `color.go` — Color LUT functions
- [ ] Step 2 — Infra: `environment_repo.go` — PostgreSQL queries for all four data sources
- [ ] Step 3 — App: `environment_service.go` — orchestration, time projection, composite scoring
- [ ] Step 4 — API: `routing_handler.go` — `RouteConditions` and `ConditionsPreview` handlers
- [ ] Step 5 — Wire: `router.go` — register new routes
- [ ] Step 6 — Tests

---

## Step 1 — Domain: Color LUT

**File:** `internal/domain/environment/color.go`

Purpose: given a normalised (0–1) value, return a hex color string by linearly interpolating between two anchor colors. Three named functions cover the three overlay types.

```go
package environment

import "fmt"

// lerpColor linearly interpolates between two RGB colors.
// t is clamped to [0, 1]; 0 returns c0, 1 returns c1.
func lerpColor(c0, c1 [3]uint8, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	r := uint8(float64(c0[0]) + t*float64(int(c1[0])-int(c0[0])))
	g := uint8(float64(c0[1]) + t*float64(int(c1[1])-int(c0[1])))
	b := uint8(float64(c0[2]) + t*float64(int(c1[2])-int(c0[2])))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

var (
	colorYellow  = [3]uint8{0xea, 0xb3, 0x08} // #eab308 — full sun
	colorDeepBlu = [3]uint8{0x1e, 0x3a, 0x8a} // #1e3a8a — full shade
	colorGreen   = [3]uint8{0x22, 0xc5, 0x5e} // #22c55e — tailwind
	colorRed     = [3]uint8{0xef, 0x44, 0x44} // #ef4444 — headwind
	colorWhite   = [3]uint8{0xf8, 0xfa, 0xfc} // #f8fafc — dry
	colorPurple  = [3]uint8{0x7c, 0x3a, 0xed} // #7c3aed — heavy rain
)

// ShadeColor returns a hex color for shade_coverage in [0, 1].
// 0 = full sun (yellow), 1 = full shade (deep blue).
func ShadeColor(shadeCoverage float64) string {
	return lerpColor(colorYellow, colorDeepBlu, shadeCoverage)
}

// WindColor returns a hex color for wind_benefit in [-1, 1].
// -1 = strong headwind (red), +1 = strong tailwind (green).
// Normalised to [0, 1] before interpolation (green → red).
func WindColor(windBenefit float64) string {
	// -1 → 1 (red), +1 → 0 (green) after normalisation
	t := (1 - windBenefit) / 2 // -1 → 1.0, 0 → 0.5, +1 → 0.0
	return lerpColor(colorGreen, colorRed, t)
}

// RainColor returns a hex color for precip intensity in [0, 1].
// 0 = dry (white), 1 = heavy rain (dark purple).
func RainColor(precipNorm float64) string {
	return lerpColor(colorWhite, colorPurple, precipNorm)
}
```

**Test file:** `internal/domain/environment/color_test.go`

```go
package environment

import "testing"

func TestShadeColor(t *testing.T) {
	tests := []struct {
		shade float64
		want  string
	}{
		{0, "#eab308"},   // full sun → yellow
		{1, "#1e3a8a"},   // full shade → deep blue
		{-1, "#eab308"},  // clamp below 0 → yellow
		{2, "#1e3a8a"},   // clamp above 1 → deep blue
	}
	for _, tc := range tests {
		got := ShadeColor(tc.shade)
		if got != tc.want {
			t.Errorf("ShadeColor(%v) = %q, want %q", tc.shade, got, tc.want)
		}
	}
}

func TestWindColor(t *testing.T) {
	// wind_benefit = +1 → pure tailwind → green
	if got := WindColor(1); got != "#22c55e" {
		t.Errorf("WindColor(1) = %q, want #22c55e", got)
	}
	// wind_benefit = -1 → pure headwind → red
	if got := WindColor(-1); got != "#ef4444" {
		t.Errorf("WindColor(-1) = %q, want #ef4444", got)
	}
}

func TestRainColor(t *testing.T) {
	if got := RainColor(0); got != "#f8fafc" {
		t.Errorf("RainColor(0) = %q, want #f8fafc", got)
	}
	if got := RainColor(1); got != "#7c3aed" {
		t.Errorf("RainColor(1) = %q, want #7c3aed", got)
	}
}
```

---

## Step 2 — Infra: Environment Repository

**File:** `internal/infra/postgres/environment_repo.go`

Four spatial queries. Every query uses `ST_Intersects` or nearest-point matching and always returns a safe default if no row is found — callers never need to distinguish "missing data" from "zero value".

```go
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvironmentRepo queries the environment schema for route-segment data.
// All methods are read-only and return zero/default values when no matching
// row exists, so callers always get a safe result regardless of data coverage.
type EnvironmentRepo struct {
	pool *pgxpool.Pool
}

// NewEnvironmentRepo creates a new EnvironmentRepo.
func NewEnvironmentRepo(pool *pgxpool.Pool) *EnvironmentRepo {
	return &EnvironmentRepo{pool: pool}
}

// ShadeForPoint returns shade_coverage (0–1) for the cell that contains
// (lon, lat) at the given time. Returns 0 when no cell covers the point.
//
// The environment.shadow_grid stores precomputed data per hour_slot (0-23)
// and month (1-12). We match on the arrival time's UTC hour and month.
func (r *EnvironmentRepo) ShadeForPoint(ctx context.Context, lon, lat float64, at time.Time) float64 {
	var shade float64
	row := r.pool.QueryRow(ctx, `
		SELECT shade_coverage
		FROM environment.shadow_grid
		WHERE hour_slot = $1
		  AND month     = $2
		  AND ST_Contains(cell_geometry, ST_SetSRID(ST_MakePoint($3, $4), 4326))
		LIMIT 1
	`, at.UTC().Hour(), int(at.UTC().Month()), lon, lat)
	_ = row.Scan(&shade) // ignore ErrNoRows — shade stays 0
	return shade
}

// WeatherForPoint returns wind_speed_ms, wind_bearing_deg, and
// precip_intensity_mmh for the cell nearest to (lon, lat) at the given time.
// It finds the closest valid_at slot within ±1 hour of the arrival time.
// Returns zeros when no weather data is available.
func (r *EnvironmentRepo) WeatherForPoint(ctx context.Context, lon, lat float64, at time.Time) (windSpeedMS, windBearingDeg, precipMMH float64) {
	row := r.pool.QueryRow(ctx, `
		SELECT wind_speed_ms, wind_bearing_deg, precip_intensity_mmh
		FROM environment.weather_grid
		WHERE ST_Contains(cell_geometry, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		  AND valid_at BETWEEN $3 AND $4
		ORDER BY ABS(EXTRACT(EPOCH FROM (valid_at - $5))) ASC
		LIMIT 1
	`, lon, lat,
		at.Add(-time.Hour), at.Add(time.Hour), at)
	_ = row.Scan(&windSpeedMS, &windBearingDeg, &precipMMH)
	return
}

// GreeneryForWay returns greenery_score (0–1) for the given OSM way ID.
// Returns 0 if no row exists for this way.
func (r *EnvironmentRepo) GreeneryForWay(ctx context.Context, osmWayID int64) float64 {
	var score float64
	row := r.pool.QueryRow(ctx, `
		SELECT greenery_score
		FROM environment.greenery_edge
		WHERE osm_way_id = $1
	`, osmWayID)
	_ = row.Scan(&score)
	return score
}

// SignalsAlongSegment returns the count of traffic signals within buffer_m
// metres of the given LINESTRING (WKT, SRID 4326).
func (r *EnvironmentRepo) SignalsAlongSegment(ctx context.Context, segmentWKT string, bufferM float64) int {
	var count int
	row := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM environment.traffic_signal
		WHERE ST_DWithin(
			geometry::geography,
			ST_GeomFromText($1, 4326)::geography,
			$2
		)
	`, segmentWKT, bufferM)
	_ = row.Scan(&count)
	return count
}

// GreenWaveForSegment returns the first active GreenWave corridor that
// overlaps the given LINESTRING (WKT), or nil if none exists.
func (r *EnvironmentRepo) GreenWaveForSegment(ctx context.Context, segmentWKT string) *greenWaveRow {
	row := r.pool.QueryRow(ctx, `
		SELECT gw.id, gw.target_speed_kmh, gw.direction_bearing, gw.confidence
		FROM environment.green_wave gw
		JOIN LATERAL (
			SELECT way.geom
			FROM osm.planet_osm_line way
			WHERE way.osm_id = ANY(gw.osm_way_ids)
			LIMIT 1
		) w ON true
		WHERE ST_Intersects(
			w.geom,
			ST_GeomFromText($1, 4326)
		)
		ORDER BY gw.confidence DESC
		LIMIT 1
	`, segmentWKT)
	var gw greenWaveRow
	if err := row.Scan(&gw.ID, &gw.TargetSpeedKmh, &gw.DirectionBearing, &gw.Confidence); err != nil {
		return nil // no green wave — safe default
	}
	return &gw
}

// greenWaveRow is a lightweight DTO used only within this package.
type greenWaveRow struct {
	ID               string
	TargetSpeedKmh   float64
	DirectionBearing float64
	Confidence       float64
}

// ConditionsPreviewCell is one heatmap cell for the conditions/preview endpoint.
type ConditionsPreviewCell struct {
	Lon         float64
	Lat         float64
	Shade       float64
	WindBenefit float64
	Precip      float64
}

// ConditionsPreview returns a grid of cells within the bounding box,
// joining shade and weather data at the given time. Used for map heatmaps.
// Returns an empty slice when no data covers the bbox.
func (r *EnvironmentRepo) ConditionsPreview(ctx context.Context, bbox [4]float64, at time.Time) ([]ConditionsPreviewCell, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			ST_X(ST_Centroid(sg.cell_geometry)) AS lon,
			ST_Y(ST_Centroid(sg.cell_geometry)) AS lat,
			sg.shade_coverage,
			COALESCE(wg.wind_speed_ms, 0)          AS wind_speed_ms,
			COALESCE(wg.wind_bearing_deg, 0)        AS wind_bearing_deg,
			COALESCE(wg.precip_intensity_mmh, 0)    AS precip_mmh
		FROM environment.shadow_grid sg
		LEFT JOIN environment.weather_grid wg
			ON ST_Intersects(sg.cell_geometry, wg.cell_geometry)
			AND wg.valid_at BETWEEN $5 AND $6
		WHERE sg.hour_slot = $1
		  AND sg.month     = $2
		  AND ST_Intersects(
			sg.cell_geometry,
			ST_MakeEnvelope($3, $4, $7, $8, 4326)
		  )
		LIMIT 500
	`,
		at.UTC().Hour(), int(at.UTC().Month()),
		bbox[0], bbox[1],
		at.Add(-time.Hour), at.Add(time.Hour),
		bbox[2], bbox[3],
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []ConditionsPreviewCell
	for rows.Next() {
		var c ConditionsPreviewCell
		var windSpeedMS, windBearingDeg, precipMMH float64
		if err := rows.Scan(&c.Lon, &c.Lat, &c.Shade, &windSpeedMS, &windBearingDeg, &precipMMH); err != nil {
			return nil, err
		}
		// wind_benefit is not directional here (no route bearing) — use 0 as neutral
		_ = windBearingDeg
		c.WindBenefit = 0
		c.Precip = normalisePrecip(precipMMH)
		cells = append(cells, c)
	}
	if cells == nil {
		cells = []ConditionsPreviewCell{}
	}
	return cells, rows.Err()
}

// normalisePrecip maps precip_intensity_mmh to [0, 1].
// 0 mm/h → 0.0; ≥10 mm/h → 1.0 (heavy rain threshold).
func normalisePrecip(mmh float64) float64 {
	if mmh <= 0 {
		return 0
	}
	if mmh >= 10 {
		return 1
	}
	return mmh / 10
}
```

---

## Step 3 — App: Environment Service

**File:** `internal/app/environment_service.go`

This is the core orchestrator. Given a `route.Route` (with `Segments`) and a departure time, it:

1. Computes each segment's centroid (lon/lat midpoint of the segment geometry).
2. Uses `AdjustedSpeedKmh` (or green wave speed) to estimate travel time per segment.
3. Accumulates elapsed time to derive the projected arrival time at each segment.
4. Queries shade, weather, greenery, and signals at that arrival time and centroid.
5. Computes `wind_benefit` from wind speed + bearing relative to segment bearing.
6. Assembles `SegmentConditions` slices.

```go
package app

import (
	"context"
	"math"
	"strings"
	"time"

	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
	"komorebi/internal/domain/route"
)

// EnvironmentQuerier is the repository interface the EnvironmentService depends on.
// Defined here so tests can inject a stub without importing infra/postgres.
type EnvironmentQuerier interface {
	ShadeForPoint(ctx context.Context, lon, lat float64, at time.Time) float64
	WeatherForPoint(ctx context.Context, lon, lat float64, at time.Time) (windSpeedMS, windBearingDeg, precipMMH float64)
	GreeneryForWay(ctx context.Context, osmWayID int64) float64
	SignalsAlongSegment(ctx context.Context, segmentWKT string, bufferM float64) int
	GreenWaveForSegment(ctx context.Context, segmentWKT string) *GreenWaveResult
}

// GreenWaveResult is the cross-boundary type that EnvironmentQuerier returns.
// (The infra layer's internal greenWaveRow satisfies this after a small adapter.)
type GreenWaveResult struct {
	ID               string
	TargetSpeedKmh   float64
	DirectionBearing float64
	Confidence       float64
}

// EnvironmentService computes time-projected per-segment conditions for a route.
type EnvironmentService struct {
	repo EnvironmentQuerier
}

// NewEnvironmentService creates an EnvironmentService backed by the given querier.
func NewEnvironmentService(repo EnvironmentQuerier) *EnvironmentService {
	return &EnvironmentService{repo: repo}
}

// SegmentConditionsResult extends the domain type with color values for all
// three overlays, computed by the Color LUT functions.
type SegmentConditionsResult struct {
	environment.SegmentConditions
	ShadeColor string
	WindColor  string
	RainColor  string
}

// RouteConditionsRequest carries the inputs needed to project conditions.
type RouteConditionsRequest struct {
	Route       *route.Route
	DepartureAt time.Time
	SpeedModel  plan.SpeedModel
}

// GetRouteConditions returns per-segment environment conditions for the route,
// projected forward in time from DepartureAt using the speed model.
//
// If any data source returns no rows for a segment, that segment's values
// default to zero — the function never returns an error due to missing env data.
func (s *EnvironmentService) GetRouteConditions(ctx context.Context, req RouteConditionsRequest) ([]SegmentConditionsResult, error) {
	segments := req.Route.Segments
	if len(segments) == 0 {
		return []SegmentConditionsResult{}, nil
	}

	results := make([]SegmentConditionsResult, 0, len(segments))
	elapsedS := 0.0
	cumulativeKm := 0.0

	for _, seg := range segments {
		distKm := segmentDistanceKm(seg.Geometry)
		centLon, centLat := segmentCentroid(seg.Geometry)
		segWKT := geometryToLineStringWKT(seg.Geometry)
		segBearingDeg := segmentBearing(seg.Geometry)

		// Check for green wave on this segment.
		gwRow := s.repo.GreenWaveForSegment(ctx, segWKT)
		var gwOverride *environment.GreenWaveOverride
		var gwDomain *environment.GreenWave
		if gwRow != nil {
			gwOverride = &environment.GreenWaveOverride{TargetSpeedKmh: gwRow.TargetSpeedKmh}
			gwDomain = &environment.GreenWave{
				ID:               gwRow.ID,
				TargetSpeedKmh:   gwRow.TargetSpeedKmh,
				DirectionBearing: gwRow.DirectionBearing,
				Confidence:       gwRow.Confidence,
			}
		}

		// Signal count (50 m buffer around segment).
		signals := s.repo.SignalsAlongSegment(ctx, segWKT, 50)

		// Speed for this segment.
		speedKmh := adjustedSpeed(seg.GradePercent, req.SpeedModel)

		// ETA seconds for this segment.
		etaS := environment.SegmentETASeconds(distKm, speedKmh, signals, gwOverride)

		// Project arrival time at the midpoint of this segment.
		arrivalAt := req.DepartureAt.Add(time.Duration(elapsedS+etaS/2) * time.Second)

		// Query environment data at projected arrival time.
		shade := s.repo.ShadeForPoint(ctx, centLon, centLat, arrivalAt)
		windSpeedMS, windBearingDeg, precipMMH := s.repo.WeatherForPoint(ctx, centLon, centLat, arrivalAt)

		windBenefit := computeWindBenefit(windSpeedMS, windBearingDeg, segBearingDeg)
		precipNorm := normalisePrecipApp(precipMMH)

		sc := environment.SegmentConditions{
			Km:          cumulativeKm,
			Shade:       shade,
			WindBenefit: windBenefit,
			Precip:      precipNorm,
			ETA:         req.DepartureAt.Add(time.Duration(elapsedS) * time.Second),
			GreenWave:   gwDomain,
			SignalCount: signals,
		}

		results = append(results, SegmentConditionsResult{
			SegmentConditions: sc,
			ShadeColor:        environment.ShadeColor(shade),
			WindColor:         environment.WindColor(windBenefit),
			RainColor:         environment.RainColor(precipNorm),
		})

		elapsedS += etaS
		cumulativeKm += distKm
	}

	return results, nil
}

// --- speed helper ---

// adjustedSpeed returns cycling speed for a segment based on grade and model.
// SpeedModelFlat always returns the base 15 km/h regardless of grade.
func adjustedSpeed(gradePercent float64, model plan.SpeedModel) float64 {
	if model == plan.SpeedModelFlat {
		return environment.AdjustedSpeedKmh(0)
	}
	return environment.AdjustedSpeedKmh(gradePercent)
}

// --- wind computation ---

// computeWindBenefit returns wind_benefit in [-1, +1].
//
// Tailwind (wind blows same direction as travel) → positive.
// Headwind (wind blows opposite to travel) → negative.
// Crosswind → near zero.
//
// Formula: benefit = cos(angle_diff) * clamp(wind_speed / 10, 0, 1)
// where angle_diff is the difference between route bearing and wind origin.
//
// Returns 0 when wind speed is zero (no data).
func computeWindBenefit(windSpeedMS, windBearingDeg, routeBearingDeg float64) float64 {
	if windSpeedMS == 0 {
		return 0
	}
	// Wind bearing is the direction the wind is coming FROM.
	// Tailwind means wind comes from behind the rider (from opposite of route bearing).
	windFromDeg := windBearingDeg
	angleDiff := routeBearingDeg - (windFromDeg + 180)
	angleDiff = math.Mod(angleDiff+360, 360)
	if angleDiff > 180 {
		angleDiff -= 360
	}
	cosAngle := math.Cos(angleDiff * math.Pi / 180)
	speedFactor := windSpeedMS / 10
	if speedFactor > 1 {
		speedFactor = 1
	}
	return cosAngle * speedFactor
}

// --- geometry helpers ---

// segmentDistanceKm computes the total arc length of a segment in km
// using the haversine formula between consecutive coordinate pairs.
func segmentDistanceKm(coords [][3]float64) float64 {
	if len(coords) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(coords); i++ {
		total += haversineKm(coords[i-1][1], coords[i-1][0], coords[i][1], coords[i][0])
	}
	return total
}

// segmentCentroid returns the midpoint coordinate of the segment geometry.
func segmentCentroid(coords [][3]float64) (lon, lat float64) {
	if len(coords) == 0 {
		return 0, 0
	}
	mid := coords[len(coords)/2]
	return mid[0], mid[1]
}

// segmentBearing returns the approximate compass bearing (degrees, 0=N, 90=E)
// from the first to the last point of the segment.
func segmentBearing(coords [][3]float64) float64 {
	if len(coords) < 2 {
		return 0
	}
	first := coords[0]
	last := coords[len(coords)-1]
	dLon := (last[0] - first[0]) * math.Pi / 180
	lat1 := first[1] * math.Pi / 180
	lat2 := last[1] * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}

// geometryToLineStringWKT encodes a segment geometry as a WKT LINESTRING Z.
func geometryToLineStringWKT(coords [][3]float64) string {
	if len(coords) == 0 {
		return "LINESTRING Z EMPTY"
	}
	pts := make([]string, len(coords))
	for i, c := range coords {
		pts[i] = formatCoord(c[0], c[1], c[2])
	}
	return "LINESTRING Z(" + strings.Join(pts, ", ") + ")"
}

func formatCoord(lon, lat, z float64) string {
	return fmt.Sprintf("%f %f %f", lon, lat, z)
}

// haversineKm returns the great-circle distance in km between two lat/lon points.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// normalisePrecipApp maps precip_intensity_mmh to [0, 1] (0 mm/h → 0; ≥10 mm/h → 1).
func normalisePrecipApp(mmh float64) float64 {
	if mmh <= 0 {
		return 0
	}
	if mmh >= 10 {
		return 1
	}
	return mmh / 10
}
```

Note: `fmt` needs to be added to the import block — shown here separately for clarity. The actual file imports `"fmt"`, `"math"`, `"strings"`, `"time"`, and the three domain packages.

---

## Step 4 — API: New Handler Methods

**File:** `internal/api/routing_handler.go` (additions to existing file)

### 4a — Route conditions endpoint

`GET /api/v1/routes/:id/conditions?departure_at=&speed_model=`

The handler needs access to both the route repository and the environment service. To avoid changing `RoutingHandler`'s signature we introduce a separate `ConditionsHandler`.

```go
// ConditionsQuerier is the read interface the handler needs from the route service.
type ConditionsQuerier interface {
	GetByID(id string) (*route.Route, error)
}

// ConditionsComputer is the interface the handler uses to project conditions.
type ConditionsComputer interface {
	GetRouteConditions(ctx context.Context, req app.RouteConditionsRequest) ([]app.SegmentConditionsResult, error)
}

// ConditionsHandler handles the route conditions and preview endpoints.
type ConditionsHandler struct {
	routes ConditionsQuerier
	env    ConditionsComputer
}

// NewConditionsHandler creates a ConditionsHandler.
func NewConditionsHandler(routes ConditionsQuerier, env ConditionsComputer) *ConditionsHandler {
	return &ConditionsHandler{routes: routes, env: env}
}
```

**Response type:**

```go
type greenWaveJSON struct {
	SpeedKmh float64 `json:"speed_kmh"`
	LengthKm float64 `json:"length_km,omitempty"`
}

type segmentConditionJSON struct {
	Km          float64        `json:"km"`
	ETA         string         `json:"eta"`
	Shade       float64        `json:"shade"`
	WindBenefit float64        `json:"wind_benefit"`
	Precip      float64        `json:"precip"`
	GreenWave   *greenWaveJSON `json:"green_wave"`
	Signals     int            `json:"signals"`
	Colors      struct {
		Shade string `json:"shade"`
		Wind  string `json:"wind"`
		Rain  string `json:"rain"`
	} `json:"colors"`
}

type routeConditionsResponse struct {
	RouteID   string                 `json:"route_id"`
	Segments  []segmentConditionJSON `json:"segments"`
}
```

**Handler method:**

```go
// RouteConditions handles GET /api/v1/routes/:id/conditions
func (h *ConditionsHandler) RouteConditions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	departureAt := time.Now()
	if raw := r.URL.Query().Get("departure_at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339 format")
			return
		}
		departureAt = parsed
	}

	speedModel := plan.SpeedModelElevation
	if r.URL.Query().Get("speed_model") == string(plan.SpeedModelFlat) {
		speedModel = plan.SpeedModelFlat
	}

	rt, err := h.routes.GetByID(id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load route")
		return
	}

	conditions, err := h.env.GetRouteConditions(r.Context(), app.RouteConditionsRequest{
		Route:       rt,
		DepartureAt: departureAt,
		SpeedModel:  speedModel,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute conditions")
		return
	}

	segs := make([]segmentConditionJSON, len(conditions))
	for i, c := range conditions {
		sj := segmentConditionJSON{
			Km:          c.Km,
			ETA:         c.ETA.Format("15:04"),
			Shade:       c.Shade,
			WindBenefit: c.WindBenefit,
			Precip:      c.Precip,
			Signals:     c.SignalCount,
		}
		sj.Colors.Shade = c.ShadeColor
		sj.Colors.Wind = c.WindColor
		sj.Colors.Rain = c.RainColor
		if c.GreenWave != nil {
			sj.GreenWave = &greenWaveJSON{SpeedKmh: c.GreenWave.TargetSpeedKmh}
		}
		segs[i] = sj
	}

	writeJSON(w, http.StatusOK, routeConditionsResponse{
		RouteID:  id,
		Segments: segs,
	})
}
```

### 4b — Conditions preview endpoint

`GET /api/v1/routing/conditions/preview?bbox=minLon,minLat,maxLon,maxLat&departure_at=`

```go
// ConditionsPreviewQuerier is the repo interface for the preview endpoint.
type ConditionsPreviewQuerier interface {
	ConditionsPreview(ctx context.Context, bbox [4]float64, at time.Time) ([]postgres.ConditionsPreviewCell, error)
}

// PreviewHandler handles the heatmap preview endpoint.
type PreviewHandler struct {
	repo ConditionsPreviewQuerier
}

// NewPreviewHandler creates a PreviewHandler.
func NewPreviewHandler(repo ConditionsPreviewQuerier) *PreviewHandler {
	return &PreviewHandler{repo: repo}
}

type previewCellJSON struct {
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	Shade       float64 `json:"shade"`
	WindBenefit float64 `json:"wind_benefit"`
	Precip      float64 `json:"precip"`
	Colors      struct {
		Shade string `json:"shade"`
		Wind  string `json:"wind"`
		Rain  string `json:"rain"`
	} `json:"colors"`
}

// ConditionsPreview handles GET /api/v1/routing/conditions/preview
func (h *PreviewHandler) ConditionsPreview(w http.ResponseWriter, r *http.Request) {
	bboxStr := r.URL.Query().Get("bbox")
	if bboxStr == "" {
		writeError(w, http.StatusBadRequest, "bbox parameter required (minLon,minLat,maxLon,maxLat)")
		return
	}
	var bbox [4]float64
	_, err := fmt.Sscanf(bboxStr, "%f,%f,%f,%f", &bbox[0], &bbox[1], &bbox[2], &bbox[3])
	if err != nil {
		writeError(w, http.StatusBadRequest, "bbox must be minLon,minLat,maxLon,maxLat")
		return
	}

	departureAt := time.Now()
	if raw := r.URL.Query().Get("departure_at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339 format")
			return
		}
		departureAt = parsed
	}

	cells, err := h.repo.ConditionsPreview(r.Context(), bbox, departureAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load conditions preview")
		return
	}

	out := make([]previewCellJSON, len(cells))
	for i, c := range cells {
		pj := previewCellJSON{
			Lon:         c.Lon,
			Lat:         c.Lat,
			Shade:       c.Shade,
			WindBenefit: c.WindBenefit,
			Precip:      c.Precip,
		}
		pj.Colors.Shade = environment.ShadeColor(c.Shade)
		pj.Colors.Wind = environment.WindColor(c.WindBenefit)
		pj.Colors.Rain = environment.RainColor(c.Precip)
		out[i] = pj
	}

	writeJSON(w, http.StatusOK, map[string]any{"cells": out})
}
```

---

## Step 5 — Wire: Router Updates

**File:** `internal/api/router.go`

Add `ConditionsHandler` and `PreviewHandler` parameters to `NewRouter` and register the two new routes:

```go
func NewRouter(
	routeSvc      *app.RouteService,
	discoverySvc  *app.DiscoveryService,
	venueSvc      *app.VenueService,
	routingH      *RoutingHandler,
	conditionsH   *ConditionsHandler,
	previewH      *PreviewHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	rh := &RouteHandler{svc: routeSvc}
	dh := NewDiscoveryHandler(discoverySvc)
	vh := NewVenueHandler(venueSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Routes
		r.Get("/routes", rh.List)
		r.Post("/routes", rh.Create)
		r.Get("/routes/{id}", rh.Get)
		r.Patch("/routes/{id}", rh.Update)
		r.Delete("/routes/{id}", rh.Archive)
		r.Get("/routes/{id}/conditions", conditionsH.RouteConditions) // NEW

		// Discovery
		r.Get("/discover/nearby", dh.Nearby)
		r.Get("/discover/viewport", dh.Viewport)
		r.Get("/discover/suggested", dh.Suggested)

		// Venues
		r.Get("/venues/along-route", vh.AlongRoute)
		r.Get("/venues/tags", vh.Tags)

		// Routing
		r.Post("/routing/directions", routingH.Directions)
		r.Get("/routing/conditions/preview", previewH.ConditionsPreview) // NEW
	})
	return r
}
```

**File:** `cmd/api/main.go` — wire up the new services after constructing `pgxpool`:

```go
envRepo         := postgres.NewEnvironmentRepo(pool)
routeRepo       := postgres.NewRouteRepo(pool)
envSvc          := app.NewEnvironmentService(envRepo)
conditionsH     := api.NewConditionsHandler(routeRepo, envSvc)
previewH        := api.NewPreviewHandler(envRepo)
```

Pass `conditionsH` and `previewH` into `api.NewRouter(...)`.

---

## Step 6 — Tests

### 6a — `color_test.go` (Step 1 above — full table-driven)

Already shown in Step 1.

### 6b — `environment_service_test.go`

**File:** `internal/app/environment_service_test.go`

Use a stub `EnvironmentQuerier` that returns controlled values. Tests verify:
- zero-value graceful handling (all env queries return 0)
- ETA accumulation (elapsed time between segments)
- wind_benefit sign (tailwind positive, headwind negative)
- green wave override bypasses signal penalty

```go
package app

import (
	"context"
	"testing"
	"time"

	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
	"komorebi/internal/domain/route"
)

// stubEnvRepo returns fixed values for all queries.
type stubEnvRepo struct {
	shade       float64
	windSpeedMS float64
	windBearing float64
	precipMMH   float64
	signals     int
	greenWave   *GreenWaveResult
}

func (s *stubEnvRepo) ShadeForPoint(_ context.Context, _, _ float64, _ time.Time) float64 {
	return s.shade
}
func (s *stubEnvRepo) WeatherForPoint(_ context.Context, _, _ float64, _ time.Time) (float64, float64, float64) {
	return s.windSpeedMS, s.windBearing, s.precipMMH
}
func (s *stubEnvRepo) GreeneryForWay(_ context.Context, _ int64) float64 { return 0 }
func (s *stubEnvRepo) SignalsAlongSegment(_ context.Context, _ string, _ float64) int {
	return s.signals
}
func (s *stubEnvRepo) GreenWaveForSegment(_ context.Context, _ string) *GreenWaveResult {
	return s.greenWave
}

func makeTestRoute(gradePercent float64) *route.Route {
	// Simple two-point segment ~1 km long (roughly), flat.
	return &route.Route{
		ID: "test-route",
		Segments: []route.Segment{
			{
				ID:           "seg-1",
				GradePercent: gradePercent,
				SegmentOrder: 0,
				// ~1 km segment in Tokyo area
				Geometry: [][3]float64{
					{139.700, 35.680, 0},
					{139.710, 35.680, 0},
				},
			},
		},
	}
}

func TestGetRouteConditions_ZeroData(t *testing.T) {
	svc := NewEnvironmentService(&stubEnvRepo{})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       makeTestRoute(0),
		DepartureAt: time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 segment, got %d", len(results))
	}
	seg := results[0]
	if seg.Shade != 0 {
		t.Errorf("shade: got %v, want 0", seg.Shade)
	}
	if seg.WindBenefit != 0 {
		t.Errorf("wind_benefit: got %v, want 0", seg.WindBenefit)
	}
	if seg.Precip != 0 {
		t.Errorf("precip: got %v, want 0", seg.Precip)
	}
	if seg.ShadeColor == "" {
		t.Error("shade color must not be empty")
	}
}

func TestGetRouteConditions_ETAAccumulation(t *testing.T) {
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	svc := NewEnvironmentService(&stubEnvRepo{signals: 1})

	rt := makeTestRoute(0)
	// Add a second segment
	rt.Segments = append(rt.Segments, route.Segment{
		ID:           "seg-2",
		GradePercent: 0,
		SegmentOrder: 1,
		Geometry: [][3]float64{
			{139.710, 35.680, 0},
			{139.720, 35.680, 0},
		},
	})

	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       rt,
		DepartureAt: departure,
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 segments, got %d", len(results))
	}
	// Second segment ETA must be after first
	if !results[1].ETA.After(results[0].ETA) {
		t.Errorf("seg[1].ETA (%v) should be after seg[0].ETA (%v)", results[1].ETA, results[0].ETA)
	}
}

func TestGetRouteConditions_TailwindPositive(t *testing.T) {
	// Route bearing: east (~90°). Wind from west (270°) = tailwind.
	svc := NewEnvironmentService(&stubEnvRepo{
		windSpeedMS: 5,
		windBearing: 270, // wind FROM west
	})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       makeTestRoute(0),
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].WindBenefit <= 0 {
		t.Errorf("expected positive wind_benefit for tailwind, got %v", results[0].WindBenefit)
	}
}

func TestGetRouteConditions_HeadwindNegative(t *testing.T) {
	// Route bearing: east (~90°). Wind from east (90°) = headwind.
	svc := NewEnvironmentService(&stubEnvRepo{
		windSpeedMS: 5,
		windBearing: 90, // wind FROM east
	})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       makeTestRoute(0),
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].WindBenefit >= 0 {
		t.Errorf("expected negative wind_benefit for headwind, got %v", results[0].WindBenefit)
	}
}

func TestGetRouteConditions_EmptySegments(t *testing.T) {
	svc := NewEnvironmentService(&stubEnvRepo{})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       &route.Route{ID: "empty"},
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("want 0 results for empty route, got %d", len(results))
	}
}

func TestComputeWindBenefit_ZeroSpeed(t *testing.T) {
	got := computeWindBenefit(0, 90, 90)
	if got != 0 {
		t.Errorf("expected 0 for zero wind speed, got %v", got)
	}
}

func TestNormalisePrecipApp(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{0, 0},
		{5, 0.5},
		{10, 1},
		{20, 1},
		{-1, 0},
	}
	for _, tc := range tests {
		got := normalisePrecipApp(tc.in)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("normalisePrecipApp(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
```

### 6c — `conditions_handler_test.go`

**File:** `internal/api/conditions_handler_test.go`

Tests the HTTP layer with a stub `ConditionsComputer` and `ConditionsQuerier`. Verifies status codes, JSON shape, and graceful handling of missing route.

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
	"komorebi/internal/domain/route"
	"komorebi/internal/infra/postgres"
)

// --- stubs ---

type stubRouteGetter struct {
	rt  *route.Route
	err error
}

func (s *stubRouteGetter) GetByID(_ string) (*route.Route, error) {
	return s.rt, s.err
}

type stubConditionsComputer struct {
	results []app.SegmentConditionsResult
	err     error
}

func (s *stubConditionsComputer) GetRouteConditions(_ context.Context, _ app.RouteConditionsRequest) ([]app.SegmentConditionsResult, error) {
	return s.results, s.err
}

func routeConditionsRequest(routeID, departureAt, speedModel string) *http.Request {
	url := "/api/v1/routes/" + routeID + "/conditions"
	if departureAt != "" || speedModel != "" {
		url += "?"
		if departureAt != "" {
			url += "departure_at=" + departureAt
		}
		if speedModel != "" {
			if departureAt != "" {
				url += "&"
			}
			url += "speed_model=" + speedModel
		}
	}
	r := httptest.NewRequest(http.MethodGet, url, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", routeID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestConditionsHandler_RouteNotFound(t *testing.T) {
	h := NewConditionsHandler(
		&stubRouteGetter{err: postgres.ErrNotFound},
		&stubConditionsComputer{},
	)
	w := httptest.NewRecorder()
	h.RouteConditions(w, routeConditionsRequest("bad-id", "", ""))
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestConditionsHandler_Success(t *testing.T) {
	rt := &route.Route{ID: "route-1"}
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	comp := &stubConditionsComputer{
		results: []app.SegmentConditionsResult{
			{
				SegmentConditions: environment.SegmentConditions{
					Km:          0,
					Shade:       0.7,
					WindBenefit: 0.3,
					Precip:      0.0,
					ETA:         departure,
					SignalCount: 1,
				},
				ShadeColor: "#abc123",
				WindColor:  "#22c55e",
				RainColor:  "#f8fafc",
			},
		},
	}
	h := NewConditionsHandler(&stubRouteGetter{rt: rt}, comp)
	req := routeConditionsRequest("route-1", "2026-04-10T14:00:00Z", "elevation")
	w := httptest.NewRecorder()
	h.RouteConditions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp routeConditionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RouteID != "route-1" {
		t.Errorf("route_id: got %q, want %q", resp.RouteID, "route-1")
	}
	if len(resp.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d", len(resp.Segments))
	}
	seg := resp.Segments[0]
	if seg.Shade != 0.7 {
		t.Errorf("shade: got %v, want 0.7", seg.Shade)
	}
	if seg.Colors.Shade != "#abc123" {
		t.Errorf("color.shade: got %q, want #abc123", seg.Colors.Shade)
	}
}

func TestConditionsHandler_InvalidDepartureAt(t *testing.T) {
	h := NewConditionsHandler(&stubRouteGetter{rt: &route.Route{ID: "x"}}, &stubConditionsComputer{})
	req := routeConditionsRequest("x", "not-a-date", "")
	w := httptest.NewRecorder()
	h.RouteConditions(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
```

---

## Infra: Adapter for `GreenWaveResult`

`EnvironmentRepo.GreenWaveForSegment` currently returns `*greenWaveRow` (unexported). The `EnvironmentService` expects `*app.GreenWaveResult`. Add a thin exported adapter method on `EnvironmentRepo`, or make `GreenWaveForSegment` return the exported type directly:

**Option (cleanest):** Change `GreenWaveForSegment` signature to return `*app.GreenWaveResult` — this creates a circular import (`infra/postgres` → `app`). Avoid that.

**Correct approach:** Define `GreenWaveResult` in the `environment` domain package instead of `app`. Then both `infra/postgres` and `app` can import it without cycles:

```go
// internal/domain/environment/green_wave.go (addition)

// GreenWaveResult is the query result type returned by environment repositories.
// It is a lightweight DTO separate from the full GreenWave aggregate.
type GreenWaveResult struct {
	ID               string
	TargetSpeedKmh   float64
	DirectionBearing float64
	Confidence       float64
}
```

Update `EnvironmentQuerier` interface in `environment_service.go` to use `*environment.GreenWaveResult`, and update `EnvironmentRepo.GreenWaveForSegment` to return `*environment.GreenWaveResult`.

---

## Graceful Degradation Summary

| Data source | Missing-data behaviour |
|-------------|----------------------|
| `shadow_grid` — no cell covers point | `ShadeForPoint` returns `0.0` (full sun) |
| `weather_grid` — no row within ±1 hr | `WeatherForPoint` returns `0, 0, 0` → `wind_benefit = 0`, `precip = 0` |
| `greenery_edge` — way not in table | `GreeneryForWay` returns `0.0` |
| `traffic_signal` — no signals near segment | `SignalsAlongSegment` returns `0` |
| `green_wave` — no corridor intersects | `GreenWaveForSegment` returns `nil` → normal speed model applies |
| Route has no segments | `GetRouteConditions` returns empty slice, 200 OK |
| `ConditionsPreview` — empty bbox | Returns `{"cells":[]}`, 200 OK |

---

## Self-Review Checklist

- [ ] `color.go`: clamp inputs before interpolation to prevent overflow
- [ ] `environment_repo.go`: every `row.Scan` error is silently dropped (returns zero default) — this is intentional and documented
- [ ] `SignalsAlongSegment`: uses `::geography` cast for accurate metre-based distance on PostGIS
- [ ] `GreenWaveForSegment`: joins `osm.planet_osm_line` (read-only osm2pgsql schema) — confirm `osm_id` column name matches osm2pgsql output (`osm_id BIGINT`)
- [ ] `ConditionsPreview`: LIMIT 500 prevents unbounded results on large bbox — add index hint if query is slow: `GiST index on environment.shadow_grid(cell_geometry)`
- [ ] `computeWindBenefit`: verify angle arithmetic with unit tests for 45°/135°/225°/315° cases
- [ ] `geometryToLineStringWKT`: empty geometry returns `"LINESTRING Z EMPTY"` which PostGIS accepts as `ST_DWithin` input — confirm with a manual query
- [ ] Handler tests cover: 200 success, 400 bad departure_at, 404 route not found, 500 env service error
- [ ] `NewRouter` signature change is backward-compatible with `main.go` — update `main.go` construction accordingly
- [ ] No circular imports: `domain/environment` ← `app` ← `api` ← `infra/postgres` ← `domain/environment` (only one direction each)
- [ ] `GreenWaveResult` defined in `domain/environment` to avoid import cycle between `app` and `infra/postgres`

---

## Execution Order

1. Write and pass `color_test.go` → implement `color.go`
2. Write and pass `environment_service_test.go` → implement `environment_service.go`
3. Implement `environment_repo.go` (no unit tests — integration tests against real DB are out of scope for this plan)
4. Add `GreenWaveResult` to `domain/environment/green_wave.go`
5. Write and pass `conditions_handler_test.go` → implement handler additions
6. Update `router.go` and `main.go`
7. Run `go build ./...` and `go test ./...`
