// internal/api/venue_handler_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/go-chi/chi/v5"
)

// stubVenueRepoHTTP implements environment.VenueRepository for handler tests.
type stubVenueRepoHTTP struct {
	venues []environment.Venue
	tags   []environment.VenueTag
}

func (s *stubVenueRepoHTTP) AlongRoute(_ environment.AlongRouteParams) ([]environment.Venue, error) {
	return s.venues, nil
}
func (s *stubVenueRepoHTTP) ListTags() ([]environment.VenueTag, error) {
	return s.tags, nil
}
func (s *stubVenueRepoHTTP) GetTagMapping(_ string) (*environment.VenueTagMapping, error) {
	return nil, nil
}
func (s *stubVenueRepoHTTP) NearestAlongLine(_ environment.NearestAlongLineParams) (*environment.Venue, error) {
	return nil, nil
}

func newTestVenueSvc(venues []environment.Venue, tags []environment.VenueTag) *app.VenueService {
	return app.NewVenueService(&stubVenueRepoHTTP{venues: venues, tags: tags})
}

func TestVenueHandler_AlongRoute_OK(t *testing.T) {
	venues := []environment.Venue{
		{ID: "v1", Name: "7-Eleven", Category: "convenience", Lat: 35.69, Lon: 139.70, OsmTags: map[string]string{}},
	}
	svc := newTestVenueSvc(venues, nil)
	h := api.NewVenueHandler(svc)

	// Wire into chi router to populate URL params
	r := chi.NewRouter()
	r.Get("/api/v1/venues/along-route", h.AlongRoute)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/venues/along-route?route_id=00000000-0000-0000-0000-000000000001&buffer_m=300", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	venues2, ok := resp["venues"].([]any)
	if !ok || len(venues2) != 1 {
		t.Fatalf("expected 1 venue, got %v", resp)
	}
}

func TestVenueHandler_AlongRoute_MissingRouteID(t *testing.T) {
	svc := newTestVenueSvc(nil, nil)
	h := api.NewVenueHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/venues/along-route", nil)
	rr := httptest.NewRecorder()
	h.AlongRoute(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestVenueHandler_Tags_OK(t *testing.T) {
	tags := []environment.VenueTag{
		{Hashtag: "#konbini", Description: "Convenience store", IsBrand: false},
		{Hashtag: "#7-eleven", Description: "7-Eleven brand", IsBrand: true},
	}
	svc := newTestVenueSvc(nil, tags)
	h := api.NewVenueHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/venues/tags", nil)
	rr := httptest.NewRecorder()
	h.Tags(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tagsOut, ok := resp["tags"].([]any)
	if !ok || len(tagsOut) != 2 {
		t.Fatalf("expected 2 tags, got %v", resp)
	}
}
