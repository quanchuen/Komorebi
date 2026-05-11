// internal/domain/environment/venue_test.go
package environment_test

import (
	"testing"

	"komorebi/internal/domain/environment"
)

func TestAlongRouteParams_Defaults(t *testing.T) {
	p := environment.AlongRouteParams{RouteID: "abc"}
	if p.BufferM != 0 {
		t.Fatalf("expected zero default buffer, got %f", p.BufferM)
	}
}

func TestVenue_Fields(t *testing.T) {
	v := environment.Venue{
		ID:       "v1",
		Name:     "7-Eleven Shinjuku",
		Category: "convenience",
		Lat:      35.69,
		Lon:      139.70,
	}
	if v.ID == "" {
		t.Fatal("ID must not be empty")
	}
}

func TestVenueTag_Fields(t *testing.T) {
	vt := environment.VenueTag{
		Hashtag:     "#konbini",
		Description: "Any convenience store",
		IsBrand:     false,
	}
	if vt.Hashtag == "" {
		t.Fatal("Hashtag must not be empty")
	}
}
