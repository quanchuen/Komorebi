# Weather Integration — Implementation Plan

**Date:** 2026-04-10
**Author:** plan
**Status:** ready to execute

---

## Context

The design spec defines a `WeatherGrid` sub-domain under Environment. Open-Meteo provides
free hourly forecasts (no API key). Weather conditions feed the per-segment wind benefit
score that appears in route condition payloads and the Valhalla environment overlay.

The domain struct already exists at `internal/domain/environment/weather.go`.
The `environment.weather_grid` table exists (migration 000009).
Nothing in the infra or app layers wires weather yet.

This plan adds four things in order:

1. Open-Meteo HTTP client (`internal/infra/openmeteo/`)
2. `weather_fetch` pipeline command (`pipelines/weather_fetch/`)
3. Weather query repository (`internal/infra/postgres/weather_repo.go`)
4. Application service + wiring into `cmd/api/main.go`

---

## Steps

### 1. Domain — add repository interface and `WindBenefit` helper to `weather.go`

- [ ] Add `WeatherRepository` interface to `internal/domain/environment/weather.go`
- [ ] Add `WindBenefit(windBearingDeg, routeBearingDeg float64) float64` pure function

`WindBenefit` returns the dot product of the wind vector against the route travel direction,
normalised to [−1, +1]. Positive means tailwind, negative means headwind.

```go
// WindBenefit returns a score in [−1, +1].
//   +1 = pure tailwind (wind blows in the direction of travel)
//   −1 = pure headwind (wind blows against the direction of travel)
//    0 = crosswind
//
// routeBearingDeg is the compass bearing the rider is travelling (0 = north, 90 = east).
// windBearingDeg  is the direction the wind is coming FROM (meteorological convention).
func WindBenefit(windBearingDeg, routeBearingDeg float64) float64 {
    // Convert the wind source bearing to the direction the wind is travelling toward.
    windTravelDeg := math.Mod(windBearingDeg+180, 360)
    diff := (windTravelDeg - routeBearingDeg) * math.Pi / 180
    return math.Cos(diff) // −1 to +1
}
```

Interface to add:

```go
// WeatherRepository is the persistence contract for weather grid reads and writes.
type WeatherRepository interface {
    // Upsert inserts or replaces weather grid rows.
    // The composite key is (cell_geometry centroid, valid_at).
    // In practice this is handled by DELETE + INSERT in the pipeline.
    Upsert(cells []WeatherGrid) error

    // AtPoint returns the single weather grid cell whose geometry contains
    // (lat, lon) and whose valid_at is nearest to t (within ±1 hour).
    // Returns ErrNoWeather if no row is found.
    AtPoint(lat, lon float64, t time.Time) (*WeatherGrid, error)

    // AlongRoute returns one WeatherGrid per route segment, using the segment
    // midpoint and projected arrival time. segments is a slice of
    // (midLat, midLon, arrivalAt) tuples.
    AlongRoute(segments []WeatherSegmentQuery) ([]WeatherGrid, error)

    // DeleteBefore removes rows with valid_at older than cutoff to keep the table
    // from growing unboundedly.
    DeleteBefore(cutoff time.Time) error
}

// WeatherSegmentQuery is an input tuple for AlongRoute.
type WeatherSegmentQuery struct {
    MidLat    float64
    MidLon    float64
    ArrivalAt time.Time
}

// ErrNoWeather is returned when no weather data covers the requested point/time.
var ErrNoWeather = errors.New("weather: no data for point/time")
```

**Test:** `internal/domain/environment/weather_test.go`

```go
func TestWindBenefit(t *testing.T) {
    cases := []struct {
        windFrom   float64
        routeBear  float64
        wantSign   float64 // +1 tailwind, -1 headwind, 0 cross
        wantApprox float64
    }{
        // Wind from south (180°), riding north (0°) → tailwind
        {180, 0, 1, 1.0},
        // Wind from north (0°), riding north (0°) → headwind
        {0, 0, -1, -1.0},
        // Wind from east (90°), riding north (0°) → crosswind
        {90, 0, 0, 0.0},
        // Wind from SW (225°), riding NE (45°) → tailwind
        {225, 45, 1, 1.0},
    }
    for _, tc := range cases {
        got := environment.WindBenefit(tc.windFrom, tc.routeBear)
        if math.Abs(got-tc.wantApprox) > 0.001 {
            t.Errorf("WindBenefit(%v, %v) = %v, want %v",
                tc.windFrom, tc.routeBear, got, tc.wantApprox)
        }
    }
}
```

