// internal/app/venue_resolution_service.go
package app

import (
	"fmt"
	"strings"

	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
)

// VenueResolver is the interface the resolution service uses to find venues.
// Satisfied by *postgres.VenueRepo.
type VenueResolver interface {
	ListTags() ([]environment.VenueTag, error)
	NearestAlongLine(params environment.NearestAlongLineParams) (*environment.Venue, error)
}

// VenueTagLookup is the interface for fetching full VenueTagMapping records (including OSMFilter).
// Separate from VenueResolver because ListTags only returns VenueTag (no OSMFilter).
type VenueTagLookup interface {
	GetTagMapping(hashtag string) (*environment.VenueTagMapping, error)
}

// VenueResolutionService resolves hashtags in PlanTask descriptions to real Venue matches.
type VenueResolutionService struct {
	resolver VenueResolver
	lookup   VenueTagLookup
}

// NewVenueResolutionService creates a VenueResolutionService.
func NewVenueResolutionService(resolver VenueResolver, lookup VenueTagLookup) *VenueResolutionService {
	return &VenueResolutionService{resolver: resolver, lookup: lookup}
}

// ResolveTask attempts to match the task's hashtag against a venue along the route.
// routeWKT is the current route's LINESTRING WKT (SRID 4326).
// Returns the updated task (status matched with resolved venue, or unchanged if no match).
// Resolution failures are non-fatal: if no venue is found the task stays unresolved.
func (s *VenueResolutionService) ResolveTask(t plan.PlanTask, routeWKT string) (plan.PlanTask, error) {
	hashtag := extractHashtag(t.Description)
	if hashtag == "" && t.Hashtag == "" {
		// No hashtag in description and no explicit hashtag field — nothing to resolve.
		return t, nil
	}
	if hashtag == "" {
		hashtag = t.Hashtag
	}
	t.Hashtag = hashtag

	mapping, err := s.lookup.GetTagMapping(hashtag)
	if err != nil {
		return t, fmt.Errorf("GetTagMapping %q: %w", hashtag, err)
	}
	if mapping == nil {
		// Unknown hashtag — leave unresolved for user refinement.
		return t, nil
	}

	venue, err := s.resolver.NearestAlongLine(environment.NearestAlongLineParams{
		RouteWKT:  routeWKT,
		OSMFilter: mapping.OSMFilter,
		IsBrand:   mapping.IsBrand,
		BufferM:   200,
	})
	if err != nil {
		return t, fmt.Errorf("NearestAlongLine: %w", err)
	}
	if venue == nil {
		// No venue found within corridor — stay unresolved.
		return t, nil
	}

	t.Status = plan.TaskMatched
	t.ResolvedVenueID = venue.ID
	return t, nil
}

// extractHashtag returns the first #word token found in s, or "".
func extractHashtag(s string) string {
	for _, word := range strings.Fields(s) {
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			// Strip trailing punctuation.
			word = strings.TrimRight(word, ".,;:!?")
			return word
		}
	}
	return ""
}
