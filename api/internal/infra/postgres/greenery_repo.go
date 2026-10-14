// internal/infra/postgres/greenery_repo.go
package postgres

import (
	"context"
	"fmt"

	"komorebi/internal/domain/environment"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GreeneryRepo implements environment.GreeneryRepository using PostGIS.
type GreeneryRepo struct {
	pool *pgxpool.Pool
}

// NewGreeneryRepo creates a new GreeneryRepo.
func NewGreeneryRepo(pool *pgxpool.Pool) *GreeneryRepo {
	return &GreeneryRepo{pool: pool}
}

// ScoreAlongRoute returns the average greenery_score for OSM way edges that
// intersect the named route geometry within a planar-degree buffer.
//
// Join path:
//
//	routes.route  →  (ST_DWithin buffer)  →  osm.roads  →  environment.greenery_edge
//
// We use a planar ST_DWithin on osm.roads.geom (not ::geography) to stay on
// the GiST index (roads_geom_idx). At Tokyo latitude (~35°N) 0.00009° ≈ 10 m,
// so the default 10 m corridor captures only edges that the route actually
// travels along rather than parallel streets.
func (r *GreeneryRepo) ScoreAlongRoute(params environment.RouteGreeneryParams) (environment.RouteGreeneryResult, error) {
	bufDeg := params.BufferDeg
	if bufDeg <= 0 {
		bufDeg = 0.00009 // ~10 m
	}

	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(ge.greenery_score), 0.0) AS avg_score,
			COUNT(ge.osm_way_id)                  AS edge_count
		FROM routes.route rt
		JOIN osm.roads road ON ST_DWithin(road.geom, rt.geometry, $2)
		JOIN environment.greenery_edge ge ON ge.osm_way_id = road.way_id
		WHERE rt.id = $1::uuid
	`, params.RouteID, bufDeg)

	var res environment.RouteGreeneryResult
	if err := row.Scan(&res.AvgScore, &res.EdgeCount); err != nil {
		return environment.RouteGreeneryResult{}, fmt.Errorf("greenery.ScoreAlongRoute: %w", err)
	}
	return res, nil
}
