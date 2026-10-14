package app

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
	"komorebi/internal/domain/route"
)

// EnvironmentQuerier is the repository interface the EnvironmentService depends on.
// Defined here so tests can inject a stub without importing infra/postgres.
type EnvironmentQuerier interface {
	ShadeForPoint(ctx context.Context, lon, lat float64, at time.Time) float64
	WeatherForPoint(ctx context.Context, lon, lat float64, at time.Time) (windSpeedMS, windBearingDeg, precipMMH float64)
	GreeneryForWay(ctx context.Context, osmWayID int64) float64
	SignalsAlongSegment(ctx context.Context, segmentWKT string, bufferM float64) int
	GreenWaveForSegment(ctx context.Context, segmentWKT string) *environment.GreenWaveResult
}

// EnvironmentService computes time-projected per-segment conditions for a route.
type EnvironmentService struct {
	repo EnvironmentQuerier
}

// NewEnvironmentService creates an EnvironmentService backed by the given querier.
func NewEnvironmentService(repo EnvironmentQuerier) *EnvironmentService {
	return &EnvironmentService{repo: repo}
}

// SegmentConditionsResult extends the domain type with color values for all
// three overlays, computed by the Color LUT functions.
type SegmentConditionsResult struct {
	environment.SegmentConditions
	ShadeColor string
	WindColor  string
	RainColor  string
}

// RouteConditionsRequest carries the inputs needed to project conditions.
type RouteConditionsRequest struct {
	Route       *route.Route
	DepartureAt time.Time
	SpeedModel  plan.SpeedModel
}

// GetRouteConditions returns per-segment environment conditions for the route,
// projected forward in time from DepartureAt using the speed model.
//
// If any data source returns no rows for a segment, that segment's values
// default to zero — the function never returns an error due to missing env data.
func (s *EnvironmentService) GetRouteConditions(ctx context.Context, req RouteConditionsRequest) ([]SegmentConditionsResult, error) {
	segments := req.Route.Segments
	if len(segments) == 0 {
		return []SegmentConditionsResult{}, nil
	}

	results := make([]SegmentConditionsResult, 0, len(segments))
	elapsedS := 0.0
	cumulativeKm := 0.0

	for _, seg := range segments {
		distKm := segmentDistanceKm(seg.Geometry)
		centLon, centLat := segmentCentroid(seg.Geometry)
		segWKT := geometryToLineStringWKT(seg.Geometry)
		segBearingDeg := segmentBearing(seg.Geometry)

		// Check for green wave on this segment.
		gwResult := s.repo.GreenWaveForSegment(ctx, segWKT)
		var gwOverride *environment.GreenWaveOverride
		var gwDomain *environment.GreenWave
		if gwResult != nil {
			gwOverride = &environment.GreenWaveOverride{TargetSpeedKmh: gwResult.TargetSpeedKmh}
			gwDomain = &environment.GreenWave{
				ID:               gwResult.ID,
				TargetSpeedKmh:   gwResult.TargetSpeedKmh,
				DirectionBearing: gwResult.DirectionBearing,
				Confidence:       gwResult.Confidence,
			}
		}

		// Signal count (50 m buffer around segment).
		signals := s.repo.SignalsAlongSegment(ctx, segWKT, 50)

		// Speed for this segment.
		speedKmh := adjustedSpeed(seg.GradePercent, req.SpeedModel)

		// ETA seconds for this segment.
		etaS := environment.SegmentETASeconds(distKm, speedKmh, signals, gwOverride)

		// Project arrival time at the midpoint of this segment.
		arrivalAt := req.DepartureAt.Add(time.Duration(elapsedS+etaS/2) * time.Second)

		// Query environment data at projected arrival time.
		shade := s.repo.ShadeForPoint(ctx, centLon, centLat, arrivalAt)
		windSpeedMS, windBearingDeg, precipMMH := s.repo.WeatherForPoint(ctx, centLon, centLat, arrivalAt)

		windBenefit := computeWindBenefit(windSpeedMS, windBearingDeg, segBearingDeg)
		precipNorm := normalisePrecipApp(precipMMH)

		sc := environment.SegmentConditions{
			Km:          cumulativeKm,
			Shade:       shade,
			WindBenefit: windBenefit,
			Precip:      precipNorm,
			ETA:         req.DepartureAt.Add(time.Duration(elapsedS) * time.Second),
			GreenWave:   gwDomain,
			SignalCount: signals,
		}

		results = append(results, SegmentConditionsResult{
			SegmentConditions: sc,
			ShadeColor:        environment.ShadeColor(shade),
			WindColor:         environment.WindColor(windBenefit),
			RainColor:         environment.RainColor(precipNorm),
		})

		elapsedS += etaS
		cumulativeKm += distKm
	}

	return results, nil
}

