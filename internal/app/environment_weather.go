package app

import (
	"context"
	"time"
)

// envQuerierWithLiveWeather decorates an EnvironmentQuerier so weather reads go
// through the WeatherService — precomputed grid first, then the cached live
// provider fetch — instead of only the environment.weather_grid table.
type envQuerierWithLiveWeather struct {
	EnvironmentQuerier
	weather *WeatherService
}

// NewEnvironmentQuerierWithLiveWeather wraps base so WeatherForPoint resolves
// through weather (which falls back to a live provider fetch on a grid miss).
// All other queries pass through to base unchanged.
func NewEnvironmentQuerierWithLiveWeather(base EnvironmentQuerier, weather *WeatherService) EnvironmentQuerier {
	return &envQuerierWithLiveWeather{EnvironmentQuerier: base, weather: weather}
}

func (q *envQuerierWithLiveWeather) WeatherForPoint(ctx context.Context, lon, lat float64, at time.Time) (windSpeedMS, windBearingDeg, precipMMH float64) {
	wg, err := q.weather.AtPointContext(ctx, lat, lon, at)
	if err != nil {
		return 0, 0, 0 // missing data stays zero, matching the querier contract
	}
	return wg.WindSpeedMS, wg.WindBearingDeg, wg.PrecipIntensityMMH
}
