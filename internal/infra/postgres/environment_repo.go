package postgres

import (
	"context"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
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
func (r *EnvironmentRepo) GreenWaveForSegment(ctx context.Context, segmentWKT string) *environment.GreenWaveResult {
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
	var gw environment.GreenWaveResult
	if err := row.Scan(&gw.ID, &gw.TargetSpeedKmh, &gw.DirectionBearing, &gw.Confidence); err != nil {
		return nil // no green wave — safe default
	}
	return &gw
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
		_ = windSpeedMS
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
