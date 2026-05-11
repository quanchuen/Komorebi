# Go API Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the Go API project with DDD structure, all domain types, PostgreSQL route repository, application service, HTTP handlers for Route CRUD, and a dev docker-compose.
**Architecture:** Hexagonal / ports-and-adapters inside a DDD bounded-context layout. Domain types are pure Go (zero external imports). Infrastructure adapters (postgres, valhalla, eventbus) implement domain repository interfaces. Application services orchestrate use cases and are injected into HTTP handlers.
**Tech Stack:** Go 1.22+, chi router, pgx v5, paulmach/orb (GeoJSON), google/uuid, PostGIS, Docker Compose (Valhalla + Martin + Go API).

---

## Task 1 — Go Module Init + Dependencies

**Files:**
- `go.mod` (create)
- `go.sum` (generated)
- `cmd/api/main.go` (create — minimal placeholder so the module compiles)

### Steps

- [ ] 1.1 Initialize the Go module and create the directory skeleton.

```bash
cd /Users/lug/src/cyclist-map
go mod init komorebi
```

- [ ] 1.2 Create a minimal `cmd/api/main.go` so the module compiles.

```go
// cmd/api/main.go
package main

import "fmt"

func main() {
	fmt.Println("cyclist-map API starting...")
}
```

- [ ] 1.3 Add core dependencies.

```bash
go get github.com/go-chi/chi/v5@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/jackc/pgx/v5/pgxpool@latest
go get github.com/google/uuid@latest
go get github.com/paulmach/orb@latest
```

- [ ] 1.4 Verify the project compiles.

```bash
go build ./cmd/api/...
```

Expected: no errors, produces binary.

- [ ] 1.5 Commit.

```bash
git add go.mod go.sum cmd/api/main.go
git commit -m "chore: init Go module with core dependencies"
```

---

## Task 2 — Domain Types: Route Context

**Files:**
- `internal/domain/route/route.go` (create)
- `internal/domain/route/segment.go` (create)
- `internal/domain/route/waypoint.go` (create)
- `internal/domain/route/repository.go` (create)
- `internal/domain/route/route_test.go` (create)

### Steps

- [ ] 2.1 Write test file first — tests for Route construction, validation, and state transitions.

```go
// internal/domain/route/route_test.go
package route_test

import (
	"testing"

	"komorebi/internal/domain/route"
)

func TestNewRoute_Valid(t *testing.T) {
	r, err := route.NewRoute("Tama River Trail", "Scenic riverside path", route.DifficultyEasy, "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if r.Name != "Tama River Trail" {
		t.Errorf("expected name 'Tama River Trail', got %q", r.Name)
	}
	if r.Status != route.StatusDraft {
		t.Errorf("expected status Draft, got %v", r.Status)
	}
}

func TestNewRoute_EmptyName(t *testing.T) {
	_, err := route.NewRoute("", "desc", route.DifficultyEasy, "user-123")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRoute_Publish(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyModerate, "user-123")
	err := r.Publish()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != route.StatusPublished {
		t.Errorf("expected Published, got %v", r.Status)
	}
}

func TestRoute_PublishTwice(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyModerate, "user-123")
	_ = r.Publish()
	err := r.Publish()
	if err == nil {
		t.Fatal("expected error publishing already-published route")
	}
}

func TestRoute_Archive(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyModerate, "user-123")
	_ = r.Publish()
	err := r.Archive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != route.StatusArchived {
		t.Errorf("expected Archived, got %v", r.Status)
	}
}

func TestRoute_ArchiveFromDraft(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyModerate, "user-123")
	err := r.Archive()
	if err == nil {
		t.Fatal("expected error archiving draft route")
	}
}

func TestRoute_AddWaypoint(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyEasy, "user-123")
	wp := route.Waypoint{
		Name:      "Viewpoint",
		Type:      route.WaypointViewpoint,
		Lat:       35.65,
		Lon:       139.70,
		SortOrder: 1,
	}
	r.AddWaypoint(wp)
	if len(r.Waypoints) != 1 {
		t.Fatalf("expected 1 waypoint, got %d", len(r.Waypoints))
	}
}

func TestRoute_AddSegment(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyEasy, "user-123")
	seg := route.Segment{
		SurfaceType:  route.SurfacePaved,
		GradePercent: 3.5,
		SegmentOrder: 1,
	}
	r.AddSegment(seg)
	if len(r.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(r.Segments))
	}
}

func TestRoute_SetTags(t *testing.T) {
	r, _ := route.NewRoute("Test", "desc", route.DifficultyEasy, "user-123")
	r.SetTags([]string{"riverside", "scenic", "family-friendly"})
	if len(r.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(r.Tags))
	}
}
```

