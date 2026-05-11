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
			"time":               []string{"2026-04-10T00:00", "2026-04-10T01:00"},
			"wind_speed_10m":     []float64{3.5, 4.0},
			"wind_direction_10m": []float64{180.0, 185.0},
			"precipitation":      []float64{0.0, 0.2},
			"temperature_2m":     []float64{15.0, 14.5},
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
