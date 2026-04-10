package route_test

import (
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/route"
)

func TestNewRoute_Valid(t *testing.T) {
	r, err := route.NewRoute("Mt. Fuji Loop", "Scenic loop around Fuji", route.DifficultyHard, "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r.ID == "" {
		t.Error("expected non-empty ID")
	}
	if r.Name != "Mt. Fuji Loop" {
		t.Errorf("expected name 'Mt. Fuji Loop', got %q", r.Name)
	}
	if r.Status != route.StatusDraft {
		t.Errorf("expected Draft status, got %v", r.Status)
	}
	if r.CreatorID != "user-1" {
		t.Errorf("expected creatorID 'user-1', got %q", r.CreatorID)
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewRoute_EmptyName(t *testing.T) {
	_, err := route.NewRoute("", "desc", route.DifficultyEasy, "user-1")
	if err != route.ErrEmptyName {
		t.Errorf("expected ErrEmptyName, got %v", err)
	}
}

func TestRoute_PublishFromDraft(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	if err := r.Publish(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r.Status != route.StatusPublished {
		t.Errorf("expected Published, got %v", r.Status)
	}
}

func TestRoute_PublishTwiceError(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	_ = r.Publish()
	if err := r.Publish(); err != route.ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestRoute_ArchiveFromPublished(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	_ = r.Publish()
	if err := r.Archive(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r.Status != route.StatusArchived {
		t.Errorf("expected Archived, got %v", r.Status)
	}
}

func TestRoute_ArchiveFromDraftError(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	if err := r.Archive(); err != route.ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestRoute_AddWaypoint(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	wp := route.Waypoint{
		ID:        "wp-1",
		Name:      "Summit View",
		Type:      route.WaypointViewpoint,
		Lat:       35.3606,
		Lon:       138.7274,
		SortOrder: 0,
	}
	r.AddWaypoint(wp)
	if len(r.Waypoints) != 1 {
		t.Errorf("expected 1 waypoint, got %d", len(r.Waypoints))
	}
	if r.Waypoints[0].Name != "Summit View" {
		t.Errorf("unexpected waypoint name: %q", r.Waypoints[0].Name)
	}
}

func TestRoute_AddSegment(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	seg := route.Segment{
		ID:           "seg-1",
		SurfaceType:  route.SurfaceGravel,
		GradePercent: 3.5,
		SegmentOrder: 0,
	}
	r.AddSegment(seg)
	if len(r.Segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(r.Segments))
	}
}

func TestRoute_SetTags(t *testing.T) {
	r, _ := route.NewRoute("Test Route", "desc", route.DifficultyEasy, "user-1")
	before := r.UpdatedAt
	time.Sleep(time.Millisecond)
	r.SetTags([]string{"scenic", "mountain"})
	if len(r.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(r.Tags))
	}
	if !r.UpdatedAt.After(before) {
		t.Error("expected UpdatedAt to advance after SetTags")
	}
}
