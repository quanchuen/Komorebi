// internal/app/venue_service.go
package app

import (
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
)

// VenueService provides application-level venue discovery use cases.
type VenueService struct {
	repo environment.VenueRepository
}

// NewVenueService creates a VenueService backed by the given repository.
func NewVenueService(repo environment.VenueRepository) *VenueService {
	return &VenueService{repo: repo}
}

// AlongRoute returns venues within BufferM metres of the named route.
// BufferM defaults to 200 m if zero.
func (s *VenueService) AlongRoute(params environment.AlongRouteParams) ([]environment.Venue, error) {
	if params.BufferM <= 0 {
		params.BufferM = 200
	}
	return s.repo.AlongRoute(params)
}

// ListTags returns all venue hashtag definitions.
func (s *VenueService) ListTags() ([]environment.VenueTag, error) {
	return s.repo.ListTags()
}