// --- speed helper ---

// adjustedSpeed returns cycling speed for a segment based on grade and model.
// SpeedModelFlat always returns the base 15 km/h regardless of grade.
func adjustedSpeed(gradePercent float64, model plan.SpeedModel) float64 {
	if model == plan.SpeedModelFlat {
		return environment.AdjustedSpeedKmh(0)
	}
	return environment.AdjustedSpeedKmh(gradePercent)
}

// --- wind computation ---

// computeWindBenefit returns wind_benefit in [-1, +1].
//
// Tailwind (wind blows same direction as travel) → positive.
// Headwind (wind blows against the direction of travel) → negative.
// Crosswind → near zero.
//
// Formula: benefit = cos(angle_diff) * clamp(wind_speed / 10, 0, 1)
// where angle_diff is the difference between route bearing and wind origin.
//
// Returns 0 when wind speed is zero (no data).
func computeWindBenefit(windSpeedMS, windBearingDeg, routeBearingDeg float64) float64 {
	if windSpeedMS == 0 {
		return 0
	}
	// Wind bearing is the direction the wind is coming FROM.
	// Tailwind means wind comes from behind the rider (from opposite of route bearing).
	windFromDeg := windBearingDeg
	angleDiff := routeBearingDeg - (windFromDeg + 180)
	angleDiff = math.Mod(angleDiff+360, 360)
	if angleDiff > 180 {
		angleDiff -= 360
	}
	cosAngle := math.Cos(angleDiff * math.Pi / 180)
	speedFactor := windSpeedMS / 10
	if speedFactor > 1 {
		speedFactor = 1
	}
	return cosAngle * speedFactor
}

// --- geometry helpers ---

// segmentDistanceKm computes the total arc length of a segment in km
// using the haversine formula between consecutive coordinate pairs.
func segmentDistanceKm(coords [][3]float64) float64 {
	if len(coords) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(coords); i++ {
		total += haversineKm(coords[i-1][1], coords[i-1][0], coords[i][1], coords[i][0])
	}
	return total
}

// segmentCentroid returns the midpoint coordinate of the segment geometry.
func segmentCentroid(coords [][3]float64) (lon, lat float64) {
	if len(coords) == 0 {
		return 0, 0
	}
	mid := coords[len(coords)/2]
	return mid[0], mid[1]
}

// segmentBearing returns the approximate compass bearing (degrees, 0=N, 90=E)
// from the first to the last point of the segment.
func segmentBearing(coords [][3]float64) float64 {
	if len(coords) < 2 {
		return 0
	}
	first := coords[0]
	last := coords[len(coords)-1]
	dLon := (last[0] - first[0]) * math.Pi / 180
	lat1 := first[1] * math.Pi / 180
	lat2 := last[1] * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	bearing := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(bearing+360, 360)
}

// geometryToLineStringWKT encodes a segment geometry as a WKT LINESTRING Z.
func geometryToLineStringWKT(coords [][3]float64) string {
	if len(coords) == 0 {
		return "LINESTRING Z EMPTY"
	}
	pts := make([]string, len(coords))
	for i, c := range coords {
		pts[i] = formatCoord(c[0], c[1], c[2])
	}
	return "LINESTRING Z(" + strings.Join(pts, ", ") + ")"
}

func formatCoord(lon, lat, z float64) string {
	return fmt.Sprintf("%f %f %f", lon, lat, z)
}

// haversineKm returns the great-circle distance in km between two lat/lon points.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// normalisePrecipApp maps precip_intensity_mmh to [0, 1] (0 mm/h → 0; ≥10 mm/h → 1).
func normalisePrecipApp(mmh float64) float64 {
	if mmh <= 0 {
		return 0
	}
	if mmh >= 10 {
		return 1
	}
	return mmh / 10
}
