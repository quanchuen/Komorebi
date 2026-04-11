package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WeatherRepo implements environment.WeatherRepository using PostGIS.
type WeatherRepo struct {
	pool *pgxpool.Pool
}

// NewWeatherRepo creates a WeatherRepo.
func NewWeatherRepo(pool *pgxpool.Pool) *WeatherRepo {
	return &WeatherRepo{pool: pool}
}

// Upsert inserts weather grid rows, replacing existing rows with the same
// (grid_lat, grid_lon, valid_at) key. Runs inside a single transaction.
func (r *WeatherRepo) Upsert(cells []environment.WeatherGrid) error {
	if len(cells) == 0 {
		return nil
	}
	ctx := context.Background()
	return r.weatherWithTx(ctx, func(tx pgx.Tx) error {
		for _, c := range cells {
			poly := polygonWKT(c.CellGeometry)
			_, err := tx.Exec(ctx, `
				INSERT INTO environment.weather_grid
					(cell_geometry, valid_at, wind_speed_ms, wind_bearing_deg,
					 precip_intensity_mmh, temperature_c)
				VALUES (
					ST_GeomFromText($1, 4326), $2, $3, $4, $5, $6
				)
				ON CONFLICT (grid_lat, grid_lon, valid_at)
				DO UPDATE SET
					wind_speed_ms        = EXCLUDED.wind_speed_ms,
					wind_bearing_deg     = EXCLUDED.wind_bearing_deg,
					precip_intensity_mmh = EXCLUDED.precip_intensity_mmh,
					temperature_c        = EXCLUDED.temperature_c
			`, poly, c.ValidAt,
				c.WindSpeedMS, c.WindBearingDeg, c.PrecipIntensityMMH, c.TemperatureC)
			if err != nil {
				return fmt.Errorf("weather upsert: %w", err)
			}
		}
		return nil
	})
}

// AtPoint returns the nearest weather grid cell containing (lat, lon) with the
// closest valid_at to t (within +/-1 hour). Returns environment.ErrNoWeather when
// no row qualifies.
func (r *WeatherRepo) AtPoint(lat, lon float64, t time.Time) (*environment.WeatherGrid, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT
			id::text,
			ST_AsText(cell_geometry),
			valid_at,
			wind_speed_ms,
			wind_bearing_deg,
			precip_intensity_mmh,
			temperature_c
		FROM environment.weather_grid
		WHERE ST_Contains(
				  cell_geometry,
				  ST_SetSRID(ST_MakePoint($1, $2), 4326)
			  )
		  AND valid_at BETWEEN $3 AND $4
		ORDER BY ABS(EXTRACT(EPOCH FROM (valid_at - $5))) ASC
		LIMIT 1
	`, lon, lat,
		t.Add(-time.Hour), t.Add(time.Hour), t)

	wg, err := scanWeatherGrid(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, environment.ErrNoWeather
		}
		return nil, fmt.Errorf("weather.AtPoint: %w", err)
	}
	return wg, nil
}

// AlongRoute returns one WeatherGrid per entry in segments, matched by the cell
// containing the segment midpoint and the nearest hourly valid_at. Missing cells
// return a zero WeatherGrid (caller should treat as "no data").
func (r *WeatherRepo) AlongRoute(segments []environment.WeatherSegmentQuery) ([]environment.WeatherGrid, error) {
	results := make([]environment.WeatherGrid, len(segments))
	for i, seg := range segments {
		wg, err := r.AtPoint(seg.MidLat, seg.MidLon, seg.ArrivalAt)
		if err != nil {
			if errors.Is(err, environment.ErrNoWeather) {
				// Leave zero value; caller interprets as missing data
				continue
			}
			return nil, fmt.Errorf("weather.AlongRoute[%d]: %w", i, err)
		}
		results[i] = *wg
	}
	return results, nil
}

// DeleteBefore removes rows with valid_at < cutoff. Used by the pipeline to prune
// stale forecasts (retain ~48 hours rolling window).
func (r *WeatherRepo) DeleteBefore(cutoff time.Time) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM environment.weather_grid WHERE valid_at < $1`, cutoff)
	if err != nil {
		return fmt.Errorf("weather.DeleteBefore: %w", err)
	}
	return nil
}

