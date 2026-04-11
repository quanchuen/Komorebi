package app

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	"github.com/cyclist-map/cyclist-map/internal/domain/route"
)

// stubEnvRepo returns fixed values for all queries.
type stubEnvRepo struct {
	shade       float64
	windSpeedMS float64
	windBearing float64
	precipMMH   float64
	signals     int
	greenWave   *environment.GreenWaveResult
}

func (s *stubEnvRepo) ShadeForPoint(_ context.Context, _, _ float64, _ time.Time) float64 {
	return s.shade
}
func (s *stubEnvRepo) WeatherForPoint(_ context.Context, _, _ float64, _ time.Time) (float64, float64, float64) {
	return s.windSpeedMS, s.windBearing, s.precipMMH
}
func (s *stubEnvRepo) GreeneryForWay(_ context.Context, _ int64) float64 { return 0 }
func (s *stubEnvRepo) SignalsAlongSegment(_ context.Context, _ string, _ float64) int {
	return s.signals
}
func (s *stubEnvRepo) GreenWaveForSegment(_ context.Context, _ string) *environment.GreenWaveResult {
	return s.greenWave
}

func makeTestRoute(gradePercent float64) *route.Route {
	// Simple two-point segment ~1 km long (roughly), flat.
	return &route.Route{
		ID: "test-route",
		Segments: []route.Segment{
			{
				ID:           "seg-1",
				GradePercent: gradePercent,
				SegmentOrder: 0,
				// ~1 km segment in Tokyo area
				Geometry: [][3]float64{
					{139.700, 35.680, 0},
					{139.710, 35.680, 0},
				},
			},
		},
	}
}

func TestGetRouteConditions_ZeroData(t *testing.T) {
	svc := NewEnvironmentService(&stubEnvRepo{})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       makeTestRoute(0),
		DepartureAt: time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 segment, got %d", len(results))
	}
	seg := results[0]
	if seg.Shade != 0 {
		t.Errorf("shade: got %v, want 0", seg.Shade)
	}
	if seg.WindBenefit != 0 {
		t.Errorf("wind_benefit: got %v, want 0", seg.WindBenefit)
	}
	if seg.Precip != 0 {
		t.Errorf("precip: got %v, want 0", seg.Precip)
	}
	if seg.ShadeColor == "" {
		t.Error("shade color must not be empty")
	}
}

func TestGetRouteConditions_ETAAccumulation(t *testing.T) {
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	svc := NewEnvironmentService(&stubEnvRepo{signals: 1})

	rt := makeTestRoute(0)
	// Add a second segment
	rt.Segments = append(rt.Segments, route.Segment{
		ID:           "seg-2",
		GradePercent: 0,
		SegmentOrder: 1,
		Geometry: [][3]float64{
			{139.710, 35.680, 0},
			{139.720, 35.680, 0},
		},
	})

	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       rt,
		DepartureAt: departure,
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 segments, got %d", len(results))
	}
	// Second segment ETA must be after first
	if !results[1].ETA.After(results[0].ETA) {
		t.Errorf("seg[1].ETA (%v) should be after seg[0].ETA (%v)", results[1].ETA, results[0].ETA)
	}
}

func TestGetRouteConditions_TailwindPositive(t *testing.T) {
	// Route bearing: east (~90°). Wind from west (270°) = tailwind.
	svc := NewEnvironmentService(&stubEnvRepo{
		windSpeedMS: 5,
		windBearing: 270, // wind FROM west
	})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       makeTestRoute(0),
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].WindBenefit <= 0 {
		t.Errorf("expected positive wind_benefit for tailwind, got %v", results[0].WindBenefit)
	}
}

func TestGetRouteConditions_HeadwindNegative(t *testing.T) {
	// Route bearing: east (~90°). Wind from east (90°) = headwind.
	svc := NewEnvironmentService(&stubEnvRepo{
		windSpeedMS: 5,
		windBearing: 90, // wind FROM east
	})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       makeTestRoute(0),
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].WindBenefit >= 0 {
		t.Errorf("expected negative wind_benefit for headwind, got %v", results[0].WindBenefit)
	}
}

func TestGetRouteConditions_EmptySegments(t *testing.T) {
	svc := NewEnvironmentService(&stubEnvRepo{})
	results, err := svc.GetRouteConditions(context.Background(), RouteConditionsRequest{
		Route:       &route.Route{ID: "empty"},
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("want 0 results for empty route, got %d", len(results))
	}
}

func TestComputeWindBenefit_ZeroSpeed(t *testing.T) {
	got := computeWindBenefit(0, 90, 90)
	if got != 0 {
		t.Errorf("expected 0 for zero wind speed, got %v", got)
	}
}

func TestNormalisePrecipApp(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{0, 0},
		{5, 0.5},
		{10, 1},
		{20, 1},
		{-1, 0},
	}
	for _, tc := range tests {
		got := normalisePrecipApp(tc.in)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("normalisePrecipApp(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
