package postgres

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
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

// shadowGridZone matches the timezone the plateau_shadow pipeline uses when
// it computes solar positions: hour_slot values are JST hours.
var shadowGridZone = time.FixedZone("JST", 9*60*60)

// ShadeForPoint returns shade_coverage (0–1) for the cell that contains
// (lon, lat) at the given time. Returns 0 when no cell covers the point.
//
// The environment.shadow_grid stores precomputed data per hour_slot (0-23,
// JST) for a set of representative months (e.g. 1/4/7/10), so the lookup
// matches the JST hour exactly and takes the cyclically nearest month that
// has data.
func (r *EnvironmentRepo) ShadeForPoint(ctx context.Context, lon, lat float64, at time.Time) float64 {
	var shade float64
	row := r.pool.QueryRow(ctx, `
		SELECT shade_coverage
		FROM environment.shadow_grid
		WHERE hour_slot = $1
		  AND ST_Contains(cell_geometry, ST_SetSRID(ST_MakePoint($3, $4), 4326))
		ORDER BY LEAST(ABS(month - $2), 12 - ABS(month - $2)) ASC
		LIMIT 1
	`, at.In(shadowGridZone).Hour(), int(at.In(shadowGridZone).Month()), lon, lat)
	if err := row.Scan(&shade); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("EnvironmentRepo.ShadeForPoint: %v", err)
	}
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
	if err := row.Scan(&windSpeedMS, &windBearingDeg, &precipMMH); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("EnvironmentRepo.WeatherForPoint: %v", err)
	}
	return
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
	if err := row.Scan(&count); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("EnvironmentRepo.SignalsAlongSegment: %v", err)
	}
	return count
}
