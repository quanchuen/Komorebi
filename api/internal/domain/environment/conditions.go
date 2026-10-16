package environment

import "time"

// SegmentConditions aggregates all environmental factors for a route segment.
type SegmentConditions struct {
	Km float64
	// Shade is a value in [0.0, 1.0] where 1.0 = fully shaded, read from the
	// shadow grid cell at the segment's projected arrival hour slot and month.
	Shade       float64
	WindBenefit float64
	Precip      float64
	ETA         time.Time
	SignalCount int
}
