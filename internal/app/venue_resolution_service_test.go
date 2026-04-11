// internal/app/venue_resolution_service_test.go
package app_test

import (
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
)

// --- stubs ---

type stubTagLookup struct {
	mapping *environment.VenueTagMapping
	err     error
}

func (s *stubTagLookup) GetTagMapping(_ string) (*environment.VenueTagMapping, error) {
	return s.mapping, s.err
}

type stubVenueNearby struct {
	venue *environment.Venue
	err   error
}

func (s *stubVenueNearby) ListTags() ([]environment.VenueTag, error) { return nil, nil }
func (s *stubVenueNearby) NearestAlongLine(_ environment.NearestAlongLineParams) (*environment.Venue, error) {
	return s.venue, s.err
}

// --- tests ---

func TestExtractHashtag_Found(t *testing.T) {
	task := plan.PlanTask{Description: "Buy coffee at #cafe today"}
	svc := app.NewVenueResolutionService(&stubVenueNearby{}, &stubTagLookup{})
	resolved, err := svc.ResolveTask(task, "LINESTRING(0 0, 1 1)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Hashtag != "#cafe" {
		t.Errorf("expected hashtag #cafe, got %q", resolved.Hashtag)
	}
}

func TestResolveTask_NoHashtag_StaysUnresolved(t *testing.T) {
	task := plan.PlanTask{Description: "No hashtag here", Status: plan.TaskUnresolved}
	svc := app.NewVenueResolutionService(&stubVenueNearby{}, &stubTagLookup{})
	resolved, err := svc.ResolveTask(task, "LINESTRING(0 0, 1 1)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved, got %q", resolved.Status)
	}
}

func TestResolveTask_UnknownHashtag_StaysUnresolved(t *testing.T) {
	task := plan.PlanTask{Description: "Buy #unknownthing", Status: plan.TaskUnresolved}
	lookup := &stubTagLookup{mapping: nil} // not found
	svc := app.NewVenueResolutionService(&stubVenueNearby{}, lookup)
	resolved, err := svc.ResolveTask(task, "LINESTRING(0 0, 1 1)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved, got %q", resolved.Status)
	}
}

func TestResolveTask_VenueFound_Matched(t *testing.T) {
	task := plan.PlanTask{Description: "Grab a snack #konbini", Status: plan.TaskUnresolved}
	mapping := &environment.VenueTagMapping{
		Hashtag:   "#konbini",
		OSMFilter: map[string]string{"shop": "convenience"},
	}
	venue := &environment.Venue{ID: "venue-42", Name: "7-Eleven"}
	svc := app.NewVenueResolutionService(
		&stubVenueNearby{venue: venue},
		&stubTagLookup{mapping: mapping},
	)
	resolved, err := svc.ResolveTask(task, "LINESTRING(139 35, 139.1 35.1)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != plan.TaskMatched {
		t.Errorf("expected matched, got %q", resolved.Status)
	}
	if resolved.ResolvedVenueID != "venue-42" {
		t.Errorf("expected venue-42, got %q", resolved.ResolvedVenueID)
	}
}

func TestResolveTask_NoVenueNearby_StaysUnresolved(t *testing.T) {
	task := plan.PlanTask{Description: "#cafe stop", Status: plan.TaskUnresolved}
	mapping := &environment.VenueTagMapping{
		Hashtag:   "#cafe",
		OSMFilter: map[string]string{"amenity": "cafe"},
	}
	svc := app.NewVenueResolutionService(
		&stubVenueNearby{venue: nil}, // no venue found
		&stubTagLookup{mapping: mapping},
	)
	resolved, err := svc.ResolveTask(task, "LINESTRING(139 35, 139.1 35.1)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved, got %q", resolved.Status)
	}
}

func TestResolveTask_ExplicitHashtagField(t *testing.T) {
	// No hashtag in description, but Hashtag field is set
	task := plan.PlanTask{Description: "Buy something", Hashtag: "#pharmacy"}
	mapping := &environment.VenueTagMapping{
		Hashtag:   "#pharmacy",
		OSMFilter: map[string]string{"amenity": "pharmacy"},
	}
	venue := &environment.Venue{ID: "pharm-1", Name: "Pharmacy"}
	svc := app.NewVenueResolutionService(
		&stubVenueNearby{venue: venue},
		&stubTagLookup{mapping: mapping},
	)
	resolved, err := svc.ResolveTask(task, "LINESTRING(0 0, 1 1)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != plan.TaskMatched {
		t.Errorf("expected matched, got %q", resolved.Status)
	}
}
