package environment

import "time"

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
