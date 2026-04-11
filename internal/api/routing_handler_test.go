package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
)

// fakeRoutingService is a test double for app.RoutingService.
type fakeRoutingService struct {
	result *app.MultiDirectionsResult
	err    error
}

func (f *fakeRoutingService) GetDirections(req app.DirectionsRequest) (*app.MultiDirectionsResult, error) {
	return f.result, f.err
}

func goodDirectionsResult() *app.MultiDirectionsResult {
	return &app.MultiDirectionsResult{
		Alternatives: []app.DirectionsResult{
			{
				TotalDistanceKm: 7.5,
				TotalDurationS:  1800,
				Legs: []app.LegResult{
					{
						DistanceKm: 7.5,
						DurationS:  1800,
						ETAAt:      time.Date(2026, 4, 10, 14, 30, 0, 0, time.UTC),
					},
				},
				GeoJSON: app.GeoJSONLineString{
					Type:        "LineString",
					Coordinates: [][2]float64{{139.6917, 35.6895}, {139.7671, 35.6812}},
				},
			},
		},
	}
}

func validDirectionsBody() []byte {
	body := map[string]any{
		"stops": []map[string]any{
			{"type": "manual", "lat": 35.6895, "lon": 139.6917},
			{"type": "manual", "lat": 35.6812, "lon": 139.7671},
		},
		"departure_at": "2026-04-10T14:00:00+09:00",
		"speed_model":  "elevation",
		"preferences": map[string]any{
			"shade": 0.8, "greenery": 0.5, "wind": 0.6,
		},
	}
	b, _ := json.Marshal(body)
	return b
}

func TestRoutingHandler_Directions_Success(t *testing.T) {
	fake := &fakeRoutingService{result: goodDirectionsResult()}
	handler := api.NewRoutingHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader(validDirectionsBody()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	alts, ok := resp["alternatives"].([]any)
	if !ok || len(alts) == 0 {
		t.Fatal("response missing or empty alternatives")
	}
	alt := alts[0].(map[string]any)
	if alt["total_distance_km"] == nil {
		t.Error("response missing total_distance_km")
	}
	if alt["geometry"] == nil {
		t.Error("response missing geometry")
	}
	legs, ok := alt["legs"].([]any)
	if !ok || len(legs) == 0 {
		t.Error("response missing or empty legs")
	}
}

func TestRoutingHandler_Directions_InvalidBody(t *testing.T) {
	fake := &fakeRoutingService{result: goodDirectionsResult()}
	handler := api.NewRoutingHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRoutingHandler_Directions_TooFewStops(t *testing.T) {
	fake := &fakeRoutingService{result: goodDirectionsResult()}
	handler := api.NewRoutingHandler(fake)

	body := map[string]any{
		"stops":        []map[string]any{{"type": "manual", "lat": 35.68, "lon": 139.69}},
		"departure_at": "2026-04-10T14:00:00+09:00",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRoutingHandler_Directions_ServiceError(t *testing.T) {
	fake := &fakeRoutingService{err: fmt.Errorf("valhalla unreachable")}
	handler := api.NewRoutingHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader(validDirectionsBody()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}
