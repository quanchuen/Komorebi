// internal/api/discovery_handler_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/discovery"
)

// stubDiscoveryRepoHTTP implements discovery.Repository for HTTP handler tests.
type stubDiscoveryRepoHTTP struct {
	results []discovery.DiscoveryResult
}

func (s *stubDiscoveryRepoHTTP) Nearby(_ discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	return s.results, nil
}
func (s *stubDiscoveryRepoHTTP) Viewport(_ discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	return s.results, nil
}
func (s *stubDiscoveryRepoHTTP) Suggested(_ discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	return s.results, nil
}

func newTestDiscoverySvc(results []discovery.DiscoveryResult) *app.DiscoveryService {
	return app.NewDiscoveryService(&stubDiscoveryRepoHTTP{results: results})
}

func TestDiscoveryHandler_Nearby_OK(t *testing.T) {
	svc := newTestDiscoverySvc([]discovery.DiscoveryResult{
		{RouteID: "r1", Name: "Loop A", DistanceM: 10000, DistFromM: 500, Tags: []string{}},
	})
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/nearby?lat=35.68&lon=139.69&radius_km=5", nil)
	rr := httptest.NewRecorder()
	h.Nearby(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	routes, ok := resp["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("expected 1 route in response, got %v", resp)
	}
}

func TestDiscoveryHandler_Nearby_MissingLat(t *testing.T) {
	svc := newTestDiscoverySvc(nil)
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/nearby?lon=139.69", nil)
	rr := httptest.NewRecorder()
	h.Nearby(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDiscoveryHandler_Viewport_OK(t *testing.T) {
	svc := newTestDiscoverySvc([]discovery.DiscoveryResult{
		{RouteID: "r2", Name: "Bay Loop", DistanceM: 8000, Tags: []string{"scenic"}},
	})
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/viewport?bbox=139.5,35.5,140.0,35.9", nil)
	rr := httptest.NewRecorder()
	h.Viewport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDiscoveryHandler_Viewport_BadBBox(t *testing.T) {
	svc := newTestDiscoverySvc(nil)
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/viewport?bbox=bad", nil)
	rr := httptest.NewRecorder()
	h.Viewport(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDiscoveryHandler_Suggested_OK(t *testing.T) {
	svc := newTestDiscoverySvc([]discovery.DiscoveryResult{
		{RouteID: "r3", Name: "Morning Ride", DistanceM: 15000, Tags: []string{}},
	})
	h := api.NewDiscoveryHandler(svc)

	dep := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/discover/suggested?lat=35.68&lon=139.69&departure_at="+dep, nil)
	rr := httptest.NewRecorder()
	h.Suggested(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
