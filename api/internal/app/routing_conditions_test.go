package app

import (
	"math"
	"testing"
)

// straightLine builds n coordinates going north from a base point, spaced
// roughly stepM meters apart (1 degree latitude ≈ 111.19 km).
func straightLine(n int, stepM float64) [][2]float64 {
	coords := make([][2]float64, n)
	for i := range coords {
		coords[i] = [2]float64{139.7, 35.6 + float64(i)*stepM/111190.0}
	}
	return coords
}

func TestBuildAdHocConditionRouteDegenerateInputs(t *testing.T) {
	for _, coords := range [][][2]float64{nil, {{139.7, 35.6}}} {
		r := BuildAdHocConditionRoute(coords, nil)
		if len(r.Segments) != 0 {
			t.Fatalf("expected no segments for %d coords", len(coords))
		}
	}
}

func TestBuildAdHocConditionRouteSegmentation(t *testing.T) {
	// ~10 km line with 101 points 100 m apart.
	coords := straightLine(101, 100)
	r := BuildAdHocConditionRoute(coords, nil)

	if math.Abs(r.DistanceM-10000) > 200 {
		t.Fatalf("total distance %f, want ~10000", r.DistanceM)
	}
	// 10 km / 12 ≈ 0.83 km target → roughly 11-12 segments.
	if len(r.Segments) < 8 || len(r.Segments) > 14 {
		t.Fatalf("unexpected segment count %d", len(r.Segments))
	}
	// Segments must cover the route contiguously: each starts where the
	// previous ended, first starts at the route start, last ends at the end.
	prevEnd := r.Segments[0].Geometry[0]
	if prevEnd != r.Geometry[0] {
		t.Fatalf("first segment does not start at route start")
	}
	for i, seg := range r.Segments {
		if len(seg.Geometry) < 2 {
			t.Fatalf("segment %d has %d points", i, len(seg.Geometry))
		}
		if seg.Geometry[0] != prevEnd {
			t.Fatalf("segment %d not contiguous", i)
		}
		if seg.SegmentOrder != i {
			t.Fatalf("segment %d has order %d", i, seg.SegmentOrder)
		}
		prevEnd = seg.Geometry[len(seg.Geometry)-1]
	}
	if prevEnd != r.Geometry[len(r.Geometry)-1] {
		t.Fatalf("last segment does not end at route end")
	}
}

func TestBuildAdHocConditionRouteGradeFromElevation(t *testing.T) {
	// 1 km line climbing 50 m → 5% grade throughout.
	coords := straightLine(11, 100)
	elevation := []ElevationPoint{
		{DistanceM: 0, ElevationM: 100},
		{DistanceM: 1000, ElevationM: 150},
	}
	r := BuildAdHocConditionRoute(coords, elevation)
	if len(r.Segments) == 0 {
		t.Fatal("expected segments")
	}
	for i, seg := range r.Segments {
		if math.Abs(seg.GradePercent-5) > 0.5 {
			t.Fatalf("segment %d grade %f, want ~5", i, seg.GradePercent)
		}
	}
	// Elevation is attached to the geometry (used by sparklines downstream).
	first := r.Geometry[0][2]
	last := r.Geometry[len(r.Geometry)-1][2]
	if math.Abs(first-100) > 1 || math.Abs(last-150) > 6 {
		t.Fatalf("elevation not interpolated onto geometry: first %f last %f", first, last)
	}
}

func TestBuildAdHocConditionRouteWithoutElevationIsFlat(t *testing.T) {
	r := BuildAdHocConditionRoute(straightLine(11, 100), nil)
	for i, seg := range r.Segments {
		if seg.GradePercent != 0 {
			t.Fatalf("segment %d grade %f, want 0 without elevation data", i, seg.GradePercent)
		}
	}
}
