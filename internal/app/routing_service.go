package app

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// ErrTooFewDirectionStops is returned when a directions request has fewer than 2 stops.
var ErrTooFewDirectionStops = errors.New("routing: at least 2 stops required")

// ValhallaRouter is the interface the RoutingService uses to call the routing engine.
// Defined here so tests can inject a fake without importing the infra package.
type ValhallaRouter interface {
	Route(stops []valhalla.Location) (*valhalla.RouteResult, error)
}

// DirectionsRequest carries the inputs for a multi-stop route calculation.
type DirectionsRequest struct {
	Stops       []plan.StopPoint
	DepartureAt time.Time
	SpeedModel  plan.SpeedModel
	Preferences plan.Preferences
}

// LegResult is the per-leg breakdown returned to callers.
type LegResult struct {
	DistanceKm float64
	DurationS  float64
	// ETAAt is the projected arrival time at the end of this leg.
	ETAAt time.Time
}

// GeoJSONLineString represents a GeoJSON LineString geometry.
type GeoJSONLineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// DirectionsResult is the structured output of a routing request.
type DirectionsResult struct {
	TotalDistanceKm float64
	TotalDurationS  float64
	Legs            []LegResult
	// GeoJSON is the merged full-route geometry as a GeoJSON LineString.
	GeoJSON GeoJSONLineString
}

// RoutingService orchestrates multi-stop bicycle routing via Valhalla.
type RoutingService struct {
	router ValhallaRouter
}

// NewRoutingService creates a RoutingService backed by the given router client.
func NewRoutingService(router ValhallaRouter) *RoutingService {
	return &RoutingService{router: router}
}

// GetDirections resolves stops to coordinates, calls Valhalla, and returns the
// route geometry with per-leg ETAs.
//
// Venue-typed stops are passed through as-is in this initial implementation;
// venue resolution (snapping to nearest OSM match) is a future concern.
func (s *RoutingService) GetDirections(req DirectionsRequest) (*DirectionsResult, error) {
	if len(req.Stops) < 2 {
		return nil, ErrTooFewDirectionStops
	}

	// Sort by SortOrder to guarantee sequence before sending to Valhalla.
	stops := make([]plan.StopPoint, len(req.Stops))
	copy(stops, req.Stops)
	sort.Slice(stops, func(i, j int) bool {
		return stops[i].SortOrder < stops[j].SortOrder
	})

	// Translate domain stops to Valhalla locations.
	locs := make([]valhalla.Location, len(stops))
	for i, sp := range stops {
		locs[i] = valhalla.Location{Lat: sp.Lat, Lon: sp.Lon}
	}

	raw, err := s.router.Route(locs)
	if err != nil {
		return nil, fmt.Errorf("routing: valhalla error: %w", err)
	}

	// Build per-leg results with projected ETAs.
	legs := make([]LegResult, len(raw.Legs))
	elapsed := 0.0
	for i, l := range raw.Legs {
		elapsed += l.DurationS
		legs[i] = LegResult{
			DistanceKm: l.DistanceKm,
			DurationS:  l.DurationS,
			ETAAt:      req.DepartureAt.Add(time.Duration(elapsed) * time.Second),
		}
	}

	// Merge all leg shapes into a single LineString.
	// Deduplicate the shared point between consecutive legs (last point of leg N
	// equals first point of leg N+1).
	var merged [][2]float64
	for i, l := range raw.Legs {
		if i == 0 {
			merged = append(merged, l.Shape...)
		} else if len(l.Shape) > 0 {
			merged = append(merged, l.Shape[1:]...)
		}
	}

	return &DirectionsResult{
		TotalDistanceKm: raw.TotalDistanceKm,
		TotalDurationS:  raw.TotalDurationS,
		Legs:            legs,
		GeoJSON: GeoJSONLineString{
			Type:        "LineString",
			Coordinates: merged,
		},
	}, nil
}
