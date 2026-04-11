// internal/domain/discovery/discovery_test.go
package discovery_test

import (
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/domain/discovery"
)

func TestNearbyParams_DefaultRadius(t *testing.T) {
	p := discovery.NearbyParams{Lat: 35.68, Lon: 139.69}
	if p.RadiusKm != 0 {
		t.Fatalf("expected zero default radius, got %f", p.RadiusKm)
	}
}

func TestViewportParams_BBoxOrder(t *testing.T) {
	p := discovery.ViewportParams{BBox: [4]float64{139.6, 35.6, 139.8, 35.8}}
	if p.BBox[0] >= p.BBox[2] {
		t.Fatalf("expected minLon < maxLon")
	}
	if p.BBox[1] >= p.BBox[3] {
		t.Fatalf("expected minLat < maxLat")
	}
}

func TestDiscoveryResult_Fields(t *testing.T) {
	r := discovery.DiscoveryResult{
		RouteID:    "abc",
		Name:       "Shinjuku Loop",
		DistanceM:  12000,
		DistFromM:  850.5,
		Difficulty: "moderate",
	}
	if r.RouteID == "" {
		t.Fatal("RouteID must not be empty")
	}
	if r.DistFromM < 0 {
		t.Fatal("DistFromM must be non-negative")
	}
}