---

### 2. Open-Meteo client (`internal/infra/openmeteo/`)

Files: `client.go`, `client_test.go`

#### `client.go`

```go
// Package openmeteo fetches hourly weather forecasts from api.open-meteo.com.
// No API key is required. Data is fetched per grid point; callers supply a
// list of (lat, lon) points and receive WeatherGrid slices back.
package openmeteo

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strconv"
    "time"

    "komorebi/internal/domain/environment"
)

const (
    defaultBaseURL = "https://api.open-meteo.com/v1/forecast"
    // cellHalfSideM is half the cell edge in degrees at Tokyo latitude (~5 km cell).
    // 0.025° ≈ 2.5 km at 35°N, giving a 5 km × 5 km cell.
    cellHalfSide = 0.025
)

// Client fetches weather from Open-Meteo.
type Client struct {
    baseURL    string
    httpClient *http.Client
}

// NewClient creates a Client. Pass "" for baseURL to use the production API.
func NewClient(baseURL string) *Client {
    if baseURL == "" {
        baseURL = defaultBaseURL
    }
    return &Client{
        baseURL:    baseURL,
        httpClient: &http.Client{Timeout: 15 * time.Second},
    }
}

// FetchPoint fetches hourly forecast for a single (lat, lon) and returns one
// WeatherGrid row per forecast hour. cell_geometry is a 5 km square centred on
// the point. The Valid_at times are in UTC.
func (c *Client) FetchPoint(ctx context.Context, lat, lon float64) ([]environment.WeatherGrid, error) {
    u, err := url.Parse(c.baseURL)
    if err != nil {
        return nil, err
    }
    q := u.Query()
    q.Set("latitude", strconv.FormatFloat(lat, 'f', 6, 64))
    q.Set("longitude", strconv.FormatFloat(lon, 'f', 6, 64))
    q.Set("hourly", "wind_speed_10m,wind_direction_10m,precipitation,temperature_2m")
    q.Set("wind_speed_unit", "ms")
    q.Set("timezone", "UTC")
    u.RawQuery = q.Encode()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("openmeteo: fetch %v,%v: %w", lat, lon, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("openmeteo: status %d for %v,%v", resp.StatusCode, lat, lon)
    }

    var raw forecastResponse
    if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
        return nil, fmt.Errorf("openmeteo: decode: %w", err)
    }

    return raw.toWeatherGrids(lat, lon)
}

// FetchGrid fetches forecasts for every point in a regular grid defined by the
// bounding box [minLat, maxLat, minLon, maxLon] at stepDeg spacing.
// Calls FetchPoint for each grid point sequentially with a short sleep to be
// polite to the free API tier.
func (c *Client) FetchGrid(ctx context.Context, minLat, maxLat, minLon, maxLon, stepDeg float64) ([]environment.WeatherGrid, error) {
    var all []environment.WeatherGrid
    for lat := minLat; lat <= maxLat+1e-9; lat += stepDeg {
        for lon := minLon; lon <= maxLon+1e-9; lon += stepDeg {
            cells, err := c.FetchPoint(ctx, round6(lat), round6(lon))
            if err != nil {
                return nil, fmt.Errorf("openmeteo: grid point %v,%v: %w", lat, lon, err)
            }
            all = append(all, cells...)
            // Polite rate-limiting: 50 ms between requests.
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(50 * time.Millisecond):
            }
        }
    }
    return all, nil
}

// --- internal types ---

type forecastResponse struct {
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Hourly    struct {
        Time            []string  `json:"time"`
        WindSpeed10M    []float64 `json:"wind_speed_10m"`
        WindDir10M      []float64 `json:"wind_direction_10m"`
        Precipitation   []float64 `json:"precipitation"`
        Temperature2M   []float64 `json:"temperature_2m"`
    } `json:"hourly"`
}

func (r *forecastResponse) toWeatherGrids(lat, lon float64) ([]environment.WeatherGrid, error) {
    n := len(r.Hourly.Time)
    if n == 0 {
        return nil, fmt.Errorf("openmeteo: empty hourly data for %v,%v", lat, lon)
    }
    cell := cellPolygon(lat, lon)
    grids := make([]environment.WeatherGrid, 0, n)
    for i := 0; i < n; i++ {
        t, err := time.Parse("2006-01-02T15:04", r.Hourly.Time[i])
        if err != nil {
            return nil, fmt.Errorf("openmeteo: parse time %q: %w", r.Hourly.Time[i], err)
        }
        grids = append(grids, environment.WeatherGrid{
            CellGeometry:       cell,
            ValidAt:            t.UTC(),
            WindSpeedMS:        safeIndex(r.Hourly.WindSpeed10M, i),
            WindBearingDeg:     safeIndex(r.Hourly.WindDir10M, i),
            PrecipIntensityMMH: safeIndex(r.Hourly.Precipitation, i),
            TemperatureC:       safeIndex(r.Hourly.Temperature2M, i),
        })
    }
    return grids, nil
}

// cellPolygon builds a 5 km square polygon centred on (lat, lon).
// Returned as [][2]float64 rings (closed, 5 points).
func cellPolygon(lat, lon float64) [][2]float64 {
    minLon := lon - cellHalfSide
    maxLon := lon + cellHalfSide
    minLat := lat - cellHalfSide
    maxLat := lat + cellHalfSide
    return [][2]float64{
        {minLon, minLat},
        {maxLon, minLat},
        {maxLon, maxLat},
        {minLon, maxLat},
        {minLon, minLat}, // closed ring
    }
}

func safeIndex(s []float64, i int) float64 {
    if i < len(s) {
        return s[i]
    }
    return 0
}

func round6(f float64) float64 {
    return float64(int(f*1e6+0.5)) / 1e6
}
```