- [ ] 2.2 Run tests — they should fail (types don't exist yet).

```bash
go test ./internal/domain/route/...
```

Expected: compilation errors.

- [ ] 2.3 Implement `route.go` — the aggregate root.

```go
// internal/domain/route/route.go
package route

import (
	"errors"
	"time"
)

// Difficulty levels for routes.
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

var (
	ErrEmptyName        = errors.New("route name must not be empty")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Route is the aggregate root for the Routes bounded context.
type Route struct {
	ID              string
	Name            string
	Description     string
	Geometry        [][3]float64 // LINESTRING Z as []{ lon, lat, elevation }
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

// NewRoute constructs a Route in Draft status with a generated UUID.
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
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Publish transitions Draft -> Published.
func (r *Route) Publish() error {
	if r.Status != StatusDraft {
		return ErrInvalidTransition
	}
	r.Status = StatusPublished
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// Archive transitions Published -> Archived (soft delete).
func (r *Route) Archive() error {
	if r.Status != StatusPublished {
		return ErrInvalidTransition
	}
	r.Status = StatusArchived
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateMetadata updates mutable fields on the route.
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

// SetGeometry sets the route line geometry and derived distance/elevation values.
func (r *Route) SetGeometry(coords [][3]float64, distanceM, elevGainM, elevLossM float64) {
	r.Geometry = coords
	r.DistanceM = distanceM
	r.ElevationGainM = elevGainM
	r.ElevationLossM = elevLossM
	r.UpdatedAt = time.Now().UTC()
}

// AddWaypoint appends a waypoint to the route.
func (r *Route) AddWaypoint(wp Waypoint) {
	wp.ID = newID()
	r.Waypoints = append(r.Waypoints, wp)
	r.UpdatedAt = time.Now().UTC()
}

// AddSegment appends a segment to the route.
func (r *Route) AddSegment(seg Segment) {
	seg.ID = newID()
	r.Segments = append(r.Segments, seg)
	r.UpdatedAt = time.Now().UTC()
}

// SetTags replaces the route's tag list.
func (r *Route) SetTags(tags []string) {
	r.Tags = tags
	r.UpdatedAt = time.Now().UTC()
}

// newID generates a new UUID string. Uses a simple approach to keep
// the domain package free of external dependencies.
func newID() string {
	// Implementation note: uses crypto/rand UUID v4 generation
	// to keep this package free of external imports.
	b := make([]byte, 16)
	_, _ = randReader(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return formatUUID(b)
}

func formatUUID(b []byte) string {
	return hexEncode(b[0:4]) + "-" +
		hexEncode(b[4:6]) + "-" +
		hexEncode(b[6:8]) + "-" +
		hexEncode(b[8:10]) + "-" +
		hexEncode(b[10:16])
}

func hexEncode(b []byte) string {
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}

// randReader is a package-level var so it can use crypto/rand without
// importing external packages. Set during init.
var randReader func([]byte) (int, error)

func init() {
	// Import crypto/rand at init time.
	// This keeps the domain free of *external* (third-party) imports
	// while still using stdlib crypto/rand.
	randReader = cryptoRandRead
}
```

- [ ] 2.4 Create the crypto helper (separate file to keep route.go clean).

```go
// internal/domain/route/rand.go
package route

import "crypto/rand"

func cryptoRandRead(b []byte) (int, error) {
	return rand.Read(b)
}
```

- [ ] 2.5 Implement `waypoint.go`.

```go
// internal/domain/route/waypoint.go
package route

// WaypointType categorizes a waypoint.
type WaypointType string

const (
	WaypointViewpoint WaypointType = "viewpoint"
	WaypointRestStop  WaypointType = "rest_stop"
	WaypointWater     WaypointType = "water"
	WaypointShrine    WaypointType = "shrine"
	WaypointKonbini   WaypointType = "konbini"
	WaypointOther     WaypointType = "other"
)

// Waypoint is a value object representing a point of interest on a route.
type Waypoint struct {
	ID        string
	Name      string
	Type      WaypointType
	Lat       float64
	Lon       float64
	SortOrder int
}
```

- [ ] 2.6 Implement `segment.go`.

```go
// internal/domain/route/segment.go
package route

// SurfaceType describes the road surface of a segment.
type SurfaceType string

const (
	SurfacePaved      SurfaceType = "paved"
	SurfaceGravel     SurfaceType = "gravel"
	SurfaceDirt       SurfaceType = "dirt"
	SurfaceCobblestone SurfaceType = "cobblestone"
)

// Segment is a value object representing a section of a route between waypoints.
type Segment struct {
	ID           string
	Geometry     [][3]float64 // LINESTRING Z
	SurfaceType  SurfaceType
	GradePercent float64
	SegmentOrder int
}
```

- [ ] 2.7 Implement `repository.go` — the port interface.

```go
// internal/domain/route/repository.go
package route

import "context"

// ListParams holds filters for listing routes.
type ListParams struct {
	BBox       *[4]float64 // [minLon, minLat, maxLon, maxLat]
	Difficulty *Difficulty
	Surface    *SurfaceType
	Tags       []string
	MinDistM   *float64
	MaxDistM   *float64
	Cursor     string
	Limit      int
}

// ListResult holds a page of routes with a cursor for the next page.
type ListResult struct {
	Routes     []*Route
	NextCursor string
}

// Repository is the port for route persistence.
type Repository interface {
	Create(ctx context.Context, r *Route) error
	GetByID(ctx context.Context, id string) (*Route, error)
	Update(ctx context.Context, r *Route) error
	List(ctx context.Context, params ListParams) (*ListResult, error)
	Delete(ctx context.Context, id string) error
}
```

- [ ] 2.8 Run tests — they should pass.

```bash
go test ./internal/domain/route/...
```

Expected: all tests pass.

- [ ] 2.9 Commit.

```bash
git add internal/domain/route/
git commit -m "feat: add Route domain types (aggregate, waypoint, segment, repository interface)"
```

---

## Task 3 — Domain Types: Plan Context

**Files:**
- `internal/domain/plan/plan.go` (create)
- `internal/domain/plan/stop_point.go` (create)
- `internal/domain/plan/plan_task.go` (create)
- `internal/domain/plan/repository.go` (create)
- `internal/domain/plan/plan_test.go` (create)

### Steps

- [ ] 3.1 Write tests first.

```go
// internal/domain/plan/plan_test.go
package plan_test

import (
	"testing"
	"time"

	"komorebi/internal/domain/plan"
)

func TestNewRoutePlan(t *testing.T) {
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	p, err := plan.NewRoutePlan("user-1", departure, plan.SpeedModelElevation, plan.Preferences{
		ShadeWeight:   0.8,
		GreeneryWeight: 0.5,
		WindWeight:     0.6,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if p.UserID != "user-1" {
		t.Errorf("expected user-1, got %q", p.UserID)
	}
}

func TestRoutePlan_AddStop(t *testing.T) {
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	p, _ := plan.NewRoutePlan("user-1", departure, plan.SpeedModelElevation, plan.Preferences{})
	stop := plan.StopPoint{
		Lat:       35.65,
		Lon:       139.70,
		Type:      plan.StopManual,
		SortOrder: 1,
	}
	p.AddStop(stop)
	if len(p.Stops) != 1 {
		t.Fatalf("expected 1 stop, got %d", len(p.Stops))
	}
}

func TestRoutePlan_AddTask(t *testing.T) {
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	p, _ := plan.NewRoutePlan("user-1", departure, plan.SpeedModelElevation, plan.Preferences{})
	task := plan.PlanTask{
		Description: "coffee at a cafe",
		Hashtag:     "#cafe",
		Status:      plan.TaskUnresolved,
	}
	p.AddTask(task)
	if len(p.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(p.Tasks))
	}
	if p.Tasks[0].Status != plan.TaskUnresolved {
		t.Errorf("expected Unresolved, got %v", p.Tasks[0].Status)
	}
}

func TestRoutePlan_NeedsMinimumStops(t *testing.T) {
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	p, _ := plan.NewRoutePlan("user-1", departure, plan.SpeedModelElevation, plan.Preferences{})
	p.AddStop(plan.StopPoint{Lat: 35.61, Lon: 139.67, Type: plan.StopManual, SortOrder: 1})
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for fewer than 2 stops")
	}
	p.AddStop(plan.StopPoint{Lat: 35.66, Lon: 139.54, Type: plan.StopManual, SortOrder: 2})
	err = p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] 3.2 Run tests — expect compilation failure.

```bash
go test ./internal/domain/plan/...
```

- [ ] 3.3 Implement `plan.go`.

```go
// internal/domain/plan/plan.go
package plan

import (
	"crypto/rand"
	"errors"
	"time"
)

// SpeedModel determines how ETAs are calculated.
type SpeedModel string

const (
	SpeedModelElevation SpeedModel = "elevation"
	SpeedModelFlat      SpeedModel = "flat"
)

// Preferences holds user-tunable weights for route optimization.
type Preferences struct {
	ShadeWeight    float64
	GreeneryWeight float64
	WindWeight     float64
}

var (
	ErrTooFewStops = errors.New("route plan requires at least 2 stops (origin + destination)")
)

// RoutePlan is the aggregate root for ride planning.
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

// NewRoutePlan creates a new plan in its initial state.
func NewRoutePlan(userID string, departureAt time.Time, speedModel SpeedModel, prefs Preferences) (*RoutePlan, error) {
	now := time.Now().UTC()
	return &RoutePlan{
		ID:          newID(),
		UserID:      userID,
		DepartureAt: departureAt,
		SpeedModel:  speedModel,
		Preferences: prefs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// AddStop appends a stop point to the plan.
func (p *RoutePlan) AddStop(s StopPoint) {
	s.ID = newID()
	p.Stops = append(p.Stops, s)
	p.UpdatedAt = time.Now().UTC()
}

// RemoveStop removes a stop by ID.
func (p *RoutePlan) RemoveStop(stopID string) error {
	for i, s := range p.Stops {
		if s.ID == stopID {
			p.Stops = append(p.Stops[:i], p.Stops[i+1:]...)
			p.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errors.New("stop not found")
}

// AddTask appends a plan task.
func (p *RoutePlan) AddTask(t PlanTask) {
	t.ID = newID()
	p.Tasks = append(p.Tasks, t)
	p.UpdatedAt = time.Now().UTC()
}

// Validate checks business invariants.
func (p *RoutePlan) Validate() error {
	if len(p.Stops) < 2 {
		return ErrTooFewStops
	}
	return nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	const h = "0123456789abcdef"
	dst := make([]byte, 36)
	idx := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			dst[idx] = '-'
			idx++
		}
		dst[idx] = h[v>>4]
		dst[idx+1] = h[v&0x0f]
		idx += 2
	}
	return string(dst)
}
```

- [ ] 3.4 Implement `stop_point.go`.

```go
// internal/domain/plan/stop_point.go
package plan

// StopType categorizes how a stop was created.
type StopType string

const (
	StopManual        StopType = "manual"
	StopVenueResolved StopType = "venue_resolved"
	StopWaypoint      StopType = "waypoint"
)

// StopPoint is a stop in a route plan.
type StopPoint struct {
	ID           string
	Lat          float64
	Lon          float64
	Type         StopType
	SortOrder    int
	VenueID      string // empty if not venue-resolved
	ResolvedName string
}
```

- [ ] 3.5 Implement `plan_task.go`.

```go
// internal/domain/plan/plan_task.go
package plan

// TaskStatus tracks venue resolution progress.
type TaskStatus string

const (
	TaskUnresolved TaskStatus = "unresolved"
	TaskMatched    TaskStatus = "matched"
	TaskCompleted  TaskStatus = "completed"
)

// PlanTask is a text task with optional venue hashtag resolution.
type PlanTask struct {
	ID              string
	Description     string
	Hashtag         string // e.g. "#cafe", "#konbini"
	Status          TaskStatus
	ResolvedVenueID string // set when Status == Matched
}
```

- [ ] 3.6 Implement `repository.go`.

```go
// internal/domain/plan/repository.go
package plan

import "context"

// Repository is the port for plan persistence.
type Repository interface {
	Create(ctx context.Context, p *RoutePlan) error
	GetByID(ctx context.Context, id string) (*RoutePlan, error)
	Update(ctx context.Context, p *RoutePlan) error
	Delete(ctx context.Context, id string) error
}
```

- [ ] 3.7 Run tests — expect pass.

```bash
go test ./internal/domain/plan/...
```

- [ ] 3.8 Commit.

```bash
git add internal/domain/plan/
git commit -m "feat: add Plan domain types (RoutePlan, StopPoint, PlanTask, repository interface)"
```

---

## Task 4 — Domain Types: Community Context

**Files:**
- `internal/domain/community/user.go` (create)
- `internal/domain/community/contribution.go` (create)
- `internal/domain/community/review.go` (create)
- `internal/domain/community/ridelog.go` (create)
- `internal/domain/community/repository.go` (create)
- `internal/domain/community/community_test.go` (create)

### Steps

- [ ] 4.1 Write tests first.

```go
// internal/domain/community/community_test.go
package community_test

import (
	"testing"
	"time"

	"komorebi/internal/domain/community"
)

func TestNewUser(t *testing.T) {
	u, err := community.NewUser("taro", "taro@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if u.DisplayName != "taro" {
		t.Errorf("expected 'taro', got %q", u.DisplayName)
	}
}

func TestNewUser_EmptyName(t *testing.T) {
	_, err := community.NewUser("", "email@example.com")
	if err == nil {
		t.Fatal("expected error for empty display name")
	}
}

func TestNewReview_Valid(t *testing.T) {
	r, err := community.NewReview("user-1", "route-1", 4, "Great ride!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Rating != 4 {
		t.Errorf("expected rating 4, got %d", r.Rating)
	}
}

func TestNewReview_InvalidRating(t *testing.T) {
	_, err := community.NewReview("user-1", "route-1", 0, "Bad")
	if err == nil {
		t.Fatal("expected error for rating 0")
	}
	_, err = community.NewReview("user-1", "route-1", 6, "Too good")
	if err == nil {
		t.Fatal("expected error for rating 6")
	}
}

func TestNewRideLog(t *testing.T) {
	rl := community.NewRideLog("user-1", "route-1", time.Now(), 3600)
	if rl.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if rl.DurationS != 3600 {
		t.Errorf("expected 3600, got %d", rl.DurationS)
	}
}

func TestNewContribution(t *testing.T) {
	c := community.NewContribution("user-1", [][3]float64{{139.7, 35.6, 10}}, map[string]any{"name": "My Route"})
	if c.Status != community.ContributionPending {
		t.Errorf("expected Pending, got %v", c.Status)
	}
}

func TestContribution_Approve(t *testing.T) {
	c := community.NewContribution("user-1", [][3]float64{{139.7, 35.6, 10}}, map[string]any{})
	err := c.Approve("Looks good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Status != community.ContributionApproved {
		t.Errorf("expected Approved, got %v", c.Status)
	}
}

func TestContribution_Reject(t *testing.T) {
	c := community.NewContribution("user-1", [][3]float64{{139.7, 35.6, 10}}, map[string]any{})
	err := c.Reject("Missing description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Status != community.ContributionRejected {
		t.Errorf("expected Rejected, got %v", c.Status)
	}
}
```

- [ ] 4.2 Run tests — expect compilation failure.

```bash
go test ./internal/domain/community/...
```

- [ ] 4.3 Implement `user.go`.

```go
// internal/domain/community/user.go
package community

import (
	"crypto/rand"
	"errors"
	"time"
)

var (
	ErrEmptyDisplayName = errors.New("display name must not be empty")
)

// User is the aggregate root for community identity.
type User struct {
	ID          string
	DisplayName string
	Email       string
	AvatarURL   string
	CreatedAt   time.Time
}

// NewUser creates a new user.
func NewUser(displayName, email string) (*User, error) {
	if displayName == "" {
		return nil, ErrEmptyDisplayName
	}
	return &User{
		ID:          newID(),
		DisplayName: displayName,
		Email:       email,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	const h = "0123456789abcdef"
	dst := make([]byte, 36)
	idx := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			dst[idx] = '-'
			idx++
		}
		dst[idx] = h[v>>4]
		dst[idx+1] = h[v&0x0f]
		idx += 2
	}
	return string(dst)
}
```

- [ ] 4.4 Implement `contribution.go`.

```go
// internal/domain/community/contribution.go
package community

import (
	"errors"
	"time"
)

// ContributionStatus tracks moderation state.
type ContributionStatus string

const (
	ContributionPending  ContributionStatus = "pending"
	ContributionApproved ContributionStatus = "approved"
	ContributionRejected ContributionStatus = "rejected"
)

var ErrContributionAlreadyDecided = errors.New("contribution already has a decision")

// Contribution is a user-submitted route pending moderation.
type Contribution struct {
	ID             string
	UserID         string
	RouteGeometry  [][3]float64       // LINESTRING Z
	Metadata       map[string]any     // arbitrary submission data (name, description, etc.)
	Status         ContributionStatus
	ModeratorNotes string
	SubmittedAt    time.Time
}

// NewContribution creates a pending contribution.
func NewContribution(userID string, geometry [][3]float64, metadata map[string]any) *Contribution {
	return &Contribution{
		ID:            newID(),
		UserID:        userID,
		RouteGeometry: geometry,
		Metadata:      metadata,
		Status:        ContributionPending,
		SubmittedAt:   time.Now().UTC(),
	}
}

// Approve marks the contribution as approved.
func (c *Contribution) Approve(notes string) error {
	if c.Status != ContributionPending {
		return ErrContributionAlreadyDecided
	}
	c.Status = ContributionApproved
	c.ModeratorNotes = notes
	return nil
}

// Reject marks the contribution as rejected.
func (c *Contribution) Reject(notes string) error {
	if c.Status != ContributionPending {
		return ErrContributionAlreadyDecided
	}
	c.Status = ContributionRejected
	c.ModeratorNotes = notes
	return nil
}
```

- [ ] 4.5 Implement `review.go`.

```go
// internal/domain/community/review.go
package community

import (
	"errors"
	"time"
)

var ErrInvalidRating = errors.New("rating must be between 1 and 5")

// Review is a user rating and comment on a route.
type Review struct {
	ID        string
	UserID    string
	RouteID   string
	Rating    int
	Body      string
	CreatedAt time.Time
}

// NewReview creates a review with validation.
func NewReview(userID, routeID string, rating int, body string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidRating
	}
	return &Review{
		ID:        newID(),
		UserID:    userID,
		RouteID:   routeID,
		Rating:    rating,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}
```

- [ ] 4.6 Implement `ridelog.go`.

```go
// internal/domain/community/ridelog.go
package community

import "time"

// RideLog records that a user rode a route, with an optional GPS track.
type RideLog struct {
	ID        string
	UserID    string
	RouteID   string
	RiddenAt  time.Time
	DurationS int
	GPXTrack  [][3]float64 // LINESTRING Z, nil if not recorded
	CreatedAt time.Time
}

// NewRideLog creates a ride log entry.
func NewRideLog(userID, routeID string, riddenAt time.Time, durationS int) *RideLog {
	return &RideLog{
		ID:        newID(),
		UserID:    userID,
		RouteID:   routeID,
		RiddenAt:  riddenAt,
		DurationS: durationS,
		CreatedAt: time.Now().UTC(),
	}
}

// SetGPXTrack attaches a GPS track to the ride log.
func (rl *RideLog) SetGPXTrack(track [][3]float64) {
	rl.GPXTrack = track
}
```

- [ ] 4.7 Implement `repository.go`.

```go
// internal/domain/community/repository.go
package community

import "context"

// UserRepository is the port for user persistence.
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id string) (*User, error)
}

