package environment

import "math"

const (
	baseSpeedKmh   = 15.0
	uphillFactor   = 1.5
	downhillFactor = 1.0
	minSpeedKmh    = 4.0
	maxSpeedKmh    = 35.0
	signalPenaltyS = 30.0
)

// AdjustedSpeedKmh returns the estimated cycling speed for the given grade.
//
// Rules:
//   - Flat (0%): base speed 15 km/h
//   - Uphill (positive grade): base - grade*1.5, clamped to 4 km/h minimum
//   - Downhill (negative grade): base + abs(grade)*1.0, capped at 35 km/h
func AdjustedSpeedKmh(gradePercent float64) float64 {
	if gradePercent > 0 {
		speed := baseSpeedKmh - gradePercent*uphillFactor
		if speed < minSpeedKmh {
			speed = minSpeedKmh
		}
		return speed
	}
	if gradePercent < 0 {
		speed := baseSpeedKmh + math.Abs(gradePercent)*downhillFactor
		if speed > maxSpeedKmh {
			speed = maxSpeedKmh
		}
		return speed
	}
	return baseSpeedKmh
}

// GreenWaveOverride indicates a green wave is active and provides its target speed.
type GreenWaveOverride struct {
	TargetSpeedKmh float64
}

// SegmentETASeconds estimates travel time in seconds for a segment.
//
// If a GreenWaveOverride is provided, the green wave speed is used and signal
// penalties are ignored. Otherwise, the provided speedKmh is used and each
// signal adds 30 seconds.
func SegmentETASeconds(distanceKm, speedKmh float64, signals int, gw *GreenWaveOverride) float64 {
	if gw != nil {
		return (distanceKm / gw.TargetSpeedKmh) * 3600
	}
	travelTime := (distanceKm / speedKmh) * 3600
	signalDelay := float64(signals) * signalPenaltyS
	return travelTime + signalDelay
}
