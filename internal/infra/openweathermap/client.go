// Package openweathermap fetches hourly weather from the OpenWeatherMap API.
// Requires an API key (free tier: 60 calls/min, 1000 calls/day).
// Uses the One Call API 3.0: https://openweathermap.org/api/one-call-3
package openweathermap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

const defaultBaseURL = "https://api.openweathermap.org/data/3.0/onecall"

const cellHalfSide = 0.025

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an OpenWeatherMap client. apiKey is required.
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

func (c *Client) Name() string { return "openweathermap" }

// FetchMinutely returns per-minute precipitation for the next 60 min via One Call API.
func (c *Client) FetchMinutely(ctx context.Context, lat, lon float64) ([]environment.MinutelyPrecip, error) {
	url := fmt.Sprintf("%s?lat=%f&lon=%f&exclude=current,hourly,daily,alerts&units=metric&appid=%s",
		c.baseURL, lat, lon, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openweathermap minutely: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openweathermap minutely: HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Minutely []struct {
			Dt            int64   `json:"dt"`
			Precipitation float64 `json:"precipitation"` // mm/h
		} `json:"minutely"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("openweathermap minutely: decode: %w", err)
	}

	now := time.Now().UTC()
	result := make([]environment.MinutelyPrecip, 0, len(raw.Minutely))
	for _, m := range raw.Minutely {
		result = append(result, environment.MinutelyPrecip{
			Lat: lat, Lon: lon,
			At: time.Unix(m.Dt, 0).UTC(), IntensityMMH: m.Precipitation,
			FetchedAt: now,
		})
	}
	return result, nil
}

var _ environment.WeatherFetcher = (*Client)(nil)

func (c *Client) FetchPoint(ctx context.Context, lat, lon float64) ([]environment.WeatherGrid, error) {
	url := fmt.Sprintf("%s?lat=%f&lon=%f&exclude=current,minutely,daily,alerts&units=metric&appid=%s",
		c.baseURL, lat, lon, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openweathermap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openweathermap: HTTP %d", resp.StatusCode)
	}

	var raw oneCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("openweathermap: decode: %w", err)
	}

	return raw.toWeatherGrids(lat, lon)
}

func (c *Client) FetchGrid(ctx context.Context, minLat, maxLat, minLon, maxLon, stepDeg float64) ([]environment.WeatherGrid, error) {
	var all []environment.WeatherGrid
	for lat := minLat; lat <= maxLat+1e-9; lat += stepDeg {
		for lon := minLon; lon <= maxLon+1e-9; lon += stepDeg {
			cells, err := c.FetchPoint(ctx, lat, lon)
			if err != nil {
				return nil, fmt.Errorf("openweathermap: grid %f,%f: %w", lat, lon, err)
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

type oneCallResponse struct {
	Hourly []struct {
		Dt        int64   `json:"dt"`        // Unix timestamp
		Temp      float64 `json:"temp"`      // Celsius (units=metric)
		WindSpeed float64 `json:"wind_speed"` // m/s
		WindDeg   float64 `json:"wind_deg"`   // degrees
		Rain      *struct {
			OneH float64 `json:"1h"` // mm/h
		} `json:"rain,omitempty"`
	} `json:"hourly"`
}

func (r *oneCallResponse) toWeatherGrids(lat, lon float64) ([]environment.WeatherGrid, error) {
	cell := cellPolygon(lat, lon)
	grids := make([]environment.WeatherGrid, 0, len(r.Hourly))
	for _, h := range r.Hourly {
		precip := 0.0
		if h.Rain != nil {
			precip = h.Rain.OneH
		}
		grids = append(grids, environment.WeatherGrid{
			CellGeometry:       cell,
			ValidAt:            time.Unix(h.Dt, 0).UTC(),
			WindSpeedMS:        h.WindSpeed,
			WindBearingDeg:     h.WindDeg,
			PrecipIntensityMMH: precip,
			TemperatureC:       h.Temp,
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
