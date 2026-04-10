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

// Sentinel errors for route domain operations.
var (
	ErrEmptyName         = errors.New("route: name must not be empty")
	ErrInvalidTransition = errors.New("route: invalid status transition")
)

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

// Publish transitions the route from Draft to Published.
func (r *Route) Publish() error {
	if r.Status != StatusDraft {
		return ErrInvalidTransition
	}
	r.Status = StatusPublished
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// Archive transitions the route from Published to Archived.
func (r *Route) Archive() error {
	if r.Status != StatusPublished {
		return ErrInvalidTransition
	}
	r.Status = StatusArchived
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateMetadata replaces name, description, difficulty, and tags.
func (r *Route) UpdateMetadata(name, description string, difficulty Difficulty, tags []string) error {
	if name == "" {
		return ErrEmptyName
	}
	r.Name = name
	r.Description = description
	r.Difficulty = difficulty
	r.Tags = tags
	r.UpdatedAt = time.Now().UTC()
	return nil
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
