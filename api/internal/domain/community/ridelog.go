package community

import "time"

// RideLog records a completed ride by a user.
type RideLog struct {
	ID        string
	UserID    string
	RouteID   string
	RiddenAt  int
	DurationS int
	GPXTrack  [][3]float64
	CreatedAt time.Time
}

// NewRideLog creates a new RideLog entry.
func NewRideLog(userID, routeID string, riddenAt, durationS int) (*RideLog, error) {
	return &RideLog{
		ID:        newID(),
		UserID:    userID,
		RouteID:   routeID,
		RiddenAt:  riddenAt,
		DurationS: durationS,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// SetGPXTrack attaches a GPS track to the ride log.
func (rl *RideLog) SetGPXTrack(track [][3]float64) {
	rl.GPXTrack = track
}