// ReviewRepository is the port for review persistence.
type ReviewRepository interface {
	Create(ctx context.Context, r *Review) error
	ListByRoute(ctx context.Context, routeID string, cursor string, limit int) ([]*Review, string, error)
}

// RideLogRepository is the port for ride log persistence.
type RideLogRepository interface {
	Create(ctx context.Context, rl *RideLog) error
	ListByUser(ctx context.Context, userID string, cursor string, limit int) ([]*RideLog, string, error)
	ListByRoute(ctx context.Context, routeID string, cursor string, limit int) ([]*RideLog, string, error)
}

// ContributionRepository is the port for contribution persistence.
type ContributionRepository interface {
	Create(ctx context.Context, c *Contribution) error
	GetByID(ctx context.Context, id string) (*Contribution, error)
	Update(ctx context.Context, c *Contribution) error
}
```

- [ ] 4.8 Run tests — expect pass.

```bash
go test ./internal/domain/community/...
```

- [ ] 4.9 Commit.

```bash
git add internal/domain/community/
git commit -m "feat: add Community domain types (User, Contribution, Review, RideLog)"
```

---

## Task 5 — Domain Types: Environment Context

**Files:**
- `internal/domain/environment/shadow.go` (create)
- `internal/domain/environment/greenery.go` (create)
- `internal/domain/environment/weather.go` (create)
- `internal/domain/environment/venue.go` (create)
- `internal/domain/environment/green_wave.go` (create)
- `internal/domain/environment/signal.go` (create)
- `internal/domain/environment/conditions.go` (create)
- `internal/domain/environment/speed.go` (create)
- `internal/domain/environment/speed_test.go` (create)

### Steps

- [ ] 5.1 Write speed model tests first.

```go
// internal/domain/environment/speed_test.go
package environment_test

import (
	"math"
	"testing"

	"komorebi/internal/domain/environment"
)

func TestSpeedModel_Flat(t *testing.T) {
	speed := environment.AdjustedSpeedKmh(0.0)
	if speed != 15.0 {
		t.Errorf("expected 15.0 on flat, got %f", speed)
	}
}

func TestSpeedModel_Uphill(t *testing.T) {
	speed := environment.AdjustedSpeedKmh(5.0)
	expected := 15.0 - (5.0 * 1.5) // 7.5
	if math.Abs(speed-expected) > 0.01 {
		t.Errorf("expected %f, got %f", expected, speed)
	}
}

func TestSpeedModel_SteepUphill_Clamped(t *testing.T) {
	speed := environment.AdjustedSpeedKmh(15.0)
	if speed != 4.0 {
		t.Errorf("expected 4.0 (clamped), got %f", speed)
	}
}

func TestSpeedModel_Downhill(t *testing.T) {
	speed := environment.AdjustedSpeedKmh(-5.0)
	expected := 15.0 + (5.0 * 1.0) // 20.0
	if math.Abs(speed-expected) > 0.01 {
		t.Errorf("expected %f, got %f", expected, speed)
	}
}

func TestSpeedModel_SteepDownhill_Capped(t *testing.T) {
	speed := environment.AdjustedSpeedKmh(-30.0)
	if speed != 35.0 {
		t.Errorf("expected 35.0 (capped), got %f", speed)
	}
}

func TestSegmentETA_WithSignals(t *testing.T) {
	eta := environment.SegmentETASeconds(1.0, 15.0, 3, nil)
	// 1 km at 15 km/h = 240s + 3 signals * 30s = 330s
	if math.Abs(eta-330.0) > 0.01 {
		t.Errorf("expected 330.0, got %f", eta)
	}
}

