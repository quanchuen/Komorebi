package app

import (
	"log"

	"komorebi/internal/domain/route"
	"komorebi/internal/infra/valhalla"
)

// ElevationSampler provides elevation lookups for route geometry. Implemented
// by the Valhalla client (/height endpoint).
type ElevationSampler interface {
	// Heights returns one elevation per input point, aligned with shape.
	Heights(shape [][2]float64) ([]float64, error)
	// ElevationProfile returns a distance/elevation profile resampled every
	// 100 m — the same sampling directions use for gain/loss, which keeps the
	// numbers comparable and suppresses per-point DEM noise.
	ElevationProfile(shape [][2]float64) ([]valhalla.ElevationPoint, error)
}

// backfillElevation fills in elevation for routes stored without any — z on
// every geometry coordinate plus gain/loss — using the same Valhalla height
// data a self-generated route gets at creation. The enriched elevation is
// persisted (elevation columns only — never the user-authored fields, so a
// read can't race a concurrent edit) so the lookup happens once per route.
// Best-effort: on any error the route is served as stored.
func (s *RouteService) backfillElevation(rt *route.Route) {
	if s.elevations == nil || len(rt.Geometry) < 2 || hasElevation(rt.Geometry) {
		return
	}

	shape := make([][2]float64, len(rt.Geometry))
	for i, c := range rt.Geometry {
		shape[i] = [2]float64{c[0], c[1]}
	}

	heights, err := s.elevations.Heights(shape)
	if err != nil {
		log.Printf("route %s: elevation backfill unavailable: %v", rt.ID, err)
		return
	}
	allZero := true
	for _, h := range heights {
		if h != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return // no elevation coverage for this area; don't persist zeros
	}

	geom := make([][3]float64, len(rt.Geometry))
	for i, c := range rt.Geometry {
		geom[i] = [3]float64{c[0], c[1], heights[i]}
	}

	gain, loss := rt.ElevationGainM, rt.ElevationLossM
	if profile, err := s.elevations.ElevationProfile(shape); err == nil && len(profile) > 1 {
		gain, loss = 0, 0
		for i := 1; i < len(profile); i++ {
			delta := profile[i].ElevationM - profile[i-1].ElevationM
			if delta > 0 {
				gain += delta
			} else {
				loss -= delta
			}
		}
	}

	rt.Geometry = geom
	rt.ElevationGainM = gain
	rt.ElevationLossM = loss
	if err := s.repo.UpdateElevation(rt.ID, geom, gain, loss); err != nil {
		log.Printf("route %s: persisting backfilled elevation: %v", rt.ID, err)
	}
}

// hasElevation reports whether any coordinate carries a non-zero z value.
func hasElevation(coords [][3]float64) bool {
	for _, c := range coords {
		if c[2] != 0 {
			return true
		}
	}
	return false
}