#### `client_test.go`

Use `httptest.NewServer` to serve a canned Open-Meteo JSON response.

```go
package openmeteo_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "komorebi/internal/infra/openmeteo"
)

func TestFetchPoint_ParsesResponse(t *testing.T) {
    payload := map[string]any{
        "latitude":  35.685,
        "longitude": 139.7,
        "hourly": map[string]any{
            "time":              []string{"2026-04-10T00:00", "2026-04-10T01:00"},
            "wind_speed_10m":    []float64{3.5, 4.0},
            "wind_direction_10m": []float64{180.0, 185.0},
            "precipitation":     []float64{0.0, 0.2},
            "temperature_2m":    []float64{15.0, 14.5},
        },
    }
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(payload)
    }))
    defer srv.Close()

    client := openmeteo.NewClient(srv.URL)
    grids, err := client.FetchPoint(context.Background(), 35.685, 139.7)
    if err != nil {
        t.Fatalf("FetchPoint: %v", err)
    }
    if len(grids) != 2 {
        t.Fatalf("want 2 grids, got %d", len(grids))
    }

    g0 := grids[0]
    if g0.WindSpeedMS != 3.5 {
        t.Errorf("WindSpeedMS: want 3.5, got %v", g0.WindSpeedMS)
    }
    if g0.WindBearingDeg != 180.0 {
        t.Errorf("WindBearingDeg: want 180, got %v", g0.WindBearingDeg)
    }
    if g0.PrecipIntensityMMH != 0.0 {
        t.Errorf("Precip: want 0, got %v", g0.PrecipIntensityMMH)
    }
    want0, _ := time.Parse("2006-01-02T15:04", "2026-04-10T00:00")
    if !g0.ValidAt.Equal(want0.UTC()) {
        t.Errorf("ValidAt: want %v, got %v", want0.UTC(), g0.ValidAt)
    }
    // Cell polygon must be a closed ring of 5 points.
    if len(g0.CellGeometry) != 5 {
        t.Errorf("CellGeometry: want 5 points, got %d", len(g0.CellGeometry))
    }
    if g0.CellGeometry[0] != g0.CellGeometry[4] {
        t.Error("CellGeometry ring is not closed")
    }
}

func TestFetchPoint_HTTPError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, "rate limited", http.StatusTooManyRequests)
    }))
    defer srv.Close()

    client := openmeteo.NewClient(srv.URL)
    _, err := client.FetchPoint(context.Background(), 35.685, 139.7)
    if err == nil {
        t.Fatal("expected error for non-200 status")
    }
}
```