func TestSegmentETA_GreenWaveOverride(t *testing.T) {
	gw := &environment.GreenWaveOverride{TargetSpeedKmh: 20.0}
	eta := environment.SegmentETASeconds(2.0, 15.0, 5, gw)
	// Green wave: 2 km at 20 km/h = 360s, signals ignored
	if math.Abs(eta-360.0) > 0.01 {
		t.Errorf("expected 360.0, got %f", eta)
	}
}
```

- [ ] 5.2 Run tests — expect compilation failure.

```bash
go test ./internal/domain/environment/...
```

- [ ] 5.3 Implement `shadow.go`.

```go
// internal/domain/environment/shadow.go
package environment

// ShadowGrid represents precomputed shade data for a spatial cell.
type ShadowGrid struct {
	ID            string
	CellGeometry  [][2]float64 // polygon ring as [][lon, lat]
	HourSlot      int          // 0-23
	Month         int          // 1-12
	ShadeCoverage float64      // 0.0 to 1.0
}
```

- [ ] 5.4 Implement `greenery.go`.

```go
// internal/domain/environment/greenery.go
package environment

// GreeneryIndex holds per-edge greenery scores derived from OSM data.
type GreeneryIndex struct {
	OSMWayID     int64
	GreeneryScore float64 // 0.0 to 1.0
	TreeLined     bool
	ParkAdjacent  bool
}
```

- [ ] 5.5 Implement `weather.go`.

```go
// internal/domain/environment/weather.go
package environment

import "time"

// WeatherGrid holds hourly weather data for a spatial cell.
type WeatherGrid struct {
	ID                string
	CellGeometry      [][2]float64 // polygon ring
	ValidAt           time.Time
	WindSpeedMS       float64
	WindBearingDeg    float64
	PrecipIntensityMMH float64
	TemperatureC      float64
}
```

- [ ] 5.6 Implement `venue.go`.

```go
// internal/domain/environment/venue.go
package environment

// Venue represents a point of interest from OSM data.
type Venue struct {
	ID       string
	OSMID    int64
	Lat      float64
	Lon      float64
	Name     string
	Category string
	Brand    string         // empty if no brand
	OSMTags  map[string]string
}

// VenueTagMapping maps a hashtag to an OSM filter for venue resolution.
type VenueTagMapping struct {
	Hashtag     string
	OSMFilter   map[string]string // key-value pairs for OSM tag matching
	Description string
	IsBrand     bool
}
```

- [ ] 5.7 Implement `green_wave.go`.

```go
// internal/domain/environment/green_wave.go
package environment

import "time"

// GreenWaveSource describes how a green wave was detected.
type GreenWaveSource string

const (
	GreenWaveRideLogInferred GreenWaveSource = "ride_log_inferred"
	GreenWaveUserReported    GreenWaveSource = "user_reported"
)

// GreenWave represents a detected coordinated traffic signal corridor.
type GreenWave struct {
	ID              string
	OSMWayIDs       []int64
	DirectionBearing float64
	TargetSpeedKmh  float64
	Confidence      float64
	Source          GreenWaveSource
	DetectedAt      time.Time
}
```

- [ ] 5.8 Implement `signal.go`.

```go
// internal/domain/environment/signal.go
package environment

// TrafficSignal represents an OSM traffic signal node.
type TrafficSignal struct {
	OSMNodeID int64
	Lat       float64
	Lon       float64
}
```

- [ ] 5.9 Implement `conditions.go` — the per-segment conditions payload.

```go
// internal/domain/environment/conditions.go
package environment

import "time"

// SegmentConditions represents time-projected environmental conditions
// for a single route segment.
type SegmentConditions struct {
	KM          float64        `json:"km"`
	ETA         time.Time      `json:"eta"`
	Shade       float64        `json:"shade"`        // 0.0 to 1.0
	WindBenefit float64        `json:"wind_benefit"`  // -1.0 (headwind) to 1.0 (tailwind)
	Precip      float64        `json:"precip"`        // mm/h
	GreenWave   *GreenWaveInfo `json:"green_wave"`    // nil if none
	Signals     int            `json:"signals"`
}

// GreenWaveInfo is the inline green wave data in a conditions response.
type GreenWaveInfo struct {
	SpeedKmh float64 `json:"speed_kmh"`
	LengthKm float64 `json:"length_km"`
}

// RouteConditions is the full conditions response for a route.
type RouteConditions struct {
	Segments []SegmentConditions `json:"segments"`
}
```

- [ ] 5.10 Implement `speed.go` — the speed model.

```go
// internal/domain/environment/speed.go
package environment

const (
	baseSpeedKmh     = 15.0
	uphillFactor     = 1.5
	downhillFactor   = 1.0
	minSpeedKmh      = 4.0
	maxSpeedKmh      = 35.0
	signalPenaltySec = 30.0
)

// GreenWaveOverride holds the green wave target speed when applicable.
type GreenWaveOverride struct {
	TargetSpeedKmh float64
}

// AdjustedSpeedKmh returns the elevation-adjusted speed for a given
// grade percentage. Positive grade = uphill, negative = downhill.
func AdjustedSpeedKmh(gradePercent float64) float64 {
	var speed float64
	if gradePercent >= 0 {
		speed = baseSpeedKmh - (gradePercent * uphillFactor)
		if speed < minSpeedKmh {
			speed = minSpeedKmh
		}
	} else {
		speed = baseSpeedKmh + (-gradePercent * downhillFactor)
		if speed > maxSpeedKmh {
			speed = maxSpeedKmh
		}
	}
	return speed
}

// SegmentETASeconds calculates the time in seconds to traverse a segment.
// If a green wave override is provided, it uses the green wave speed and
// ignores signal penalties.
func SegmentETASeconds(distanceKm, speedKmh float64, signals int, gw *GreenWaveOverride) float64 {
	if gw != nil {
		return (distanceKm / gw.TargetSpeedKmh) * 3600.0
	}
	travelSec := (distanceKm / speedKmh) * 3600.0
	return travelSec + float64(signals)*signalPenaltySec
}
```

- [ ] 5.11 Run tests — expect pass.

```bash
go test ./internal/domain/environment/...
```

- [ ] 5.12 Commit.

```bash
git add internal/domain/environment/
git commit -m "feat: add Environment domain types (Shadow, Greenery, Weather, Venue, GreenWave, Signal, Conditions, Speed model)"
```

---

## Task 6 — Domain Events

**Files:**
- `internal/domain/events/events.go` (create)
- `internal/domain/events/events_test.go` (create)

### Steps

- [ ] 6.1 Write tests first.

```go
// internal/domain/events/events_test.go
package events_test

import (
	"testing"
	"time"

	"komorebi/internal/domain/events"
)

func TestContributionApprovedEvent(t *testing.T) {
	e := events.ContributionApproved{
		ContributionID: "contrib-1",
		RouteID:        "route-1",
		OccurredAt:     time.Now(),
	}
	if e.EventName() != "contribution.approved" {
		t.Errorf("expected 'contribution.approved', got %q", e.EventName())
	}
}

func TestRideLogCreatedEvent(t *testing.T) {
	e := events.RideLogCreated{
		RideLogID:  "log-1",
		UserID:     "user-1",
		RouteID:    "route-1",
		HasGPX:     true,
		OccurredAt: time.Now(),
	}
	if e.EventName() != "ridelog.created" {
		t.Errorf("expected 'ridelog.created', got %q", e.EventName())
	}
}

func TestRouteCreatedEvent(t *testing.T) {
	e := events.RouteCreated{
		RouteID:    "route-1",
		CreatorID:  "user-1",
		OccurredAt: time.Now(),
	}
	if e.EventName() != "route.created" {
		t.Errorf("expected 'route.created', got %q", e.EventName())
	}
}

func TestRoutePublishedEvent(t *testing.T) {
	e := events.RoutePublished{
		RouteID:    "route-1",
		OccurredAt: time.Now(),
	}
	if e.EventName() != "route.published" {
		t.Errorf("expected 'route.published', got %q", e.EventName())
	}
}
```

- [ ] 6.2 Run tests — expect compilation failure.

```bash
go test ./internal/domain/events/...
```

- [ ] 6.3 Implement `events.go`.

```go
// internal/domain/events/events.go
package events

import "time"

// Event is the base interface for all domain events.
type Event interface {
	EventName() string
}

// RouteCreated is emitted when a new route is created.
type RouteCreated struct {
	RouteID    string
	CreatorID  string
	OccurredAt time.Time
}

func (e RouteCreated) EventName() string { return "route.created" }

// RoutePublished is emitted when a route transitions to Published.
type RoutePublished struct {
	RouteID    string
	OccurredAt time.Time
}

func (e RoutePublished) EventName() string { return "route.published" }

// RouteArchived is emitted when a route is soft-deleted.
type RouteArchived struct {
	RouteID    string
	OccurredAt time.Time
}

func (e RouteArchived) EventName() string { return "route.archived" }

// ContributionApproved is emitted when a contribution passes moderation.
// Triggers route creation in the Routes context.
type ContributionApproved struct {
	ContributionID string
	RouteID        string
	OccurredAt     time.Time
}

func (e ContributionApproved) EventName() string { return "contribution.approved" }

// RideLogCreated is emitted when a user logs a ride.
// If HasGPX is true, triggers green wave inference in the Environment context.
type RideLogCreated struct {
	RideLogID  string
	UserID     string
	RouteID    string
	HasGPX     bool
	OccurredAt time.Time
}

