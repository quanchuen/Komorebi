package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
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
