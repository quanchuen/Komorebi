package app

import (
	"sort"

	"komorebi/internal/domain/route"
)

// Ad-hoc segmentation bounds. Segments target roughly 1/12 of the route so a
// typical ride yields enough samples for sparklines without flooding the
// environment queries; the clamp keeps very short and very long rides sane.
const (
	adHocMinSegmentKm = 0.25
	adHocMaxSegmentKm = 2.0
	adHocTargetSplits = 12
)

// BuildAdHocConditionRoute converts raw routing geometry (as returned by the
// routing engine, [lon, lat] pairs) plus an optional elevation profile into a
// transient route.Route whose Segments can feed EnvironmentService's
// GetRouteConditions. The route is never persisted.
func BuildAdHocConditionRoute(coords [][2]float64, elevation []ElevationPoint) *route.Route {
	if len(coords) < 2 {
		return &route.Route{Segments: []route.Segment{}}
	}

	// Cumulative distance in km at every coordinate.
	cum := make([]float64, len(coords))
	for i := 1; i < len(coords); i++ {
		cum[i] = cum[i-1] + haversineKm(coords[i-1][1], coords[i-1][0], coords[i][1], coords[i][0])
	}
	totalKm := cum[len(coords)-1]

	targetKm := totalKm / adHocTargetSplits
	if targetKm < adHocMinSegmentKm {
		targetKm = adHocMinSegmentKm
	}
	if targetKm > adHocMaxSegmentKm {
		targetKm = adHocMaxSegmentKm
	}

	elevAt := elevationInterpolator(elevation)

	var segments []route.Segment
	startIdx := 0
	for i := 1; i < len(coords); i++ {
		if cum[i]-cum[startIdx] < targetKm && i != len(coords)-1 {
			continue
		}
		geom := make([][3]float64, 0, i-startIdx+1)
		for j := startIdx; j <= i; j++ {
			geom = append(geom, [3]float64{coords[j][0], coords[j][1], elevAt(cum[j] * 1000)})
		}
		lengthKm := cum[i] - cum[startIdx]
		grade := 0.0
		if lengthKm > 0 {
			grade = (elevAt(cum[i]*1000) - elevAt(cum[startIdx]*1000)) / (lengthKm * 1000) * 100
		}
		segments = append(segments, route.Segment{
			Geometry:     geom,
			GradePercent: grade,
			SegmentOrder: len(segments),
		})
		startIdx = i
	}

	geometry := make([][3]float64, len(coords))
	for i, c := range coords {
		geometry[i] = [3]float64{c[0], c[1], elevAt(cum[i] * 1000)}
	}

	return &route.Route{
		Geometry:  geometry,
		DistanceM: totalKm * 1000,
		Segments:  segments,
	}
}

// elevationInterpolator returns a function mapping distance along the route
// (meters) to elevation (meters), linearly interpolating the profile. With no
// profile it returns 0 everywhere (flat), which degrades ETA speed, not
// correctness.
func elevationInterpolator(profile []ElevationPoint) func(distM float64) float64 {
	if len(profile) == 0 {
		return func(float64) float64 { return 0 }
	}
	pts := make([]ElevationPoint, len(profile))
	copy(pts, profile)
	sort.Slice(pts, func(i, j int) bool { return pts[i].DistanceM < pts[j].DistanceM })

	return func(distM float64) float64 {
		if distM <= pts[0].DistanceM {
			return pts[0].ElevationM
		}
		last := pts[len(pts)-1]
		if distM >= last.DistanceM {
			return last.ElevationM
		}
		idx := sort.Search(len(pts), func(i int) bool { return pts[i].DistanceM >= distM })
		lo, hi := pts[idx-1], pts[idx]
		span := hi.DistanceM - lo.DistanceM
		if span <= 0 {
			return lo.ElevationM
		}
		t := (distM - lo.DistanceM) / span
		return lo.ElevationM + t*(hi.ElevationM-lo.ElevationM)
	}
}
