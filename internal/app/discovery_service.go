// internal/app/discovery_service.go
package app

import (
	"github.com/cyclist-map/cyclist-map/internal/domain/discovery"
)

// DiscoveryService provides application-level route discovery use cases.
type DiscoveryService struct {
	repo discovery.Repository
}

// NewDiscoveryService creates a DiscoveryService backed by the given repository.
func NewDiscoveryService(repo discovery.Repository) *DiscoveryService {
	return &DiscoveryService{repo: repo}
}

// Nearby returns published routes near the given point.
// RadiusKm defaults to 10 km if zero; Limit defaults to 20 if zero.
func (s *DiscoveryService) Nearby(params discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	if params.RadiusKm <= 0 {
		params.RadiusKm = 10
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	return s.repo.Nearby(params)
}

// Viewport returns published routes intersecting the given bounding box.
// Limit defaults to 50 if zero.
func (s *DiscoveryService) Viewport(params discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	return s.repo.Viewport(params)
}

// Suggested returns recommended routes for a location and departure time.
// Phase 2: proximity-ordered. Phase 3: environment-scored.
// Limit defaults to 10 if zero.
func (s *DiscoveryService) Suggested(params discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}
	return s.repo.Suggested(params)
}
