// Package weatherprovider selects a WeatherFetcher implementation from
// environment configuration. Shared by the API server and the weather_fetch
// pipeline so provider selection stays in one place.
package weatherprovider

import (
	"fmt"
	"os"

	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/openmeteo"
	"komorebi/internal/infra/openweathermap"
	"komorebi/internal/infra/tomorrowio"
)

// FromEnv builds the fetcher named by WEATHER_PROVIDER (default "open-meteo").
// WEATHER_API_KEY is required for paid providers; WEATHER_BASE_URL overrides
// the provider endpoint for testing.
func FromEnv() (environment.WeatherFetcher, error) {
	return Named(os.Getenv("WEATHER_PROVIDER"))
}

// Named builds the fetcher for a specific provider name, reading credentials
// from the environment. An empty name selects open-meteo.
func Named(provider string) (environment.WeatherFetcher, error) {
	apiKey := os.Getenv("WEATHER_API_KEY")
	baseURL := os.Getenv("WEATHER_BASE_URL")

	switch provider {
	case "tomorrow-io":
		if apiKey == "" {
			return nil, fmt.Errorf("WEATHER_API_KEY is required for tomorrow-io")
		}
		return tomorrowio.NewClient(apiKey, baseURL), nil

	case "openweathermap":
		if apiKey == "" {
			return nil, fmt.Errorf("WEATHER_API_KEY is required for openweathermap")
		}
		return openweathermap.NewClient(apiKey, baseURL), nil

	case "open-meteo", "":
		return openmeteo.NewClient(baseURL), nil

	default:
		return nil, fmt.Errorf("unknown WEATHER_PROVIDER: %q (supported: open-meteo, tomorrow-io, openweathermap)", provider)
	}
}