func (e RideLogCreated) EventName() string { return "ridelog.created" }
```

- [ ] 6.4 Run tests — expect pass.

```bash
go test ./internal/domain/events/...
```

- [ ] 6.5 Commit.

```bash
git add internal/domain/events/
git commit -m "feat: add domain event definitions (Route, Contribution, RideLog events)"
```

---

## Task 7 — PostgreSQL Route Repository

**Files:**
- `internal/infra/postgres/route_repo.go` (create)
- `internal/infra/postgres/route_repo_test.go` (create)

### Steps

- [ ] 7.1 Write integration test (uses build tag `integration` so it only runs when a DB is available).

```go
// internal/infra/postgres/route_repo_test.go
//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"komorebi/internal/domain/route"
	"komorebi/internal/infra/postgres"
)

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/cyclist_map_test?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestRouteRepo_CreateAndGet(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewRouteRepository(pool)
	ctx := context.Background()

	r, err := route.NewRoute("Integration Test Route", "A test route", route.DifficultyEasy, "user-1")
	if err != nil {
		t.Fatalf("failed to create route: %v", err)
	}
	r.SetGeometry([][3]float64{{139.7, 35.6, 10}, {139.71, 35.61, 15}}, 1500.0, 50.0, 20.0)
	r.SetTags([]string{"test", "riverside"})
	r.AddWaypoint(route.Waypoint{Name: "Start", Type: route.WaypointOther, Lat: 35.6, Lon: 139.7, SortOrder: 1})

	err = repo.Create(ctx, r)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "Integration Test Route" {
		t.Errorf("expected name 'Integration Test Route', got %q", got.Name)
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}
	if len(got.Waypoints) != 1 {
		t.Errorf("expected 1 waypoint, got %d", len(got.Waypoints))
	}

	// Cleanup
	_ = repo.Delete(ctx, r.ID)
}

func TestRouteRepo_List(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewRouteRepository(pool)
	ctx := context.Background()

	r1, _ := route.NewRoute("Route A", "desc", route.DifficultyEasy, "user-1")
	r1.SetGeometry([][3]float64{{139.7, 35.6, 10}, {139.71, 35.61, 15}}, 1000.0, 10.0, 5.0)
	_ = repo.Create(ctx, r1)

	r2, _ := route.NewRoute("Route B", "desc", route.DifficultyHard, "user-1")
	r2.SetGeometry([][3]float64{{139.72, 35.62, 20}, {139.73, 35.63, 25}}, 5000.0, 200.0, 150.0)
	_ = repo.Create(ctx, r2)

	result, err := repo.List(ctx, route.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Routes) < 2 {
		t.Errorf("expected at least 2 routes, got %d", len(result.Routes))
	}

	// Cleanup
	_ = repo.Delete(ctx, r1.ID)
	_ = repo.Delete(ctx, r2.ID)
}
```

- [ ] 7.2 Implement `route_repo.go`.

```go
// internal/infra/postgres/route_repo.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"komorebi/internal/domain/route"
)

// RouteRepository implements route.Repository using pgx + PostGIS.
type RouteRepository struct {
	pool *pgxpool.Pool
}

// NewRouteRepository creates a new PostgreSQL-backed route repository.
func NewRouteRepository(pool *pgxpool.Pool) *RouteRepository {
	return &RouteRepository{pool: pool}
}