---

### 3. Migration — add composite uniqueness index and partial index for current data

- [ ] Create `migrations/000016_weather_grid_indexes.up.sql`

The existing table and basic indexes are already in migration 000009. This migration adds a
unique constraint to enable upsert-by-position, and a partial index to accelerate the
"current forecast window" query.

```sql
-- migrations/000016_weather_grid_indexes.up.sql

-- Unique constraint on (centroid of cell, valid_at) for upsert semantics.
-- We represent centroid as the rounded lat/lon used as the grid key.
-- Because PostGIS doesn't support functional unique indexes on geometry expressions
-- without an immutable function, we store grid_lat and grid_lon as generated columns.
ALTER TABLE environment.weather_grid
    ADD COLUMN IF NOT EXISTS grid_lat DOUBLE PRECISION
        GENERATED ALWAYS AS (ST_Y(ST_Centroid(cell_geometry))) STORED,
    ADD COLUMN IF NOT EXISTS grid_lon DOUBLE PRECISION
        GENERATED ALWAYS AS (ST_X(ST_Centroid(cell_geometry))) STORED;

CREATE UNIQUE INDEX IF NOT EXISTS uidx_weather_grid_point_time
    ON environment.weather_grid (grid_lat, grid_lon, valid_at);

-- Partial index for rows within ±2 hours of now (query-time optimisation).
-- Rebuilt each run; useful for the point-lookup query in the weather repo.
CREATE INDEX IF NOT EXISTS idx_weather_grid_recent
    ON environment.weather_grid (valid_at)
    WHERE valid_at >= NOW() - INTERVAL '2 hours';
```

```sql
-- migrations/000016_weather_grid_indexes.down.sql
DROP INDEX IF EXISTS environment.idx_weather_grid_recent;
DROP INDEX IF EXISTS environment.uidx_weather_grid_point_time;
ALTER TABLE environment.weather_grid
    DROP COLUMN IF EXISTS grid_lat,
    DROP COLUMN IF EXISTS grid_lon;
```

---

### 4. Weather repository (`internal/infra/postgres/weather_repo.go`)

- [ ] Write `WeatherRepo` implementing `environment.WeatherRepository`
- [ ] Write integration tests in `weather_repo_test.go`

```go
// internal/infra/postgres/weather_repo.go
package postgres

import (
    "context"
    "errors"
    "fmt"
    "time"

    "komorebi/internal/domain/environment"
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
    return r.withTx(ctx, func(tx pgx.Tx) error {
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
// closest valid_at to t (within ±1 hour). Returns environment.ErrNoWeather when
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

// --- helpers ---

func (r *WeatherRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
```

#### `weather_repo_test.go`

