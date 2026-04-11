package valhalla_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// minimalValhallaResponse returns a Valhalla /route JSON response with one leg.
func minimalValhallaResponse() string {
	return `{
		"trip": {
			"summary": {"length": 5.23, "time": 1245},
			"legs": [
				{
					"summary": {"length": 5.23, "time": 1245},
					"shape": "efo}Hqsk|YuA` + "`" + `Bk@nA"
				}
			]
		}
	}`
}

func TestClient_Route_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/route" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		// Verify costing field is present in request
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("could not decode request body: %v", err)
		}
		if body["costing"] != "bicycle" {
			t.Errorf("expected costing=bicycle, got %v", body["costing"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(minimalValhallaResponse()))
	}))
	defer srv.Close()

	client := valhalla.NewClient(srv.URL)

	stops := []valhalla.Location{
		{Lat: 35.6895, Lon: 139.6917},
		{Lat: 35.6812, Lon: 139.7671},
	}
	result, err := client.Route(stops)
	if err != nil {
		t.Fatalf("Route() returned error: %v", err)
	}
	if result.TotalDistanceKm < 0.01 {
		t.Errorf("expected non-zero distance, got %f", result.TotalDistanceKm)
	}
	if len(result.Legs) != 1 {
		t.Errorf("expected 1 leg, got %d", len(result.Legs))
	}
	if result.Legs[0].DurationS < 1 {
		t.Errorf("expected non-zero duration, got %f", result.Legs[0].DurationS)
	}
	// Decoded shape must be non-empty
	if len(result.Legs[0].Shape) == 0 {
		t.Error("expected decoded shape coordinates, got empty slice")
	}
}

func TestClient_Route_MultiStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		locs, ok := body["locations"].([]any)
		if !ok || len(locs) != 3 {
			t.Errorf("expected 3 locations in request, got %v", body["locations"])
		}
		// Second location should be a via-point (type=through or break_through)
		// For multi-stop, middle stops should be "through" type
		resp := `{
			"trip": {
				"summary": {"length": 10.5, "time": 2500},
				"legs": [
					{"summary": {"length": 5.0, "time": 1200}, "shape": "efo}Hqsk|YuA` + "`" + `Bk@nA"},
					{"summary": {"length": 5.5, "time": 1300}, "shape": "efo}Hqsk|YuA` + "`" + `Bk@nA"}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	client := valhalla.NewClient(srv.URL)
	stops := []valhalla.Location{
		{Lat: 35.6895, Lon: 139.6917},
		{Lat: 35.6900, Lon: 139.7000},
		{Lat: 35.6812, Lon: 139.7671},
	}
	result, err := client.Route(stops)
	if err != nil {
		t.Fatalf("Route() returned error: %v", err)
	}
	if len(result.Legs) != 2 {
		t.Errorf("expected 2 legs for 3 stops, got %d", len(result.Legs))
	}
}

func TestClient_Route_ValhallaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "No path found", "error_code": 442}`))
	}))
	defer srv.Close()

	client := valhalla.NewClient(srv.URL)
	stops := []valhalla.Location{
		{Lat: 35.6895, Lon: 139.6917},
		{Lat: 35.6812, Lon: 139.7671},
	}
	_, err := client.Route(stops)
	if err == nil {
		t.Fatal("expected error for Valhalla 400 response, got nil")
	}
}

func TestClient_Route_TooFewStops(t *testing.T) {
	client := valhalla.NewClient("http://localhost:8002")
	_, err := client.Route([]valhalla.Location{{Lat: 35.0, Lon: 139.0}})
	if err == nil {
		t.Fatal("expected error for single stop, got nil")
	}
}
