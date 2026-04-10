package environment

import "time"

// SegmentConditions aggregates all environmental factors for a route segment.
type SegmentConditions struct {
	Km          float64
	Shade       float64
	WindBenefit float64
	Precip      float64
	ETA         time.Time
	GreenWave   *GreenWave
	SignalCount  int
}
