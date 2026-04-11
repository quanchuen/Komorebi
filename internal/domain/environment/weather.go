package environment

import (
	"context"
	"errors"
	"math"
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

// SegmentWeather is the weather summary for one route segment, pre-computed
// with a wind benefit score relative to the segment's bearing.
type SegmentWeather struct {
	WindBenefit        float64 // -1 (headwind) to +1 (tailwind)
	PrecipIntensityMMH float64
	TemperatureC       float64
	WindSpeedMS        float64
	UVIndex            float64 // 0-11+
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

// WeatherSegmentQuery is an input tuple for AlongRoute.
type WeatherSegmentQuery struct {
	MidLat    float64
	MidLon    float64
	ArrivalAt time.Time
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

	// AlongRoute returns one WeatherGrid per route segment, using the segment
	// midpoint and projected arrival time.
	AlongRoute(segments []WeatherSegmentQuery) ([]WeatherGrid, error)

	// DeleteBefore removes rows with valid_at older than cutoff to keep the table
	// from growing unboundedly.
	DeleteBefore(cutoff time.Time) error

	// UpsertMinutely inserts or replaces minutely precipitation rows.
	UpsertMinutely(rows []MinutelyPrecip) error

	// MinutelyAt returns cached minutely precipitation near (lat, lon)
	// with At times within the range [from, to].
	MinutelyAt(lat, lon float64, from, to time.Time) ([]MinutelyPrecip, error)

	// DeleteMinutelyBefore prunes stale minutely rows.
	DeleteMinutelyBefore(cutoff time.Time) error
}

// WindBenefit returns a score in [-1, +1].
//
//	+1 = pure tailwind (wind blows in the direction of travel)
//	-1 = pure headwind (wind blows against the direction of travel)
//	 0 = crosswind
//
// routeBearingDeg is the compass bearing the rider is travelling (0 = north, 90 = east).
// windBearingDeg  is the direction the wind is coming FROM (meteorological convention).
func WindBenefit(windBearingDeg, routeBearingDeg float64) float64 {
	// Convert the wind source bearing to the direction the wind is travelling toward.
	windTravelDeg := math.Mod(windBearingDeg+180, 360)
	diff := (windTravelDeg - routeBearingDeg) * math.Pi / 180
	return math.Cos(diff) // -1 to +1
}
