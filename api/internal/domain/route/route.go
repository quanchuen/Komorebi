package route

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Difficulty describes how challenging a route is.
type Difficulty string

const (
	DifficultyEasy     Difficulty = "easy"
	DifficultyModerate Difficulty = "moderate"
	DifficultyHard     Difficulty = "hard"
	DifficultyExpert   Difficulty = "expert"
)

// Status represents the lifecycle state of a route.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

// ErrEmptyName is returned when a route is created without a name.
var ErrEmptyName = errors.New("route: name must not be empty")

// Route is the aggregate root for the route bounded context.
type Route struct {
	ID              string
	Name            string
	Description     string
	Geometry        [][3]float64
	DistanceM       float64
	ElevationGainM  float64
	ElevationLossM  float64
	Difficulty      Difficulty
	Status          Status
	Featured        bool // house pick shown as a suggested route; set in the database, not via the API
	CreatorID       string
	Tags            []string
	Waypoints       []Waypoint
	Segments        []Segment
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewRoute creates a new Route in Draft status.
func NewRoute(name, description string, difficulty Difficulty, creatorID string) (*Route, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	now := time.Now().UTC()
	return &Route{
		ID:          newID(),
		Name:        name,
		Description: description,
		Difficulty:  difficulty,
		Status:      StatusDraft,
		CreatorID:   creatorID,
		Tags:        []string{},
		Waypoints:   []Waypoint{},
		Segments:    []Segment{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// SetGeometry replaces the route geometry and derived metrics.
func (r *Route) SetGeometry(coords [][3]float64, distanceM, elevGainM, elevLossM float64) {
	r.Geometry = coords
	r.DistanceM = distanceM
	r.ElevationGainM = elevGainM
	r.ElevationLossM = elevLossM
	r.UpdatedAt = time.Now().UTC()
}

// AddWaypoint appends a waypoint to the route.
func (r *Route) AddWaypoint(wp Waypoint) {
	r.Waypoints = append(r.Waypoints, wp)
	r.UpdatedAt = time.Now().UTC()
}

// AddSegment appends a segment to the route.
func (r *Route) AddSegment(seg Segment) {
	r.Segments = append(r.Segments, seg)
	r.UpdatedAt = time.Now().UTC()
}

// SetTags replaces all tags on the route.
func (r *Route) SetTags(tags []string) {
	r.Tags = tags
	r.UpdatedAt = time.Now().UTC()
}

// newID generates a random UUID-like hex string using crypto/rand.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Format as UUID v4 (variant bits set)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}
