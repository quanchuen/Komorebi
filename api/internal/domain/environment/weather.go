package environment

import (
	"context"
	"errors"
	"time"
)

// WeatherGrid stores weather conditions for a map cell at a given time.
type WeatherGrid struct {
	ID                 string
	CellGeometry       [][2]float64
	ValidAt            time.Time
	WindSpeedMS        float64
	WindBearingDeg     float64
	PrecipIntensityMMH float64
	TemperatureC       float64
	UVIndex            float64 // 0-11+, UV radiation index
}

// MinutelyPrecip is one minute (or sub-hour interval) of precipitation nowcast.
type MinutelyPrecip struct {
	ID           string
	Lat          float64
	Lon          float64
	At           time.Time
	IntensityMMH float64
	FetchedAt    time.Time // when this nowcast was fetched
}

// ErrNoWeather is returned when no weather data covers the requested point/time.
var ErrNoWeather = errors.New("weather: no data for point/time")

// WeatherFetcher abstracts an external weather API provider.
// Each provider (Open-Meteo, Tomorrow.io, OpenWeatherMap) implements this
// interface so the pipeline and services can swap providers via configuration.
type WeatherFetcher interface {
	// FetchPoint returns hourly forecast rows for a single (lat, lon).
	FetchPoint(ctx context.Context, lat, lon float64) ([]WeatherGrid, error)

	// FetchGrid fetches forecasts for a bounding box at the given step size.
	FetchGrid(ctx context.Context, minLat, maxLat, minLon, maxLon, stepDeg float64) ([]WeatherGrid, error)

	// FetchMinutely returns per-minute precipitation nowcast for the next ~60 min.
	// Returns nil, nil if the provider doesn't support minutely data.
	FetchMinutely(ctx context.Context, lat, lon float64) ([]MinutelyPrecip, error)

	// Name returns the provider name (e.g. "open-meteo", "tomorrow-io", "openweathermap").
	Name() string
}

// WeatherRepository is the persistence contract for weather grid reads and writes.
type WeatherRepository interface {
	// Upsert inserts or replaces weather grid rows.
	// The composite key is (cell_geometry centroid, valid_at).
	Upsert(cells []WeatherGrid) error

	// AtPoint returns the single weather grid cell whose geometry contains
	// (lat, lon) and whose valid_at is nearest to t (within +/-1 hour).
	// Returns ErrNoWeather if no row is found.
	AtPoint(lat, lon float64, t time.Time) (*WeatherGrid, error)

	// DeleteBefore removes rows with valid_at older than cutoff to keep the table
	// from growing unboundedly.
	DeleteBefore(cutoff time.Time) error

	// UpsertMinutely inserts or replaces minutely precipitation rows.
	UpsertMinutely(rows []MinutelyPrecip) error

	// GridInBBox returns the cells intersecting bbox (minLon, minLat, maxLon,
	// maxLat) at the single nearest distinct valid_at within +/-1 hour of at.
	// Returns an empty slice (nil error) when no snapshot or cell qualifies.
	GridInBBox(ctx context.Context, bbox [4]float64, at time.Time) ([]WeatherGrid, error)

	// MinutelySnapshot returns all nowcast points at the nearest distinct At
	// within +/-5 minutes of at, or an empty slice when none exists.
	MinutelySnapshot(ctx context.Context, at time.Time) ([]MinutelyPrecip, error)

	// DeleteMinutelyBefore prunes stale minutely rows.
	DeleteMinutelyBefore(cutoff time.Time) error
}