```go
// internal/infra/postgres/weather_repo_test.go
package postgres_test

import (
    "errors"
    "testing"
    "time"

    "komorebi/internal/domain/environment"
    "komorebi/internal/infra/postgres"
)

// seedWeatherCell inserts a single 5 km cell centred on (lat, lon) for the given time.
func seedWeatherCell(t *testing.T, repo *postgres.WeatherRepo, lat, lon float64, validAt time.Time) {
    t.Helper()
    half := 0.025
    cell := [][2]float64{
        {lon - half, lat - half},
        {lon + half, lat - half},
        {lon + half, lat + half},
        {lon - half, lat + half},
        {lon - half, lat - half},
    }
    err := repo.Upsert([]environment.WeatherGrid{{
        CellGeometry:       cell,
        ValidAt:            validAt,
        WindSpeedMS:        5.0,
        WindBearingDeg:     180.0,
        PrecipIntensityMMH: 0.0,
        TemperatureC:       18.0,
    }})
    if err != nil {
        t.Fatalf("seedWeatherCell: %v", err)
    }
}

func TestWeatherRepo_UpsertAndAtPoint(t *testing.T) {
    pool := newTestPool(t)
    repo := postgres.NewWeatherRepo(pool)

    // Use a fixed time well in the past to avoid index conflicts with real data.
    validAt := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
    // Tokyo station ~35.6812°N, 139.7671°E
    lat, lon := 35.6812, 139.7671
    seedWeatherCell(t, repo, lat, lon, validAt)

    got, err := repo.AtPoint(lat, lon, validAt)
    if err != nil {
        t.Fatalf("AtPoint: %v", err)
    }
    if got.WindSpeedMS != 5.0 {
        t.Errorf("WindSpeedMS: want 5.0, got %v", got.WindSpeedMS)
    }
    if got.WindBearingDeg != 180.0 {
        t.Errorf("WindBearingDeg: want 180, got %v", got.WindBearingDeg)
    }

    // Cleanup
    _ = repo.DeleteBefore(validAt.Add(time.Second))
}

func TestWeatherRepo_AtPoint_NoData(t *testing.T) {
    pool := newTestPool(t)
    repo := postgres.NewWeatherRepo(pool)

    // Point in the middle of the ocean, definitely no data
    _, err := repo.AtPoint(0.0, 0.0, time.Now())
    if !errors.Is(err, environment.ErrNoWeather) {
        t.Fatalf("want ErrNoWeather, got %v", err)
    }
}

func TestWeatherRepo_Upsert_Idempotent(t *testing.T) {
    pool := newTestPool(t)
    repo := postgres.NewWeatherRepo(pool)

    validAt := time.Date(2020, 2, 1, 6, 0, 0, 0, time.UTC)
    lat, lon := 35.7000, 139.8000
    seedWeatherCell(t, repo, lat, lon, validAt)

    // Second upsert with updated wind speed — should not error or duplicate.
    half := 0.025
    cell := [][2]float64{
        {lon - half, lat - half}, {lon + half, lat - half},
        {lon + half, lat + half}, {lon - half, lat + half},
        {lon - half, lat - half},
    }
    err := repo.Upsert([]environment.WeatherGrid{{
        CellGeometry:   cell,
        ValidAt:        validAt,
        WindSpeedMS:    9.0,
        WindBearingDeg: 90.0,
    }})
    if err != nil {
        t.Fatalf("second upsert: %v", err)
    }

    got, err := repo.AtPoint(lat, lon, validAt)
    if err != nil {
        t.Fatalf("AtPoint after re-upsert: %v", err)
    }
    if got.WindSpeedMS != 9.0 {
        t.Errorf("WindSpeedMS after update: want 9.0, got %v", got.WindSpeedMS)
    }

    _ = repo.DeleteBefore(validAt.Add(time.Second))
}

func TestWeatherRepo_DeleteBefore(t *testing.T) {
    pool := newTestPool(t)
    repo := postgres.NewWeatherRepo(pool)

    validAt := time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
    lat, lon := 35.6500, 139.7500
    seedWeatherCell(t, repo, lat, lon, validAt)

    if err := repo.DeleteBefore(validAt.Add(time.Second)); err != nil {
        t.Fatalf("DeleteBefore: %v", err)
    }

    _, err := repo.AtPoint(lat, lon, validAt)
    if !errors.Is(err, environment.ErrNoWeather) {
        t.Fatalf("want ErrNoWeather after delete, got %v", err)
    }
}
```

---

### 5. Weather application service (`internal/app/weather_service.go`)

- [ ] Add `WeatherService` struct with `AtPoint` and `AlongRoute` methods
- [ ] Expose `WindBenefitForSegment` that calls the domain function

