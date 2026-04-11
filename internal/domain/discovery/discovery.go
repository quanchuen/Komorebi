// internal/domain/discovery/discovery.go
package discovery

import "time"

// NearbyParams holds parameters for a proximity search.
type NearbyParams struct {
	Lat      float64
	Lon      float64
	RadiusKm float64 // default applied by service if zero: 10 km
	Limit    int     // default 20, max 100
}

// ViewportParams holds parameters for a map-viewport search.
type ViewportParams struct {
	BBox  [4]float64 // [minLon, minLat, maxLon, maxLat]
	Limit int        // default 50, max 200
}

// SuggestedParams holds parameters for the suggested-routes query.
// In Phase 2 (this plan) scoring is proximity-only; environment scoring
// is deferred to Phase 3.
type SuggestedParams struct {
	Lat         float64
	Lon         float64
	DepartureAt time.Time
	Limit       int // default 10, max 50
}

// DiscoveryResult is a lightweight route summary returned by discovery queries.
// It carries only the fields needed to populate a route card in the UI.
type DiscoveryResult struct {
	RouteID        string   // UUID
	Name           string
	Description    string
	DistanceM      float64
	ElevationGainM float64
	ElevationLossM float64
	Difficulty     string
	Status         string
	Tags           []string
	// DistFromM is the distance in metres from the query point to the
	// nearest point on the route geometry. Zero for viewport queries.
	DistFromM float64
}
