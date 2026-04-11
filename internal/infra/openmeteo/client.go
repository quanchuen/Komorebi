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

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

const (
	defaultBaseURL = "https://api.open-meteo.com/v1/forecast"
	// cellHalfSide is half the cell edge in degrees at Tokyo latitude (~5 km cell).
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

// Name returns the provider identifier.
func (c *Client) Name() string { return "open-meteo" }

// FetchMinutely returns 15-minute precipitation intervals for the next 24h.
// Open-Meteo supports minutely_15, not true per-minute. Returns nil if unavailable.
func (c *Client) FetchMinutely(ctx context.Context, lat, lon float64) ([]environment.MinutelyPrecip, error) {
	u := fmt.Sprintf("%s?latitude=%s&longitude=%s&minutely_15=precipitation",
		c.baseURL, strconv.FormatFloat(lat, 'f', 6, 64), strconv.FormatFloat(lon, 'f', 6, 64))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo minutely: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo minutely: HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Minutely15 struct {
			Time          []string  `json:"time"`
			Precipitation []float64 `json:"precipitation"`
		} `json:"minutely_15"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("open-meteo minutely: decode: %w", err)
	}

	now := time.Now().UTC()
	result := make([]environment.MinutelyPrecip, 0, len(raw.Minutely15.Time))
	for i, ts := range raw.Minutely15.Time {
		t, err := time.Parse("2006-01-02T15:04", ts)
		if err != nil {
			continue
		}
		precip := 0.0
		if i < len(raw.Minutely15.Precipitation) {
			precip = raw.Minutely15.Precipitation[i]
		}
		result = append(result, environment.MinutelyPrecip{
			Lat: lat, Lon: lon,
			At: t.UTC(), IntensityMMH: precip,
			FetchedAt: now,
		})
	}
	return result, nil
}

// Compile-time check that Client implements WeatherFetcher.
var _ environment.WeatherFetcher = (*Client)(nil)

// FetchPoint fetches hourly forecast for a single (lat, lon) and returns one
// WeatherGrid row per forecast hour. cell_geometry is a 5 km square centred on
// the point. The ValidAt times are in UTC.
func (c *Client) FetchPoint(ctx context.Context, lat, lon float64) ([]environment.WeatherGrid, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 6, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 6, 64))
	q.Set("hourly", "wind_speed_10m,wind_direction_10m,precipitation,temperature_2m,uv_index")
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
		Time          []string  `json:"time"`
		WindSpeed10M  []float64 `json:"wind_speed_10m"`
		WindDir10M    []float64 `json:"wind_direction_10m"`
		Precipitation []float64 `json:"precipitation"`
		Temperature2M []float64 `json:"temperature_2m"`
		UVIndex       []float64 `json:"uv_index"`
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
			UVIndex:            safeIndex(r.Hourly.UVIndex, i),
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
