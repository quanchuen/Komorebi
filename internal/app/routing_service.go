package app

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"komorebi/internal/domain/plan"
	"komorebi/internal/infra/valhalla"
)

// ErrTooFewDirectionStops is returned when a directions request has fewer than 2 stops.
var ErrTooFewDirectionStops = errors.New("routing: at least 2 stops required")

// ValhallaRouter is the interface the RoutingService uses to call the routing engine.
type ValhallaRouter interface {
	Route(stops []valhalla.Location, profile valhalla.RouteProfile) (*valhalla.RouteResult, error)
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
	ETAAt      time.Time
}

// GeoJSONLineString represents a GeoJSON LineString geometry.
type GeoJSONLineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// DirectionsResult is the structured output of a single route alternative.
type DirectionsResult struct {
	Profile         string  `json:"profile"`
	Label           string  `json:"label"`
	TotalDistanceKm float64 `json:"total_distance_km"`
	TotalDurationS  float64 `json:"total_duration_s"`
	Legs            []LegResult
	GeoJSON         GeoJSONLineString `json:"geometry"`
}

// MultiDirectionsResult contains all route alternatives.
type MultiDirectionsResult struct {
	Alternatives []DirectionsResult `json:"alternatives"`
}

// RoutingService orchestrates multi-stop bicycle routing via Valhalla.
type RoutingService struct {
	router ValhallaRouter
}

// NewRoutingService creates a RoutingService backed by the given router client.
func NewRoutingService(router ValhallaRouter) *RoutingService {
	return &RoutingService{router: router}
}

var routeProfiles = []struct {
	profile valhalla.RouteProfile
	label   string
}{
	{valhalla.ProfileSuggested, "Suggested"},
	{valhalla.ProfileFast, "Fast"},
	{valhalla.ProfileAvoidMainRoads, "Avoid main roads"},
}

// GetDirections resolves stops, calls Valhalla with all 3 profiles in parallel,
// and returns route alternatives.
func (s *RoutingService) GetDirections(req DirectionsRequest) (*MultiDirectionsResult, error) {
	if len(req.Stops) < 2 {
		return nil, ErrTooFewDirectionStops
	}

	stops := make([]plan.StopPoint, len(req.Stops))
	copy(stops, req.Stops)
	sort.Slice(stops, func(i, j int) bool {
		return stops[i].SortOrder < stops[j].SortOrder
	})

	locs := make([]valhalla.Location, len(stops))
	for i, sp := range stops {
		locs[i] = valhalla.Location{Lat: sp.Lat, Lon: sp.Lon}
	}

	// Call all 3 profiles in parallel
	type result struct {
		idx int
		res *DirectionsResult
		err error
	}
	ch := make(chan result, len(routeProfiles))
	var wg sync.WaitGroup

	for idx, rp := range routeProfiles {
		wg.Add(1)
		go func(i int, profile valhalla.RouteProfile, label string) {
			defer wg.Done()
			raw, err := s.router.Route(locs, profile)
			if err != nil {
				ch <- result{i, nil, err}
				return
			}
			dr := buildDirectionsResult(raw, req.DepartureAt, string(profile), label)
			ch <- result{i, dr, nil}
		}(idx, rp.profile, rp.label)
	}

	go func() { wg.Wait(); close(ch) }()

	alternatives := make([]DirectionsResult, len(routeProfiles))
	var firstErr error
	received := 0
	for r := range ch {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		alternatives[r.idx] = *r.res
		received++
	}

	if received == 0 {
		return nil, fmt.Errorf("routing: all profiles failed: %w", firstErr)
	}

	// Filter out empty slots (failed profiles)
	var valid []DirectionsResult
	for _, a := range alternatives {
		if a.Profile != "" {
			valid = append(valid, a)
		}
	}

	return &MultiDirectionsResult{Alternatives: valid}, nil
}

// GetSingleDirections routes with a specific profile (for re-routing on stop changes).
func (s *RoutingService) GetSingleDirections(req DirectionsRequest, profile valhalla.RouteProfile) (*DirectionsResult, error) {
	if len(req.Stops) < 2 {
		return nil, ErrTooFewDirectionStops
	}

	stops := make([]plan.StopPoint, len(req.Stops))
	copy(stops, req.Stops)
	sort.Slice(stops, func(i, j int) bool {
		return stops[i].SortOrder < stops[j].SortOrder
	})

	locs := make([]valhalla.Location, len(stops))
	for i, sp := range stops {
		locs[i] = valhalla.Location{Lat: sp.Lat, Lon: sp.Lon}
	}

	raw, err := s.router.Route(locs, profile)
	if err != nil {
		return nil, fmt.Errorf("routing: valhalla error: %w", err)
	}

	label := "Suggested"
	for _, rp := range routeProfiles {
		if rp.profile == profile {
			label = rp.label
			break
		}
	}

	return buildDirectionsResult(raw, req.DepartureAt, string(profile), label), nil
}

func buildDirectionsResult(raw *valhalla.RouteResult, departure time.Time, profile, label string) *DirectionsResult {
	legs := make([]LegResult, len(raw.Legs))
	elapsed := 0.0
	for i, l := range raw.Legs {
		elapsed += l.DurationS
		legs[i] = LegResult{
			DistanceKm: l.DistanceKm,
			DurationS:  l.DurationS,
			ETAAt:      departure.Add(time.Duration(elapsed) * time.Second),
		}
	}

	var merged [][2]float64
	for i, l := range raw.Legs {
		if i == 0 {
			merged = append(merged, l.Shape...)
		} else if len(l.Shape) > 0 {
			merged = append(merged, l.Shape[1:]...)
		}
	}

	return &DirectionsResult{
		Profile:         profile,
		Label:           label,
		TotalDistanceKm: raw.TotalDistanceKm,
		TotalDurationS:  raw.TotalDurationS,
		Legs:            legs,
		GeoJSON: GeoJSONLineString{
			Type:        "LineString",
			Coordinates: merged,
		},
	}
}
