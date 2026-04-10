package plan

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// SpeedModel controls how ETAs are calculated.
type SpeedModel string

const (
	SpeedModelElevation SpeedModel = "elevation"
	SpeedModelFlat      SpeedModel = "flat"
)

// Preferences holds user-specified weighting factors for route scoring.
type Preferences struct {
	ShadeWeight    float64
	GreeneryWeight float64
	WindWeight     float64
}

// ErrTooFewStops is returned when a plan has fewer than 2 stops.
var ErrTooFewStops = errors.New("plan: at least 2 stops required")

// RoutePlan aggregates a planned cycling outing.
type RoutePlan struct {
	ID          string
	UserID      string
	DepartureAt time.Time
	SpeedModel  SpeedModel
	Preferences Preferences
	Stops       []StopPoint
	Tasks       []PlanTask
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewRoutePlan creates an empty RoutePlan for the given user.
func NewRoutePlan(userID string) *RoutePlan {
	now := time.Now().UTC()
	return &RoutePlan{
		ID:        newID(),
		UserID:    userID,
		Stops:     []StopPoint{},
		Tasks:     []PlanTask{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddStop appends a stop to the plan.
func (p *RoutePlan) AddStop(sp StopPoint) {
	p.Stops = append(p.Stops, sp)
	p.UpdatedAt = time.Now().UTC()
}

// RemoveStop removes the stop with the given ID from the plan.
func (p *RoutePlan) RemoveStop(id string) {
	filtered := p.Stops[:0]
	for _, s := range p.Stops {
		if s.ID != id {
			filtered = append(filtered, s)
		}
	}
	p.Stops = filtered
	p.UpdatedAt = time.Now().UTC()
}

// AddTask appends a task to the plan.
func (p *RoutePlan) AddTask(t PlanTask) {
	p.Tasks = append(p.Tasks, t)
	p.UpdatedAt = time.Now().UTC()
}

// Validate checks that the plan meets minimum requirements.
func (p *RoutePlan) Validate() error {
	if len(p.Stops) < 2 {
		return ErrTooFewStops
	}
	return nil
}

// newID generates a random UUID-like string using crypto/rand.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}
