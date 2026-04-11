// Package tomorrowio fetches hourly weather from the Tomorrow.io API.
// Requires an API key (free tier: 500 calls/day).
// https://docs.tomorrow.io/reference/weather-forecast
package tomorrowio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

const defaultBaseURL = "https://api.tomorrow.io/v4/weather/forecast"

// cellHalfSide matches Open-Meteo's 5 km grid for consistency.
const cellHalfSide = 0.025

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Tomorrow.io client. apiKey is required.
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Name() string { return "tomorrow-io" }

var _ environment.WeatherFetcher = (*Client)(nil)

func (c *Client) FetchPoint(ctx context.Context, lat, lon float64) ([]environment.WeatherGrid, error) {
	url := fmt.Sprintf("%s?location=%f,%f&timesteps=1h&apikey=%s",
		c.baseURL, lat, lon, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tomorrow-io: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tomorrow-io: HTTP %d", resp.StatusCode)
	}

	var raw forecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tomorrow-io: decode: %w", err)
	}

	return raw.toWeatherGrids(lat, lon)
}

func (c *Client) FetchGrid(ctx context.Context, minLat, maxLat, minLon, maxLon, stepDeg float64) ([]environment.WeatherGrid, error) {
	var all []environment.WeatherGrid
	for lat := minLat; lat <= maxLat+1e-9; lat += stepDeg {
		for lon := minLon; lon <= maxLon+1e-9; lon += stepDeg {
			cells, err := c.FetchPoint(ctx, lat, lon)
			if err != nil {
				return nil, fmt.Errorf("tomorrow-io: grid %f,%f: %w", lat, lon, err)
			}
			all = append(all, cells...)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond): // rate limit
			}
		}
	}
	return all, nil
}

// --- response types ---

type forecastResponse struct {
	Timelines struct {
		Hourly []struct {
			Time   string `json:"time"`
			Values struct {
				WindSpeed        float64 `json:"windSpeed"`
				WindDirection    float64 `json:"windDirection"`
				PrecipIntensity  float64 `json:"precipitationIntensity"`
				Temperature      float64 `json:"temperature"`
			} `json:"values"`
		} `json:"hourly"`
	} `json:"timelines"`
}

func (r *forecastResponse) toWeatherGrids(lat, lon float64) ([]environment.WeatherGrid, error) {
	cell := cellPolygon(lat, lon)
	grids := make([]environment.WeatherGrid, 0, len(r.Timelines.Hourly))
	for _, h := range r.Timelines.Hourly {
		t, err := time.Parse(time.RFC3339, h.Time)
		if err != nil {
			return nil, fmt.Errorf("tomorrow-io: parse time %q: %w", h.Time, err)
		}
		grids = append(grids, environment.WeatherGrid{
			CellGeometry:       cell,
			ValidAt:            t.UTC(),
			WindSpeedMS:        h.Values.WindSpeed,
			WindBearingDeg:     h.Values.WindDirection,
			PrecipIntensityMMH: h.Values.PrecipIntensity,
			TemperatureC:       h.Values.Temperature,
		})
	}
	return grids, nil
}

func cellPolygon(lat, lon float64) [][2]float64 {
	return [][2]float64{
		{lon - cellHalfSide, lat - cellHalfSide},
		{lon + cellHalfSide, lat - cellHalfSide},
		{lon + cellHalfSide, lat + cellHalfSide},
		{lon - cellHalfSide, lat + cellHalfSide},
		{lon - cellHalfSide, lat - cellHalfSide},
	}
}
