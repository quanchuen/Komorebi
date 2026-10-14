package environment

import "time"

// GreenWaveResult is the query result type returned by environment repositories.
// It is a lightweight DTO separate from the full GreenWave aggregate.
type GreenWaveResult struct {
	ID               string
	TargetSpeedKmh   float64
	DirectionBearing float64
	Confidence       float64
}

// GreenWaveSource indicates how a green wave was detected.
type GreenWaveSource string

const (
	GreenWaveRideLogInferred GreenWaveSource = "ride_log_inferred"
	GreenWaveUserReported    GreenWaveSource = "user_reported"
)

// GreenWave models a traffic signal coordination pattern on a set of OSM ways.
type GreenWave struct {
	ID               string
	OSMWayIDs        []int64
	DirectionBearing float64
	TargetSpeedKmh   float64
	Confidence       float64
	Source           GreenWaveSource
	DetectedAt       time.Time
}