// UpsertMinutely inserts minutely precipitation rows with ON CONFLICT upsert.
func (r *WeatherRepo) UpsertMinutely(rows []environment.MinutelyPrecip) error {
	if len(rows) == 0 {
		return nil
	}
	ctx := context.Background()
	return r.weatherWithTx(ctx, func(tx pgx.Tx) error {
		for _, m := range rows {
			_, err := tx.Exec(ctx, `
				INSERT INTO environment.minutely_precip (lat, lon, at, intensity_mmh, fetched_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (lat, lon, at)
				DO UPDATE SET intensity_mmh = EXCLUDED.intensity_mmh,
				              fetched_at    = EXCLUDED.fetched_at
			`, m.Lat, m.Lon, m.At, m.IntensityMMH, m.FetchedAt)
			if err != nil {
				return fmt.Errorf("minutely upsert: %w", err)
			}
		}
		return nil
	})
}

// MinutelyAt returns cached minutely precipitation near (lat, lon) in [from, to].
// Uses a small tolerance (~2.5km) to match the grid cell.
func (r *WeatherRepo) MinutelyAt(lat, lon float64, from, to time.Time) ([]environment.MinutelyPrecip, error) {
	ctx := context.Background()
	const tolerance = 0.025 // ~2.5km at Tokyo latitude
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, lat, lon, at, intensity_mmh, fetched_at
		FROM environment.minutely_precip
		WHERE lat BETWEEN $1 AND $2
		  AND lon BETWEEN $3 AND $4
		  AND at BETWEEN $5 AND $6
		ORDER BY at ASC
	`, lat-tolerance, lat+tolerance, lon-tolerance, lon+tolerance, from, to)
	if err != nil {
		return nil, fmt.Errorf("minutely.AtPoint: %w", err)
	}
	defer rows.Close()

	var result []environment.MinutelyPrecip
	for rows.Next() {
		var m environment.MinutelyPrecip
		if err := rows.Scan(&m.ID, &m.Lat, &m.Lon, &m.At, &m.IntensityMMH, &m.FetchedAt); err != nil {
			return nil, fmt.Errorf("minutely scan: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// DeleteMinutelyBefore prunes stale minutely rows.
func (r *WeatherRepo) DeleteMinutelyBefore(cutoff time.Time) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx,
		`DELETE FROM environment.minutely_precip WHERE at < $1`, cutoff)
	if err != nil {
		return fmt.Errorf("minutely.DeleteBefore: %w", err)
	}
	return nil
}

// --- helpers ---

func (r *WeatherRepo) weatherWithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// polygonWKT converts a [][2]float64 ring to a WKT POLYGON string.
// Expected format: 5-point closed ring (lon, lat ordering for WKT).
func polygonWKT(ring [][2]float64) string {
	if len(ring) == 0 {
		return "POLYGON EMPTY"
	}
	pts := make([]string, len(ring))
	for i, p := range ring {
		pts[i] = fmt.Sprintf("%f %f", p[0], p[1])
	}
	return fmt.Sprintf("POLYGON((%s))", joinStrings(pts, ", "))
}

func joinStrings(ss []string, sep string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}

func scanWeatherGrid(row pgx.Row) (*environment.WeatherGrid, error) {
	var wg environment.WeatherGrid
	var polyWKT string
	if err := row.Scan(
		&wg.ID,
		&polyWKT,
		&wg.ValidAt,
		&wg.WindSpeedMS,
		&wg.WindBearingDeg,
		&wg.PrecipIntensityMMH,
		&wg.TemperatureC,
	); err != nil {
		return nil, err
	}
	// Parse WKT polygon back into [][2]float64 ring (omitted in this pass;
	// callers only need the numeric fields for scoring).
	wg.CellGeometry = [][2]float64{}
	return &wg, nil
}