// Create inserts a route with its waypoints, segments, and tags in a transaction.
func (r *RouteRepository) Create(ctx context.Context, rt *route.Route) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	geomWKT := lineStringZToWKT(rt.Geometry)

	_, err = tx.Exec(ctx, `
		INSERT INTO routes.route (id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id, created_at, updated_at)
		VALUES ($1, $2, $3, ST_GeomFromText($4, 4326), $5, $6, $7, $8, $9, $10, $11, $12)`,
		rt.ID, rt.Name, rt.Description, geomWKT,
		rt.DistanceM, rt.ElevationGainM, rt.ElevationLossM,
		string(rt.Difficulty), string(rt.Status), rt.CreatorID,
		rt.CreatedAt, rt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert route: %w", err)
	}

	for _, tag := range rt.Tags {
		_, err = tx.Exec(ctx, `INSERT INTO routes.route_tag (route_id, tag) VALUES ($1, $2)`, rt.ID, tag)
		if err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	for _, wp := range rt.Waypoints {
		_, err = tx.Exec(ctx, `
			INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order)
			VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326), $5, $6, $7)`,
			wp.ID, rt.ID, wp.Lon, wp.Lat, wp.Name, string(wp.Type), wp.SortOrder,
		)
		if err != nil {
			return fmt.Errorf("insert waypoint: %w", err)
		}
	}

	for _, seg := range rt.Segments {
		segWKT := lineStringZToWKT(seg.Geometry)
		_, err = tx.Exec(ctx, `
			INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order)
			VALUES ($1, $2, ST_GeomFromText($3, 4326), $4, $5, $6)`,
			seg.ID, rt.ID, segWKT, string(seg.SurfaceType), seg.GradePercent, seg.SegmentOrder,
		)
		if err != nil {
			return fmt.Errorf("insert segment: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a route by ID, including waypoints, segments, and tags.
func (r *RouteRepository) GetByID(ctx context.Context, id string) (*route.Route, error) {
	rt := &route.Route{}
	var geomJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, ST_AsGeoJSON(geometry)::jsonb, distance_m,
		       elevation_gain_m, elevation_loss_m, difficulty, status, creator_id,
		       created_at, updated_at
		FROM routes.route WHERE id = $1`, id,
	).Scan(
		&rt.ID, &rt.Name, &rt.Description, &geomJSON,
		&rt.DistanceM, &rt.ElevationGainM, &rt.ElevationLossM,
		&rt.Difficulty, &rt.Status, &rt.CreatorID,
		&rt.CreatedAt, &rt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("route not found: %s", id)
		}
		return nil, fmt.Errorf("query route: %w", err)
	}

	rt.Geometry = parseGeoJSONCoords(geomJSON)

	// Load tags
	tagRows, err := r.pool.Query(ctx, `SELECT tag FROM routes.route_tag WHERE route_id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		rt.Tags = append(rt.Tags, tag)
	}

	// Load waypoints
	wpRows, err := r.pool.Query(ctx, `
		SELECT id, name, type, ST_Y(geometry), ST_X(geometry), sort_order
		FROM routes.waypoint WHERE route_id = $1 ORDER BY sort_order`, id)
	if err != nil {
		return nil, fmt.Errorf("query waypoints: %w", err)
	}
	defer wpRows.Close()
	for wpRows.Next() {
		var wp route.Waypoint
		if err := wpRows.Scan(&wp.ID, &wp.Name, &wp.Type, &wp.Lat, &wp.Lon, &wp.SortOrder); err != nil {
			return nil, fmt.Errorf("scan waypoint: %w", err)
		}
		rt.Waypoints = append(rt.Waypoints, wp)
	}

	// Load segments
	segRows, err := r.pool.Query(ctx, `
		SELECT id, ST_AsGeoJSON(geometry)::jsonb, surface_type, grade_percent, segment_order
		FROM routes.route_segment WHERE route_id = $1 ORDER BY segment_order`, id)
	if err != nil {
		return nil, fmt.Errorf("query segments: %w", err)
	}
	defer segRows.Close()
	for segRows.Next() {
		var seg route.Segment
		var segGeomJSON []byte
		if err := segRows.Scan(&seg.ID, &segGeomJSON, &seg.SurfaceType, &seg.GradePercent, &seg.SegmentOrder); err != nil {
			return nil, fmt.Errorf("scan segment: %w", err)
		}
		seg.Geometry = parseGeoJSONCoords(segGeomJSON)
		rt.Segments = append(rt.Segments, seg)
	}

	return rt, nil
}

// Update updates a route's metadata, tags, waypoints, and segments.
func (r *RouteRepository) Update(ctx context.Context, rt *route.Route) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	geomWKT := lineStringZToWKT(rt.Geometry)

	_, err = tx.Exec(ctx, `
		UPDATE routes.route
		SET name = $2, description = $3, geometry = ST_GeomFromText($4, 4326),
		    distance_m = $5, elevation_gain_m = $6, elevation_loss_m = $7,
		    difficulty = $8, status = $9, updated_at = $10
		WHERE id = $1`,
		rt.ID, rt.Name, rt.Description, geomWKT,
		rt.DistanceM, rt.ElevationGainM, rt.ElevationLossM,
		string(rt.Difficulty), string(rt.Status), rt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}

	// Replace tags
	_, _ = tx.Exec(ctx, `DELETE FROM routes.route_tag WHERE route_id = $1`, rt.ID)
	for _, tag := range rt.Tags {
		_, err = tx.Exec(ctx, `INSERT INTO routes.route_tag (route_id, tag) VALUES ($1, $2)`, rt.ID, tag)
		if err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	// Replace waypoints
	_, _ = tx.Exec(ctx, `DELETE FROM routes.waypoint WHERE route_id = $1`, rt.ID)
	for _, wp := range rt.Waypoints {
		_, err = tx.Exec(ctx, `
			INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order)
			VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326), $5, $6, $7)`,
			wp.ID, rt.ID, wp.Lon, wp.Lat, wp.Name, string(wp.Type), wp.SortOrder,
		)
		if err != nil {
			return fmt.Errorf("insert waypoint: %w", err)
		}
	}

	// Replace segments
	_, _ = tx.Exec(ctx, `DELETE FROM routes.route_segment WHERE route_id = $1`, rt.ID)
	for _, seg := range rt.Segments {
		segWKT := lineStringZToWKT(seg.Geometry)
		_, err = tx.Exec(ctx, `
			INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order)
			VALUES ($1, $2, ST_GeomFromText($3, 4326), $4, $5, $6)`,
			seg.ID, rt.ID, segWKT, string(seg.SurfaceType), seg.GradePercent, seg.SegmentOrder,
		)
		if err != nil {
			return fmt.Errorf("insert segment: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// List returns a page of routes matching the given filters.
func (r *RouteRepository) List(ctx context.Context, params route.ListParams) (*route.ListResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []any
	argIdx := 1

	if params.BBox != nil {
		conditions = append(conditions, fmt.Sprintf(
			"geometry && ST_MakeEnvelope($%d, $%d, $%d, $%d, 4326)",
			argIdx, argIdx+1, argIdx+2, argIdx+3,
		))
		args = append(args, params.BBox[0], params.BBox[1], params.BBox[2], params.BBox[3])
		argIdx += 4
	}

	if params.Difficulty != nil {
		conditions = append(conditions, fmt.Sprintf("difficulty = $%d", argIdx))
		args = append(args, string(*params.Difficulty))
		argIdx++
	}

	if params.MinDistM != nil {
		conditions = append(conditions, fmt.Sprintf("distance_m >= $%d", argIdx))
		args = append(args, *params.MinDistM)
		argIdx++
	}

	if params.MaxDistM != nil {
		conditions = append(conditions, fmt.Sprintf("distance_m <= $%d", argIdx))
		args = append(args, *params.MaxDistM)
		argIdx++
	}

	if len(params.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf(
			"id IN (SELECT route_id FROM routes.route_tag WHERE tag = ANY($%d))", argIdx,
		))
		args = append(args, params.Tags)
		argIdx++
	}

	if params.Cursor != "" {
		conditions = append(conditions, fmt.Sprintf("created_at < (SELECT created_at FROM routes.route WHERE id = $%d)", argIdx))
		args = append(args, params.Cursor)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, name, description, distance_m, elevation_gain_m, elevation_loss_m,
		       difficulty, status, creator_id, created_at, updated_at
		FROM routes.route %s
		ORDER BY created_at DESC
		LIMIT $%d`, where, argIdx)
	args = append(args, limit+1) // fetch one extra for cursor

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query routes: %w", err)
	}
	defer rows.Close()

	var routes []*route.Route
	for rows.Next() {
		rt := &route.Route{}
		if err := rows.Scan(
			&rt.ID, &rt.Name, &rt.Description, &rt.DistanceM,
			&rt.ElevationGainM, &rt.ElevationLossM,
			&rt.Difficulty, &rt.Status, &rt.CreatorID,
			&rt.CreatedAt, &rt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, rt)
	}

	result := &route.ListResult{}
	if len(routes) > limit {
		result.Routes = routes[:limit]
		result.NextCursor = routes[limit-1].ID
	} else {
		result.Routes = routes
	}

	return result, nil
}

// Delete hard-deletes a route and all related records.
func (r *RouteRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, _ = tx.Exec(ctx, `DELETE FROM routes.route_tag WHERE route_id = $1`, id)
	_, _ = tx.Exec(ctx, `DELETE FROM routes.waypoint WHERE route_id = $1`, id)
	_, _ = tx.Exec(ctx, `DELETE FROM routes.route_segment WHERE route_id = $1`, id)
	_, err = tx.Exec(ctx, `DELETE FROM routes.route WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}

	return tx.Commit(ctx)
}

// lineStringZToWKT converts [][3]float64 (lon, lat, elev) to WKT LINESTRING Z.
func lineStringZToWKT(coords [][3]float64) string {
	if len(coords) == 0 {
		return "LINESTRING Z EMPTY"
	}
	var sb strings.Builder
	sb.WriteString("LINESTRING Z(")
	for i, c := range coords {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%f %f %f", c[0], c[1], c[2]))
	}
	sb.WriteString(")")
	return sb.String()
}

// parseGeoJSONCoords extracts coordinates from a GeoJSON geometry object.
func parseGeoJSONCoords(geojson []byte) [][3]float64 {
	if len(geojson) == 0 {
		return nil
	}
	var geom struct {
		Coordinates [][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(geojson, &geom); err != nil {
		return nil
	}
	coords := make([][3]float64, len(geom.Coordinates))
	for i, c := range geom.Coordinates {
		if len(c) >= 3 {
			coords[i] = [3]float64{c[0], c[1], c[2]}
		} else if len(c) >= 2 {
			coords[i] = [3]float64{c[0], c[1], 0}
		}
	}
	return coords
}
```

- [ ] 7.3 Verify the package compiles (integration tests need a DB, so just compile-check).

```bash
go build ./internal/infra/postgres/...
```

Expected: no errors.

- [ ] 7.4 Commit.

```bash
git add internal/infra/postgres/
git commit -m "feat: add PostgreSQL route repository with PostGIS geometry support"
```

---

## Task 8 — Application Service for Routes

**Files:**
- `internal/app/route_service.go` (create)
- `internal/app/route_service_test.go` (create)

### Steps

- [ ] 8.1 Write tests first using an in-memory mock repository.

```go
// internal/app/route_service_test.go
package app_test

import (
	"context"
	"errors"
	"testing"

	"komorebi/internal/app"
	"komorebi/internal/domain/route"
)

// mockRouteRepo is a simple in-memory route repository for unit tests.
type mockRouteRepo struct {
	routes map[string]*route.Route
}

func newMockRouteRepo() *mockRouteRepo {
	return &mockRouteRepo{routes: make(map[string]*route.Route)}
}

func (m *mockRouteRepo) Create(_ context.Context, r *route.Route) error {
	m.routes[r.ID] = r
	return nil
}

func (m *mockRouteRepo) GetByID(_ context.Context, id string) (*route.Route, error) {
	r, ok := m.routes[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockRouteRepo) Update(_ context.Context, r *route.Route) error {
	if _, ok := m.routes[r.ID]; !ok {
		return errors.New("not found")
	}
	m.routes[r.ID] = r
	return nil
}

func (m *mockRouteRepo) List(_ context.Context, params route.ListParams) (*route.ListResult, error) {
	var all []*route.Route
	for _, r := range m.routes {
		all = append(all, r)
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return &route.ListResult{Routes: all}, nil
}

func (m *mockRouteRepo) Delete(_ context.Context, id string) error {
	delete(m.routes, id)
	return nil
}

func TestRouteService_Create(t *testing.T) {
	repo := newMockRouteRepo()
	svc := app.NewRouteService(repo)
	ctx := context.Background()

	r, err := svc.CreateRoute(ctx, app.CreateRouteInput{
		Name:        "Test Route",
		Description: "A test",
		Difficulty:  "easy",
		CreatorID:   "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if r.Status != route.StatusDraft {
		t.Errorf("expected draft, got %v", r.Status)
	}
}

func TestRouteService_Get(t *testing.T) {
	repo := newMockRouteRepo()
	svc := app.NewRouteService(repo)
	ctx := context.Background()

	created, _ := svc.CreateRoute(ctx, app.CreateRouteInput{
		Name:       "Test",
		Difficulty: "easy",
		CreatorID:  "user-1",
	})

	got, err := svc.GetRoute(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Test" {
		t.Errorf("expected 'Test', got %q", got.Name)
	}
}

func TestRouteService_Update(t *testing.T) {
	repo := newMockRouteRepo()
	svc := app.NewRouteService(repo)
	ctx := context.Background()

	created, _ := svc.CreateRoute(ctx, app.CreateRouteInput{
		Name:       "Original",
		Difficulty: "easy",
		CreatorID:  "user-1",
	})

	updated, err := svc.UpdateRoute(ctx, created.ID, app.UpdateRouteInput{
		Name:        "Updated",
		Description: "new description",
		Difficulty:  "moderate",
		Tags:        []string{"scenic"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected 'Updated', got %q", updated.Name)
	}
}

func TestRouteService_Archive(t *testing.T) {
	repo := newMockRouteRepo()
	svc := app.NewRouteService(repo)
	ctx := context.Background()

	created, _ := svc.CreateRoute(ctx, app.CreateRouteInput{
		Name:       "To Archive",
		Difficulty: "easy",
		CreatorID:  "user-1",
	})
	// Must publish first to archive
	_ = created.Publish()
	_ = repo.Update(ctx, created)

	err := svc.ArchiveRoute(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := svc.GetRoute(ctx, created.ID)
	if got.Status != route.StatusArchived {
		t.Errorf("expected archived, got %v", got.Status)
	}
}

func TestRouteService_List(t *testing.T) {
	repo := newMockRouteRepo()
	svc := app.NewRouteService(repo)
	ctx := context.Background()

	_, _ = svc.CreateRoute(ctx, app.CreateRouteInput{Name: "A", Difficulty: "easy", CreatorID: "user-1"})
	_, _ = svc.CreateRoute(ctx, app.CreateRouteInput{Name: "B", Difficulty: "hard", CreatorID: "user-1"})

	result, err := svc.ListRoutes(ctx, route.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(result.Routes))
	}
}
```

- [ ] 8.2 Run tests — expect compilation failure.

```bash
go test ./internal/app/...
```

- [ ] 8.3 Implement `route_service.go`.

```go
// internal/app/route_service.go
package app

import (
	"context"
	"fmt"

	"komorebi/internal/domain/route"
)

// CreateRouteInput holds the data needed to create a route.
type CreateRouteInput struct {
	Name        string
	Description string
	Difficulty  string
	CreatorID   string
	Tags        []string
	Geometry    [][3]float64
	DistanceM   float64
	ElevGainM   float64
	ElevLossM   float64
}

// UpdateRouteInput holds the data for updating route metadata.
type UpdateRouteInput struct {
	Name        string
	Description string
	Difficulty  string
	Tags        []string
}

// RouteService orchestrates route use cases.
type RouteService struct {
	repo route.Repository
}

// NewRouteService constructs a RouteService with the given repository.
func NewRouteService(repo route.Repository) *RouteService {
	return &RouteService{repo: repo}
}

// CreateRoute creates a new route in draft status.
func (s *RouteService) CreateRoute(ctx context.Context, input CreateRouteInput) (*route.Route, error) {
	r, err := route.NewRoute(input.Name, input.Description, route.Difficulty(input.Difficulty), input.CreatorID)
	if err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}

	if len(input.Geometry) > 0 {
		r.SetGeometry(input.Geometry, input.DistanceM, input.ElevGainM, input.ElevLossM)
	}
	if len(input.Tags) > 0 {
		r.SetTags(input.Tags)
	}

	if err := s.repo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("persist route: %w", err)
	}
	return r, nil
}

// GetRoute retrieves a route by ID.
func (s *RouteService) GetRoute(ctx context.Context, id string) (*route.Route, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}
	return r, nil
}

// UpdateRoute updates mutable metadata on an existing route.
func (s *RouteService) UpdateRoute(ctx context.Context, id string, input UpdateRouteInput) (*route.Route, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get route for update: %w", err)
	}

	if err := r.UpdateMetadata(input.Name, input.Description, route.Difficulty(input.Difficulty), input.Tags); err != nil {
		return nil, fmt.Errorf("update metadata: %w", err)
	}

	if err := s.repo.Update(ctx, r); err != nil {
		return nil, fmt.Errorf("persist route update: %w", err)
	}
	return r, nil
}

// ListRoutes lists routes matching the given parameters.
func (s *RouteService) ListRoutes(ctx context.Context, params route.ListParams) (*route.ListResult, error) {
	result, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	return result, nil
}

// ArchiveRoute soft-deletes a route (Published -> Archived).
func (s *RouteService) ArchiveRoute(ctx context.Context, id string) error {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get route for archive: %w", err)
	}
	if err := r.Archive(); err != nil {
		return fmt.Errorf("archive route: %w", err)
	}
	if err := s.repo.Update(ctx, r); err != nil {
		return fmt.Errorf("persist archive: %w", err)
	}
	return nil
}
```

- [ ] 8.4 Run tests — expect pass.

```bash
go test ./internal/app/...
```

Expected: all tests pass.

- [ ] 8.5 Commit.

```bash
git add internal/app/
git commit -m "feat: add RouteService application layer with CRUD use cases"
```

---

## Task 9 — HTTP Handlers + Router for Route CRUD

**Files:**
- `internal/api/routes_handler.go` (create)
- `internal/api/router.go` (create)
- `internal/api/response.go` (create)
- `internal/api/routes_handler_test.go` (create)

### Steps

- [ ] 9.1 Write handler tests first.

```go
// internal/api/routes_handler_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/domain/route"
)

type mockRouteRepo struct {
	routes map[string]*route.Route
}

func newMockRepo() *mockRouteRepo {
	return &mockRouteRepo{routes: make(map[string]*route.Route)}
}

func (m *mockRouteRepo) Create(_ context.Context, r *route.Route) error {
	m.routes[r.ID] = r
	return nil
}

func (m *mockRouteRepo) GetByID(_ context.Context, id string) (*route.Route, error) {
	r, ok := m.routes[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockRouteRepo) Update(_ context.Context, r *route.Route) error {
	m.routes[r.ID] = r
	return nil
}

func (m *mockRouteRepo) List(_ context.Context, _ route.ListParams) (*route.ListResult, error) {
	var all []*route.Route
	for _, r := range m.routes {
		all = append(all, r)
	}
	return &route.ListResult{Routes: all}, nil
}

func (m *mockRouteRepo) Delete(_ context.Context, id string) error {
	delete(m.routes, id)
	return nil
}

func setupRouter() (http.Handler, *mockRouteRepo) {
	repo := newMockRepo()
	svc := app.NewRouteService(repo)
	router := api.NewRouter(svc)
	return router, repo
}

func TestCreateRoute_Handler(t *testing.T) {
	router, _ := setupRouter()

	body := `{"name":"Tama River","description":"Scenic path","difficulty":"easy","creator_id":"user-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["id"] == nil || resp["id"] == "" {
		t.Fatal("expected non-empty id in response")
	}
	if resp["status"] != "draft" {
		t.Errorf("expected status 'draft', got %v", resp["status"])
	}
}

func TestGetRoute_Handler(t *testing.T) {
	router, _ := setupRouter()

	// Create a route first
	body := `{"name":"Test","description":"desc","difficulty":"easy","creator_id":"user-1"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created map[string]any
	json.Unmarshal(createRec.Body.Bytes(), &created)
	id := created["id"].(string)

	// Get it
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/routes/"+id, nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestGetRoute_NotFound(t *testing.T) {
	router, _ := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListRoutes_Handler(t *testing.T) {
	router, _ := setupRouter()

	body := `{"name":"Route A","description":"","difficulty":"easy","creator_id":"user-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/routes", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}

	var resp map[string]any
	json.Unmarshal(listRec.Body.Bytes(), &resp)
	routes, ok := resp["routes"].([]any)
	if !ok {
		t.Fatal("expected 'routes' array in response")
	}
	if len(routes) < 1 {
		t.Error("expected at least 1 route")
	}
}

func TestUpdateRoute_Handler(t *testing.T) {
	router, _ := setupRouter()

	// Create
	body := `{"name":"Original","description":"","difficulty":"easy","creator_id":"user-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	id := created["id"].(string)

	// Update
	updateBody := `{"name":"Updated","description":"new","difficulty":"moderate","tags":["scenic"]}`
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/routes/"+id, bytes.NewBufferString(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var updated map[string]any
	json.Unmarshal(updateRec.Body.Bytes(), &updated)
	if updated["name"] != "Updated" {
		t.Errorf("expected 'Updated', got %v", updated["name"])
	}
}

func TestDeleteRoute_Handler(t *testing.T) {
	router, _ := setupRouter()

	// Create and publish so we can archive
	body := `{"name":"To Delete","description":"","difficulty":"easy","creator_id":"user-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	id := created["id"].(string)

	// DELETE on a draft route should fail (needs Published status first)
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/routes/"+id, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)

	// Expect 422 because draft cannot be archived
	if delRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", delRec.Code, delRec.Body.String())
	}
}
```

- [ ] 9.2 Run tests — expect compilation failure.

```bash
go test ./internal/api/...
```

- [ ] 9.3 Implement `response.go` — JSON response helpers.

```go
// internal/api/response.go
package api

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
```

- [ ] 9.4 Implement `routes_handler.go`.

```go
// internal/api/routes_handler.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"komorebi/internal/app"
	"komorebi/internal/domain/route"
)

// RoutesHandler holds HTTP handlers for route endpoints.
type RoutesHandler struct {
	svc *app.RouteService
}

// NewRoutesHandler creates route handlers.
func NewRoutesHandler(svc *app.RouteService) *RoutesHandler {
	return &RoutesHandler{svc: svc}
}

// createRouteRequest is the JSON body for POST /routes.
type createRouteRequest struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Difficulty  string       `json:"difficulty"`
	CreatorID   string       `json:"creator_id"`
	Tags        []string     `json:"tags"`
	Geometry    [][3]float64 `json:"geometry"`
	DistanceM   float64      `json:"distance_m"`
	ElevGainM   float64      `json:"elevation_gain_m"`
	ElevLossM   float64      `json:"elevation_loss_m"`
}

// updateRouteRequest is the JSON body for PATCH /routes/:id.
type updateRouteRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Difficulty  string   `json:"difficulty"`
	Tags        []string `json:"tags"`
}

// routeResponse is the JSON response for a single route.
type routeResponse struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Geometry       [][3]float64       `json:"geometry,omitempty"`
	DistanceM      float64            `json:"distance_m"`
	ElevationGainM float64            `json:"elevation_gain_m"`
	ElevationLossM float64            `json:"elevation_loss_m"`
	Difficulty     string             `json:"difficulty"`
	Status         string             `json:"status"`
	CreatorID      string             `json:"creator_id"`
	Tags           []string           `json:"tags"`
	Waypoints      []waypointResponse `json:"waypoints,omitempty"`
	Segments       []segmentResponse  `json:"segments,omitempty"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

type waypointResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	SortOrder int     `json:"sort_order"`
}

type segmentResponse struct {
	ID           string       `json:"id"`
	Geometry     [][3]float64 `json:"geometry,omitempty"`
	SurfaceType  string       `json:"surface_type"`
	GradePercent float64      `json:"grade_percent"`
	SegmentOrder int          `json:"segment_order"`
}

func toRouteResponse(r *route.Route) routeResponse {
	resp := routeResponse{
		ID:             r.ID,
		Name:           r.Name,
		Description:    r.Description,
		Geometry:       r.Geometry,
		DistanceM:      r.DistanceM,
		ElevationGainM: r.ElevationGainM,
		ElevationLossM: r.ElevationLossM,
		Difficulty:     string(r.Difficulty),
		Status:         string(r.Status),
		CreatorID:      r.CreatorID,
		Tags:           r.Tags,
		CreatedAt:      r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if resp.Tags == nil {
		resp.Tags = []string{}
	}
	for _, wp := range r.Waypoints {
		resp.Waypoints = append(resp.Waypoints, waypointResponse{
			ID: wp.ID, Name: wp.Name, Type: string(wp.Type),
			Lat: wp.Lat, Lon: wp.Lon, SortOrder: wp.SortOrder,
		})
	}
	for _, seg := range r.Segments {
		resp.Segments = append(resp.Segments, segmentResponse{
			ID: seg.ID, Geometry: seg.Geometry, SurfaceType: string(seg.SurfaceType),
			GradePercent: seg.GradePercent, SegmentOrder: seg.SegmentOrder,
		})
	}
	return resp
}

// Create handles POST /api/v1/routes.
func (h *RoutesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	created, err := h.svc.CreateRoute(r.Context(), app.CreateRouteInput{
		Name:        req.Name,
		Description: req.Description,
		Difficulty:  req.Difficulty,
		CreatorID:   req.CreatorID,
		Tags:        req.Tags,
		Geometry:    req.Geometry,
		DistanceM:   req.DistanceM,
		ElevGainM:   req.ElevGainM,
		ElevLossM:   req.ElevLossM,
	})
	if err != nil {
		if strings.Contains(err.Error(), "name must not be empty") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create route")
		return
	}

	writeJSON(w, http.StatusCreated, toRouteResponse(created))
}

// Get handles GET /api/v1/routes/:id.
func (h *RoutesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	rt, err := h.svc.GetRoute(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get route")
		return
	}

	writeJSON(w, http.StatusOK, toRouteResponse(rt))
}

// List handles GET /api/v1/routes.
func (h *RoutesHandler) List(w http.ResponseWriter, r *http.Request) {
	params := route.ListParams{}

	if v := r.URL.Query().Get("limit"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			params.Limit = limit
		}
	}
	if v := r.URL.Query().Get("cursor"); v != "" {
		params.Cursor = v
	}
	if v := r.URL.Query().Get("difficulty"); v != "" {
		d := route.Difficulty(v)
		params.Difficulty = &d
	}
	if v := r.URL.Query().Get("tags"); v != "" {
		params.Tags = strings.Split(v, ",")
	}
	if v := r.URL.Query().Get("bbox"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) == 4 {
			var bbox [4]float64
			valid := true
			for i, p := range parts {
				f, err := strconv.ParseFloat(p, 64)
				if err != nil {
					valid = false
					break
				}
				bbox[i] = f
			}
			if valid {
				params.BBox = &bbox
			}
		}
	}

	result, err := h.svc.ListRoutes(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list routes")
		return
	}

	var routes []routeResponse
	for _, rt := range result.Routes {
		routes = append(routes, toRouteResponse(rt))
	}
	if routes == nil {
		routes = []routeResponse{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"routes":      routes,
		"next_cursor": result.NextCursor,
	})
}

// Update handles PATCH /api/v1/routes/:id.
func (h *RoutesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	updated, err := h.svc.UpdateRoute(r.Context(), id, app.UpdateRouteInput{
		Name:        req.Name,
		Description: req.Description,
		Difficulty:  req.Difficulty,
		Tags:        req.Tags,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update route")
		return
	}

	writeJSON(w, http.StatusOK, toRouteResponse(updated))
}

// Delete handles DELETE /api/v1/routes/:id — archives the route (soft delete).
func (h *RoutesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := h.svc.ArchiveRoute(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		if strings.Contains(err.Error(), "invalid status transition") {
			writeError(w, http.StatusUnprocessableEntity, "route cannot be archived from current status")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to archive route")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] 9.5 Implement `router.go`.

```go
// internal/api/router.go
package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"komorebi/internal/app"
)

// NewRouter builds the chi router with all routes mounted.
func NewRouter(routeSvc *app.RouteService) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	routeHandler := NewRoutesHandler(routeSvc)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/routes", func(r chi.Router) {
			r.Get("/", routeHandler.List)
			r.Post("/", routeHandler.Create)
			r.Get("/{id}", routeHandler.Get)
			r.Patch("/{id}", routeHandler.Update)
			r.Delete("/{id}", routeHandler.Delete)
		})
	})

	return r
}
```

- [ ] 9.6 Run tests — expect pass.

```bash
go test ./internal/api/...
```

Expected: all tests pass.

- [ ] 9.7 Commit.

```bash
git add internal/api/
git commit -m "feat: add HTTP handlers and chi router for Route CRUD endpoints"
```

---

## Task 10 — main.go Wiring

**Files:**
- `cmd/api/main.go` (modify — replace placeholder)

### Steps

- [ ] 10.1 Implement `main.go` with config, pool setup, and graceful shutdown.

```go
// cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/infra/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	// Config from environment
	port := envOrDefault("PORT", "8080")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://localhost:5432/cyclist_map?sslmode=disable")

	// Database pool
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	log.Println("connected to database")

	// Repositories
	routeRepo := postgres.NewRouteRepository(pool)

	// Application services
	routeSvc := app.NewRouteService(routeRepo)

	// HTTP router
	router := api.NewRouter(routeSvc)

	// Server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", port)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %v, shutting down...", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
```

- [ ] 10.2 Verify the project compiles end-to-end.

```bash
go build ./cmd/api/...
```

Expected: no errors.

- [ ] 10.3 Commit.

```bash
git add cmd/api/main.go
git commit -m "feat: wire main.go with database pool, services, and graceful shutdown"
```

---

## Task 11 — docker-compose.yml for Development

**Files:**
- `docker-compose.yml` (create)
- `Dockerfile` (create)

PostGIS is external (already running), so it is NOT included in docker-compose. The Go API, Valhalla, and Martin are containerized.

### Steps

- [ ] 11.1 Create the `Dockerfile` for the Go API.

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cyclist-map-api ./cmd/api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /cyclist-map-api /usr/local/bin/cyclist-map-api
EXPOSE 8080
ENTRYPOINT ["cyclist-map-api"]
```

- [ ] 11.2 Create `docker-compose.yml`.

```yaml
# docker-compose.yml
version: "3.8"

services:
  api:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      PORT: "8080"
      DATABASE_URL: "postgres://cyclist:cyclist@host.docker.internal:5432/cyclist_map?sslmode=disable"
      VALHALLA_URL: "http://valhalla:8002"
    depends_on:
      - valhalla
      - martin
    restart: unless-stopped

  valhalla:
    image: ghcr.io/gis-ops/docker-valhalla/valhalla:latest
    ports:
      - "8002:8002"
    volumes:
      - valhalla-data:/custom_files
    environment:
      tile_urls: "https://download.geofabrik.de/asia/japan/kanto-latest.osm.pbf"
      use_tiles_ignore_pbf: "False"
      build_elevation: "True"
      build_admins: "True"
      force_rebuild: "False"
    restart: unless-stopped

  martin:
    image: ghcr.io/maplibre/martin:latest
    ports:
      - "3000:3000"
    environment:
      DATABASE_URL: "postgres://cyclist:cyclist@host.docker.internal:5432/cyclist_map?sslmode=disable"
    restart: unless-stopped

volumes:
  valhalla-data:
```

- [ ] 11.3 Verify docker-compose config is valid.

```bash
docker compose config --quiet
```

Expected: no errors (exit code 0).

- [ ] 11.4 Commit.

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: add Dockerfile and docker-compose for dev (API, Valhalla, Martin)"
```

---

## Verification Checklist

After all tasks are complete, verify the following:

- [ ] `go build ./...` compiles with zero errors
- [ ] `go test ./...` passes all unit tests
- [ ] `go vet ./...` reports no issues
- [ ] All six Route CRUD endpoints from the spec are covered:
  - `GET /api/v1/routes` (list/search with bbox, difficulty, tags, cursor pagination)
  - `GET /api/v1/routes/:id` (single route with waypoints, segments)
  - `POST /api/v1/routes` (create)
  - `PATCH /api/v1/routes/:id` (update metadata)
  - `DELETE /api/v1/routes/:id` (archive / soft delete)
  - `GET /api/v1/routes/:id/conditions` (not in this skeleton — requires Environment context wiring, tracked as a follow-up)
- [ ] Domain types match the design spec for all five bounded contexts (Route, Plan, Community, Environment, Events)
- [ ] Repository interfaces are defined for Route, Plan, Community contexts
- [ ] Speed model matches spec: base 15 km/h, uphill -1.5x grade clamped at 4, downhill +1.0x grade capped at 35, signal penalty 30s, green wave override
- [ ] docker-compose.yml includes API, Valhalla, Martin (PostGIS is external)
- [ ] Zero placeholder text ("TBD", "implement later", "TODO") in any code