```go
// internal/app/weather_service.go
package app

import (
    "math"
    "time"

    "komorebi/internal/domain/environment"
)

// WeatherService is the application-layer facade over weather data.
type WeatherService struct {
    repo environment.WeatherRepository
}

// NewWeatherService creates a WeatherService backed by the given repository.
func NewWeatherService(repo environment.WeatherRepository) *WeatherService {
    return &WeatherService{repo: repo}
}

// AtPoint returns weather conditions at a geographic point and time.
// Returns ErrNoWeather if no data covers that point/time.
func (s *WeatherService) AtPoint(lat, lon float64, t time.Time) (*environment.WeatherGrid, error) {
    return s.repo.AtPoint(lat, lon, t)
}

// AlongRoute scores each segment by fetching the nearest weather cell and
// computing the wind benefit relative to each segment's bearing.
// segmentBearings[i] is the compass bearing (degrees) of segment i.
func (s *WeatherService) AlongRoute(
    segments []environment.WeatherSegmentQuery,
    segmentBearings []float64,
) ([]environment.SegmentWeather, error) {
    grids, err := s.repo.AlongRoute(segments)
    if err != nil {
        return nil, err
    }

    results := make([]environment.SegmentWeather, len(grids))
    for i, g := range grids {
        bearing := 0.0
        if i < len(segmentBearings) {
            bearing = segmentBearings[i]
        }
        benefit := 0.0
        if g.WindSpeedMS > 0 {
            benefit = environment.WindBenefit(g.WindBearingDeg, bearing)
        }
        results[i] = environment.SegmentWeather{
            WindBenefit:        benefit,
            PrecipIntensityMMH: g.PrecipIntensityMMH,
            TemperatureC:       g.TemperatureC,
            WindSpeedMS:        g.WindSpeedMS,
        }
    }
    return results, nil
}

// BearingDeg computes the compass bearing from point A to point B in degrees
// (0 = north, 90 = east). Used by the routing pipeline to compute segment bearings.
func BearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
    φ1 := lat1 * math.Pi / 180
    φ2 := lat2 * math.Pi / 180
    Δλ := (lon2 - lon1) * math.Pi / 180
    y := math.Sin(Δλ) * math.Cos(φ2)
    x := math.Cos(φ1)*math.Sin(φ2) - math.Sin(φ1)*math.Cos(φ2)*math.Cos(Δλ)
    return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}
```

Add `SegmentWeather` to the domain package (`internal/domain/environment/weather.go`):

```go
// SegmentWeather is the weather summary for one route segment, pre-computed
// with a wind benefit score relative to the segment's bearing.
type SegmentWeather struct {
    WindBenefit        float64 // −1 (headwind) to +1 (tailwind)
    PrecipIntensityMMH float64
    TemperatureC       float64
    WindSpeedMS        float64
}
```

---

### 6. Weather fetch pipeline (`pipelines/weather_fetch/main.go`)

- [ ] Write the pipeline command
- [ ] Document cron invocation

```go
// pipelines/weather_fetch/main.go
//
// Weather Fetch Pipeline
//
// Fetches hourly Open-Meteo forecasts for the Greater Tokyo grid and stores
// them in environment.weather_grid. Designed to run hourly via cron:
//
//   0 * * * * DATABASE_URL=... /path/to/weather_fetch
//
// Grid coverage: 35.5–35.85°N, 139.4–140.0°E at 0.05° spacing (~5 km cells).
// Each run fetches ~48 forecast hours × ~91 grid points = ~4 400 rows.
// Rows older than 48 hours are pruned after each successful fetch.
package main

import (
    "context"
    "log"
    "os"
    "time"

    "komorebi/internal/infra/openmeteo"
    "komorebi/internal/infra/postgres"
    "github.com/jackc/pgx/v5/pgxpool"
)

const (
    gridMinLat = 35.50
    gridMaxLat = 35.85
    gridMinLon = 139.40
    gridMaxLon = 140.00
    gridStepDeg = 0.05 // ~5 km at Tokyo latitude
    retainHours = 48
)

func main() {
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        log.Fatal("DATABASE_URL is required")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    pool, err := pgxpool.New(ctx, dbURL)
    if err != nil {
        log.Fatalf("db connect: %v", err)
    }
    defer pool.Close()

    if err := pool.Ping(ctx); err != nil {
        log.Fatalf("db ping: %v", err)
    }

    weatherRepo := postgres.NewWeatherRepo(pool)

    omBaseURL := os.Getenv("OPENMETEO_BASE_URL") // override in tests
    client := openmeteo.NewClient(omBaseURL)

    log.Println("fetching Open-Meteo grid...")
    cells, err := client.FetchGrid(ctx, gridMinLat, gridMaxLat, gridMinLon, gridMaxLon, gridStepDeg)
    if err != nil {
        log.Fatalf("FetchGrid: %v", err)
    }
    log.Printf("fetched %d forecast rows", len(cells))

    log.Println("upserting into weather_grid...")
    if err := weatherRepo.Upsert(cells); err != nil {
        log.Fatalf("Upsert: %v", err)
    }

    cutoff := time.Now().UTC().Add(-retainHours * time.Hour)
    log.Printf("pruning rows older than %v...", cutoff.Format(time.RFC3339))
    if err := weatherRepo.DeleteBefore(cutoff); err != nil {
        log.Printf("WARN: DeleteBefore: %v (non-fatal)", err)
    }

    log.Println("weather_fetch: done")
}
```

