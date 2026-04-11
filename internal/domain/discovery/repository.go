// internal/domain/discovery/repository.go
package discovery

// Repository defines read-only spatial queries for route discovery.
// The implementation lives in internal/infra/postgres.
type Repository interface {
	// Nearby returns published routes whose geometry falls within RadiusKm of
	// the given point, ordered by ascending distance.
	Nearby(params NearbyParams) ([]DiscoveryResult, error)

	// Viewport returns published routes whose geometry intersects the given
	// bounding box, ordered by route name.
	Viewport(params ViewportParams) ([]DiscoveryResult, error)

	// Suggested returns candidate routes for the suggested endpoint.
	// Phase 2: returns Nearby results ordered by distance.
	// Phase 3: will incorporate environment scoring at departure_at.
	Suggested(params SuggestedParams) ([]DiscoveryResult, error)
}
