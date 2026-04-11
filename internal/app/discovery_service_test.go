// internal/app/discovery_service_test.go
package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/discovery"
)

// stubDiscoveryRepo implements discovery.Repository for tests.
type stubDiscoveryRepo struct {
	nearbyResults    []discovery.DiscoveryResult
	viewportResults  []discovery.DiscoveryResult
	suggestedResults []discovery.DiscoveryResult
	err              error
}

func (s *stubDiscoveryRepo) Nearby(_ discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	return s.nearbyResults, s.err
}
func (s *stubDiscoveryRepo) Viewport(_ discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	return s.viewportResults, s.err
}
func (s *stubDiscoveryRepo) Suggested(_ discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	return s.suggestedResults, s.err
}

func TestDiscoveryService_Nearby_DefaultRadius(t *testing.T) {
	stub := &stubDiscoveryRepo{
		nearbyResults: []discovery.DiscoveryResult{
			{RouteID: "r1", Name: "Loop A", DistFromM: 500},
		},
	}
	svc := app.NewDiscoveryService(stub)

	results, err := svc.Nearby(discovery.NearbyParams{Lat: 35.68, Lon: 139.69})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RouteID != "r1" {
		t.Fatalf("expected r1, got %s", results[0].RouteID)
	}
}

func TestDiscoveryService_Nearby_PropagatesError(t *testing.T) {
	stub := &stubDiscoveryRepo{err: errors.New("db down")}
	svc := app.NewDiscoveryService(stub)

	_, err := svc.Nearby(discovery.NearbyParams{Lat: 35.68, Lon: 139.69})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDiscoveryService_Viewport(t *testing.T) {
	stub := &stubDiscoveryRepo{
		viewportResults: []discovery.DiscoveryResult{
			{RouteID: "r2", Name: "Loop B"},
		},
	}
	svc := app.NewDiscoveryService(stub)

	results, err := svc.Viewport(discovery.ViewportParams{
		BBox: [4]float64{139.6, 35.6, 139.8, 35.8},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestDiscoveryService_Suggested(t *testing.T) {
	stub := &stubDiscoveryRepo{
		suggestedResults: []discovery.DiscoveryResult{
			{RouteID: "r3", Name: "Morning Ride"},
		},
	}
	svc := app.NewDiscoveryService(stub)

	results, err := svc.Suggested(discovery.SuggestedParams{
		Lat:         35.68,
		Lon:         139.69,
		DepartureAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