---

### 7. Wire into `cmd/api/main.go`

- [ ] Add `WeatherRepo` and `WeatherService` construction
- [ ] Pass `WeatherService` to a `WeatherHandler` (new handler in `internal/api/`)
- [ ] Register `/api/v1/weather/point` endpoint

The existing wiring pattern in `cmd/api/main.go`:

```go
// After: venueRepo / venueSvc wiring
weatherRepo := postgres.NewWeatherRepo(pool)
weatherSvc  := app.NewWeatherService(weatherRepo)
weatherHandler := api.NewWeatherHandler(weatherSvc)
```

Pass `weatherHandler` to `api.NewRouter` (update that function's signature).

---

### 8. Weather HTTP handler (`internal/api/weather_handler.go`)

```go
// internal/api/weather_handler.go
package api

import (
    "encoding/json"
    "errors"
    "net/http"
    "strconv"
    "time"

    "komorebi/internal/app"
    "komorebi/internal/domain/environment"
)

// WeatherHandler serves weather condition endpoints.
type WeatherHandler struct {
    svc *app.WeatherService
}

// NewWeatherHandler creates a WeatherHandler.
func NewWeatherHandler(svc *app.WeatherService) *WeatherHandler {
    return &WeatherHandler{svc: svc}
}

// GET /api/v1/weather/point?lat=&lon=&at=
// at is optional ISO-8601; defaults to current time.
func (h *WeatherHandler) AtPoint(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    lat, err := strconv.ParseFloat(q.Get("lat"), 64)
    if err != nil {
        http.Error(w, "invalid lat", http.StatusBadRequest)
        return
    }
    lon, err := strconv.ParseFloat(q.Get("lon"), 64)
    if err != nil {
        http.Error(w, "invalid lon", http.StatusBadRequest)
        return
    }
    t := time.Now().UTC()
    if atStr := q.Get("at"); atStr != "" {
        t, err = time.Parse(time.RFC3339, atStr)
        if err != nil {
            http.Error(w, "invalid at (use RFC3339)", http.StatusBadRequest)
            return
        }
        t = t.UTC()
    }

    wg, err := h.svc.AtPoint(lat, lon, t)
    if err != nil {
        if errors.Is(err, environment.ErrNoWeather) {
            http.Error(w, "no weather data for point/time", http.StatusNotFound)
            return
        }
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    type response struct {
        ValidAt            time.Time `json:"valid_at"`
        WindSpeedMS        float64   `json:"wind_speed_ms"`
        WindBearingDeg     float64   `json:"wind_bearing_deg"`
        PrecipIntensityMMH float64   `json:"precip_intensity_mmh"`
        TemperatureC       float64   `json:"temperature_c"`
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(response{
        ValidAt:            wg.ValidAt,
        WindSpeedMS:        wg.WindSpeedMS,
        WindBearingDeg:     wg.WindBearingDeg,
        PrecipIntensityMMH: wg.PrecipIntensityMMH,
        TemperatureC:       wg.TemperatureC,
    })
}
```

Register in `internal/api/router.go`:

```go
r.Get("/api/v1/weather/point", weatherHandler.AtPoint)
```

---

### 9. Handler unit test (`internal/api/weather_handler_test.go`)

```go
package api_test

import (
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "komorebi/internal/api"
    "komorebi/internal/app"
    "komorebi/internal/domain/environment"
)

// fakeWeatherRepo implements environment.WeatherRepository for handler tests.
type fakeWeatherRepo struct {
    cell *environment.WeatherGrid
    err  error
}

func (f *fakeWeatherRepo) Upsert(_ []environment.WeatherGrid) error { return nil }
func (f *fakeWeatherRepo) AtPoint(_, _ float64, _ time.Time) (*environment.WeatherGrid, error) {
    return f.cell, f.err
}
func (f *fakeWeatherRepo) AlongRoute(_ []environment.WeatherSegmentQuery) ([]environment.WeatherGrid, error) {
    return nil, nil
}
func (f *fakeWeatherRepo) DeleteBefore(_ time.Time) error { return nil }

func TestWeatherHandler_AtPoint_OK(t *testing.T) {
    stub := &environment.WeatherGrid{
        ValidAt:            time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
        WindSpeedMS:        4.2,
        WindBearingDeg:     270.0,
        PrecipIntensityMMH: 0.0,
        TemperatureC:       20.0,
    }
    repo := &fakeWeatherRepo{cell: stub}
    svc := app.NewWeatherService(repo)
    h := api.NewWeatherHandler(svc)

    req := httptest.NewRequest(http.MethodGet,
        "/api/v1/weather/point?lat=35.68&lon=139.77", nil)
    rr := httptest.NewRecorder()
    h.AtPoint(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("status: want 200, got %d: %s", rr.Code, rr.Body.String())
    }
    var body map[string]any
    if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if body["wind_speed_ms"] != 4.2 {
        t.Errorf("wind_speed_ms: want 4.2, got %v", body["wind_speed_ms"])
    }
}

func TestWeatherHandler_AtPoint_NotFound(t *testing.T) {
    repo := &fakeWeatherRepo{err: environment.ErrNoWeather}
    svc := app.NewWeatherService(repo)
    h := api.NewWeatherHandler(svc)

    req := httptest.NewRequest(http.MethodGet,
        "/api/v1/weather/point?lat=0&lon=0", nil)
    rr := httptest.NewRecorder()
    h.AtPoint(rr, req)

    if rr.Code != http.StatusNotFound {
        t.Fatalf("status: want 404, got %d", rr.Code)
    }
}

func TestWeatherHandler_AtPoint_MissingLat(t *testing.T) {
    repo := &fakeWeatherRepo{}
    svc := app.NewWeatherService(repo)
    h := api.NewWeatherHandler(svc)

    req := httptest.NewRequest(http.MethodGet,
        "/api/v1/weather/point?lon=139.77", nil)
    rr := httptest.NewRecorder()
    h.AtPoint(rr, req)

    if rr.Code != http.StatusBadRequest {
        t.Fatalf("status: want 400, got %d", rr.Code)
    }
}
```

---

## Execution order

```
Step 1  → weather.go domain changes + weather_test.go          (pure, no DB needed)
Step 2  → openmeteo/client.go + client_test.go                 (httptest, no DB needed)
Step 3  → run migration 000016 against cyclist_map_dev
Step 4  → weather_repo.go + weather_repo_test.go               (integration, DB needed)
Step 5  → weather_service.go + SegmentWeather domain type      (unit)
Step 6  → pipelines/weather_fetch/main.go
Step 7  → cmd/api/main.go wiring
Step 8  → weather_handler.go
Step 9  → weather_handler_test.go                              (unit, no DB)
```

Run the full test suite after each step:

```
go test ./internal/domain/environment/...
go test ./internal/infra/openmeteo/...
TEST_DB_DSN="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
    go test ./internal/infra/postgres/...
go test ./internal/app/...
go test ./internal/api/...
```

Build the pipeline binary:

```
go build -o bin/weather_fetch ./pipelines/weather_fetch/
```

---

## Self-review checklist

- [ ] `WindBenefit` uses meteorological convention: wind bearing is FROM, so +180° gives the travel direction
- [ ] `cellHalfSide = 0.025°` gives ~2.5 km half-edge → 5 km cell, matching design spec resolution
- [ ] `polygonWKT` outputs lon-lat order (PostGIS SRID 4326 convention)
- [ ] `Upsert` uses `ON CONFLICT (grid_lat, grid_lon, valid_at)` — requires migration 000016 applied first
- [ ] `AtPoint` uses `ST_Contains` (point inside polygon) not `ST_DWithin`; cells are non-overlapping by construction
- [ ] `AlongRoute` iterates sequentially; acceptable at ≤30 segments per typical route. Can be batched with a single unnest() query if profiling reveals this as a bottleneck
- [ ] Pipeline prunes rows older than 48 hours after each successful write; table stays bounded
- [ ] `FetchGrid` sleeps 50 ms between requests — Open-Meteo free tier limit is ~10 000 calls/day, Tokyo grid is ~91 points, well within limit
- [ ] Handler uses `errors.Is(err, environment.ErrNoWeather)` not type assertion
- [ ] No API key, no secrets — Open-Meteo is fully open
- [ ] `newTestPool` helper in `discovery_repo_test.go` is reused by all postgres integration tests via the shared `postgres_test` package; `weather_repo_test.go` follows the same convention
