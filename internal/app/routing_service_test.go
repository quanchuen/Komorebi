package app_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// fakeValhallaClient is a test double for the valhalla.Client.
type fakeValhallaClient struct {
	result *valhalla.RouteResult
	err    error
}

func (f *fakeValhallaClient) Route(stops []valhalla.Location, profile valhalla.RouteProfile) (*valhalla.RouteResult, error) {
	return f.result, f.err
}

func twoStopPlan() []plan.StopPoint {
	return []plan.StopPoint{
		{ID: "a", Lat: 35.6895, Lon: 139.6917, Type: plan.StopManual, SortOrder: 0},
		{ID: "b", Lat: 35.6812, Lon: 139.7671, Type: plan.StopManual, SortOrder: 1},
	}
}

func fakeRouteResult() *valhalla.RouteResult {
	return &valhalla.RouteResult{
		TotalDistanceKm: 7.5,
		TotalDurationS:  1800,
		Legs: []valhalla.Leg{
			{
				DistanceKm: 7.5,
				DurationS:  1800,
				Shape: [][2]float64{
					{139.6917, 35.6895},
					{139.7000, 35.6850},
					{139.7671, 35.6812},
				},
			},
		},
	}
}

func TestRoutingService_GetDirections_Success(t *testing.T) {
	fake := &fakeValhallaClient{result: fakeRouteResult()}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops:       twoStopPlan(),
		DepartureAt: time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC),
		SpeedModel:  plan.SpeedModelElevation,
		Preferences: plan.Preferences{ShadeWeight: 0.5, GreeneryWeight: 0.5, WindWeight: 0.3},
	}

	result, err := svc.GetDirections(req)
	if err != nil {
		t.Fatalf("GetDirections() error: %v", err)
	}
	if len(result.Alternatives) == 0 {
		t.Fatal("expected at least one alternative")
	}
	alt := result.Alternatives[0]
	if alt.TotalDistanceKm != 7.5 {
		t.Errorf("expected distance 7.5, got %f", alt.TotalDistanceKm)
	}
	if len(alt.Legs) != 1 {
		t.Errorf("expected 1 leg, got %d", len(alt.Legs))
	}
	// GeoJSON geometry must be populated
	if len(alt.GeoJSON.Coordinates) == 0 {
		t.Error("expected non-empty GeoJSON coordinates")
	}
}

func TestRoutingService_GetDirections_TooFewStops(t *testing.T) {
	fake := &fakeValhallaClient{result: fakeRouteResult()}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops:       twoStopPlan()[:1],
		DepartureAt: time.Now(),
	}
	_, err := svc.GetDirections(req)
	if err == nil {
		t.Fatal("expected error for fewer than 2 stops")
	}
}

func TestRoutingService_GetDirections_ClientError(t *testing.T) {
	fake := &fakeValhallaClient{err: fmt.Errorf("valhalla: http 442: No path found")}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops:       twoStopPlan(),
		DepartureAt: time.Now(),
	}
	_, err := svc.GetDirections(req)
	if err == nil {
		t.Fatal("expected error when client returns error")
	}
}

func TestRoutingService_GetDirections_MultiStop(t *testing.T) {
	fake := &fakeValhallaClient{
		result: &valhalla.RouteResult{
			TotalDistanceKm: 15.0,
			TotalDurationS:  3600,
			Legs: []valhalla.Leg{
				{DistanceKm: 7.5, DurationS: 1800, Shape: [][2]float64{{139.69, 35.68}, {139.70, 35.69}}},
				{DistanceKm: 7.5, DurationS: 1800, Shape: [][2]float64{{139.70, 35.69}, {139.77, 35.68}}},
			},
		},
	}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops: []plan.StopPoint{
			{ID: "a", Lat: 35.6895, Lon: 139.6917, Type: plan.StopManual, SortOrder: 0},
			{ID: "b", Lat: 35.6900, Lon: 139.7000, Type: plan.StopManual, SortOrder: 1},
			{ID: "c", Lat: 35.6812, Lon: 139.7671, Type: plan.StopManual, SortOrder: 2},
		},
		DepartureAt: time.Now(),
	}
	result, err := svc.GetDirections(req)
	if err != nil {
		t.Fatalf("GetDirections() error: %v", err)
	}
	if len(result.Alternatives) == 0 {
		t.Fatal("expected at least one alternative")
	}
	alt := result.Alternatives[0]
	if len(alt.Legs) != 2 {
		t.Errorf("expected 2 legs, got %d", len(alt.Legs))
	}
	// Full geometry should concatenate both legs' shapes
	if len(alt.GeoJSON.Coordinates) < 3 {
		t.Errorf("expected merged geometry with at least 3 points, got %d", len(alt.GeoJSON.Coordinates))
	}
}
