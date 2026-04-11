// internal/app/venue_service_test.go
package app_test

import (
	"errors"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

// stubVenueRepo implements environment.VenueRepository for tests.
type stubVenueRepo struct {
	venues []environment.Venue
	tags   []environment.VenueTag
	err    error
}

func (s *stubVenueRepo) AlongRoute(_ environment.AlongRouteParams) ([]environment.Venue, error) {
	return s.venues, s.err
}
func (s *stubVenueRepo) ListTags() ([]environment.VenueTag, error) {
	return s.tags, s.err
}
func (s *stubVenueRepo) GetTagMapping(_ string) (*environment.VenueTagMapping, error) {
	return nil, s.err
}
func (s *stubVenueRepo) NearestAlongLine(_ environment.NearestAlongLineParams) (*environment.Venue, error) {
	return nil, s.err
}

func TestVenueService_AlongRoute_DefaultBuffer(t *testing.T) {
	stub := &stubVenueRepo{
		venues: []environment.Venue{
			{ID: "v1", Name: "7-Eleven", Category: "convenience", Lat: 35.69, Lon: 139.70},
		},
	}
	svc := app.NewVenueService(stub)

	venues, err := svc.AlongRoute(environment.AlongRouteParams{RouteID: "r1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(venues) != 1 {
		t.Fatalf("expected 1 venue, got %d", len(venues))
	}
}

func TestVenueService_AlongRoute_PropagatesError(t *testing.T) {
	stub := &stubVenueRepo{err: errors.New("db down")}
	svc := app.NewVenueService(stub)

	_, err := svc.AlongRoute(environment.AlongRouteParams{RouteID: "r1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestVenueService_ListTags(t *testing.T) {
	stub := &stubVenueRepo{
		tags: []environment.VenueTag{
			{Hashtag: "#konbini", Description: "Convenience store", IsBrand: false},
		},
	}
	svc := app.NewVenueService(stub)

	tags, err := svc.ListTags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Hashtag != "#konbini" {
		t.Fatalf("expected #konbini, got %s", tags[0].Hashtag)
	}
}
