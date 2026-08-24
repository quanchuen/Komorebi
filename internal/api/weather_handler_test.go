package api_test

import (
	"context"
	"encoding/json"
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
	cell     *environment.WeatherGrid
	grid     []environment.WeatherGrid
	minutely []environment.MinutelyPrecip
	err      error
}

func (f *fakeWeatherRepo) Upsert(_ []environment.WeatherGrid) error { return nil }
func (f *fakeWeatherRepo) AtPoint(_, _ float64, _ time.Time) (*environment.WeatherGrid, error) {
	return f.cell, f.err
}
func (f *fakeWeatherRepo) AlongRoute(_ []environment.WeatherSegmentQuery) ([]environment.WeatherGrid, error) {
	return nil, nil
}
func (f *fakeWeatherRepo) DeleteBefore(_ time.Time) error                        { return nil }
func (f *fakeWeatherRepo) UpsertMinutely(_ []environment.MinutelyPrecip) error   { return nil }
func (f *fakeWeatherRepo) MinutelyAt(_, _ float64, _, _ time.Time) ([]environment.MinutelyPrecip, error) {
	return nil, nil
}
func (f *fakeWeatherRepo) DeleteMinutelyBefore(_ time.Time) error { return nil }
func (f *fakeWeatherRepo) GridInBBox(_ context.Context, _ [4]float64, _ time.Time) ([]environment.WeatherGrid, error) {
	return f.grid, f.err
}
func (f *fakeWeatherRepo) MinutelySnapshot(_ context.Context, _ time.Time) ([]environment.MinutelyPrecip, error) {
	return f.minutely, nil
}

func TestWeatherHandler_AtPoint_OK(t *testing.T) {
	stub := &environment.WeatherGrid{
		ValidAt:            time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		WindSpeedMS:        4.2,
		WindBearingDeg:     270.0,
		PrecipIntensityMMH: 0.0,
		TemperatureC:       20.0,
	}
	repo := &fakeWeatherRepo{cell: stub}
	svc := app.NewWeatherService(repo, nil)
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
	svc := app.NewWeatherService(repo, nil)
	h := api.NewWeatherHandler(svc)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/weather/point?lat=0&lon=0", nil)
	rr := httptest.NewRecorder()
	h.AtPoint(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rr.Code)
	}
}

func gridCellFixture(validAt time.Time) environment.WeatherGrid {
	return environment.WeatherGrid{
		CellGeometry: [][2]float64{
			{139.60, 35.60}, {139.65, 35.60},
			{139.65, 35.65}, {139.60, 35.65},
			{139.60, 35.60},
		},
		ValidAt:            validAt,
		WindSpeedMS:        3.1,
		WindBearingDeg:     210,
		PrecipIntensityMMH: 0.4,
		TemperatureC:       29.0,
		UVIndex:            6.2,
	}
}

func TestWeatherHandler_Grid_OK(t *testing.T) {
	validAt := time.Date(2026, 8, 20, 5, 0, 0, 0, time.UTC)
	repo := &fakeWeatherRepo{grid: []environment.WeatherGrid{gridCellFixture(validAt)}}
	svc := app.NewWeatherService(repo, nil)
	h := api.NewWeatherHandler(svc)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/weather/grid?bbox=139.60,35.60,139.85,35.75&at=2026-08-20T05:10:00Z", nil)
	rr := httptest.NewRecorder()
	h.Grid(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		ValidAt time.Time `json:"valid_at"`
		Cells   []struct {
			MinLon             float64 `json:"min_lon"`
			MaxLat             float64 `json:"max_lat"`
			PrecipIntensityMMH float64 `json:"precip_intensity_mmh"`
			UVIndex            float64 `json:"uv_index"`
			PrecipSource       string  `json:"precip_source"`
		} `json:"cells"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.ValidAt.Equal(validAt) {
		t.Errorf("valid_at: want %v, got %v", validAt, body.ValidAt)
	}
	if len(body.Cells) != 1 {
		t.Fatalf("cells: want 1, got %d", len(body.Cells))
	}
	c := body.Cells[0]
	if c.MinLon != 139.60 || c.MaxLat != 35.65 {
		t.Errorf("bounds: got min_lon=%v max_lat=%v", c.MinLon, c.MaxLat)
	}
	if c.PrecipIntensityMMH != 0.4 || c.UVIndex != 6.2 {
		t.Errorf("values: got precip=%v uv=%v", c.PrecipIntensityMMH, c.UVIndex)
	}
	if c.PrecipSource != "hourly" {
		t.Errorf("precip_source: want hourly, got %q", c.PrecipSource)
	}
}

func TestWeatherHandler_Grid_DefaultAtAndEmpty(t *testing.T) {
	repo := &fakeWeatherRepo{}
	svc := app.NewWeatherService(repo, nil)
	h := api.NewWeatherHandler(svc)

	before := time.Now().UTC()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/weather/grid?bbox=139.60,35.60,139.85,35.75", nil)
	rr := httptest.NewRecorder()
	h.Grid(rr, req)
	after := time.Now().UTC()

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		ValidAt time.Time         `json:"valid_at"`
		Cells   []json.RawMessage `json:"cells"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Cells == nil || len(body.Cells) != 0 {
		t.Errorf("cells: want empty array, got %v", body.Cells)
	}
	if body.ValidAt.Before(before.Truncate(time.Second)) || body.ValidAt.After(after.Add(time.Second)) {
		t.Errorf("valid_at should default to now, got %v", body.ValidAt)
	}
}

func TestWeatherHandler_Grid_BadRequest(t *testing.T) {
	repo := &fakeWeatherRepo{}
	svc := app.NewWeatherService(repo, nil)
	h := api.NewWeatherHandler(svc)

	cases := map[string]string{
		"missing bbox":   "/api/v1/weather/grid",
		"malformed bbox": "/api/v1/weather/grid?bbox=139.60,35.60,139.85",
		"malformed at":   "/api/v1/weather/grid?bbox=139.60,35.60,139.85,35.75&at=today",
	}
	for name, url := range cases {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rr := httptest.NewRecorder()
		h.Grid(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, rr.Code)
		}
	}
}

func TestWeatherHandler_AtPoint_MissingLat(t *testing.T) {
	repo := &fakeWeatherRepo{}
	svc := app.NewWeatherService(repo, nil)
	h := api.NewWeatherHandler(svc)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/weather/point?lon=139.77", nil)
	rr := httptest.NewRecorder()
	h.AtPoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rr.Code)
	}
}
