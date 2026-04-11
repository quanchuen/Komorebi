package environment

import "time"

// SegmentConditions aggregates all environmental factors for a route segment.
type SegmentConditions struct {
	Km float64
	// Shade is a value in [0.0, 1.0] where 1.0 = fully shaded.
	// Populated from ShadowRepository.ForRoute at the segment's projected arrival
	// hour slot and month, averaged over intersecting grid cells.
	Shade         float64
	WindBenefit   float64
	Precip        float64
	ETA           time.Time
	GreenWave     *GreenWave
	SignalCount   int
	GreeneryScore float64 // 0.0–1.0; 0 when greenery_edge table is unpopulated
	UVIndex       float64 // 0-11+; UV radiation index at projected arrival time
}
