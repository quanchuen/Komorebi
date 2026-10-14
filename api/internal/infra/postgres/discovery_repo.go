// internal/infra/postgres/discovery_repo.go
package postgres

import (
	"context"
	"fmt"

	"komorebi/internal/domain/discovery"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DiscoveryRepo implements discovery.Repository using PostGIS spatial queries.
type DiscoveryRepo struct {
	pool *pgxpool.Pool
}

// NewDiscoveryRepo creates a new DiscoveryRepo.
func NewDiscoveryRepo(pool *pgxpool.Pool) *DiscoveryRepo {
	return &DiscoveryRepo{pool: pool}
}

// Nearby returns published routes within RadiusKm of (Lat, Lon), ordered by
// ascending distance. Uses ST_DWithin for index-friendly filtering and
// ST_Distance for ordering.
func (r *DiscoveryRepo) Nearby(params discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	radiusKm := params.RadiusKm
	if radiusKm <= 0 {
		radiusKm = 10
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT
			r.id,
			r.name,
			r.description,
			r.distance_m,
			r.elevation_gain_m,
			r.elevation_loss_m,
			r.difficulty::text,
			r.status::text,
			r.featured,
			ST_Distance(
				r.geometry::geography,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
			) AS dist_m
		FROM routes.route r
		WHERE r.status = 'published'
		  AND NOT EXISTS (
				SELECT 1 FROM routes.route_tag rt
				WHERE rt.route_id = r.id AND rt.tag = 'demo'
		  )
		  AND ST_DWithin(
				r.geometry::geography,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography,
				$3
			)
		ORDER BY dist_m ASC
		LIMIT $4
	`, params.Lat, params.Lon, radiusKm*1000, limit)
	if err != nil {
		return nil, fmt.Errorf("discovery.Nearby query: %w", err)
	}
	defer rows.Close()

	results, err := scanDiscoveryRows(rows)
	if err != nil {
		return nil, err
	}

	// Load tags for each result
	for i := range results {
		tags, err := loadTagsForRoute(ctx, r.pool, results[i].RouteID)
		if err == nil {
			results[i].Tags = tags
		}
	}

	return results, nil
}

// Viewport returns published routes whose geometry intersects the given
// bounding box [minLon, minLat, maxLon, maxLat], ordered by route name.
// Uses ST_Intersects + ST_MakeEnvelope for GiST index utilisation.
func (r *DiscoveryRepo) Viewport(params discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	limit := params.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT
			r.id,
			r.name,
			r.description,
			r.distance_m,
			r.elevation_gain_m,
			r.elevation_loss_m,
			r.difficulty::text,
			r.status::text,
			r.featured,
			0.0 AS dist_m
		FROM routes.route r
		WHERE r.status = 'published'
		  AND NOT EXISTS (
				SELECT 1 FROM routes.route_tag rt
				WHERE rt.route_id = r.id AND rt.tag = 'demo'
		  )
		  AND ST_Intersects(
				r.geometry,
				ST_MakeEnvelope($1, $2, $3, $4, 4326)
			)
		ORDER BY r.name ASC
		LIMIT $5
	`, params.BBox[0], params.BBox[1], params.BBox[2], params.BBox[3], limit)
	if err != nil {
		return nil, fmt.Errorf("discovery.Viewport query: %w", err)
	}
	defer rows.Close()

	results, err := scanDiscoveryRows(rows)
	if err != nil {
		return nil, err
	}

	for i := range results {
		tags, err := loadTagsForRoute(ctx, r.pool, results[i].RouteID)
		if err == nil {
			results[i].Tags = tags
		}
	}

	return results, nil
}

// Suggested returns the featured (house-pick) routes, nearest first. Unlike
// Nearby there is no radius: the set is the same wherever the map is, so the
// resting view's "Suggested routes" list never changes as the rider pans.
// Environment scoring at departure_at is deferred to Phase 3.
func (r *DiscoveryRepo) Suggested(params discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT
			r.id,
			r.name,
			r.description,
			r.distance_m,
			r.elevation_gain_m,
			r.elevation_loss_m,
			r.difficulty::text,
			r.status::text,
			r.featured,
			ST_Distance(
				r.geometry::geography,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
			) AS dist_m
		FROM routes.route r
		WHERE r.featured
		  AND r.status = 'published'
		  AND NOT EXISTS (
				SELECT 1 FROM routes.route_tag rt
				WHERE rt.route_id = r.id AND rt.tag = 'demo'
		  )
		ORDER BY dist_m ASC, r.name ASC
		LIMIT $3
	`, params.Lat, params.Lon, limit)
	if err != nil {
		return nil, fmt.Errorf("discovery.Suggested query: %w", err)
	}
	defer rows.Close()

	results, err := scanDiscoveryRows(rows)
	if err != nil {
		return nil, err
	}

	for i := range results {
		tags, err := loadTagsForRoute(ctx, r.pool, results[i].RouteID)
		if err == nil {
			results[i].Tags = tags
		}
	}

	return results, nil
}

// --- helpers ---

// scanDiscoveryRows reads rows from a discovery SELECT and returns DiscoveryResult slice.
// Expected column order: id, name, description, distance_m, elevation_gain_m,
// elevation_loss_m, difficulty, status, featured, dist_m.
func scanDiscoveryRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]discovery.DiscoveryResult, error) {
	var results []discovery.DiscoveryResult
	for rows.Next() {
		var dr discovery.DiscoveryResult
		if err := rows.Scan(
			&dr.RouteID,
			&dr.Name,
			&dr.Description,
			&dr.DistanceM,
			&dr.ElevationGainM,
			&dr.ElevationLossM,
			&dr.Difficulty,
			&dr.Status,
			&dr.Featured,
			&dr.DistFromM,
		); err != nil {
			return nil, fmt.Errorf("scan discovery row: %w", err)
		}
		dr.Tags = []string{}
		results = append(results, dr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("discovery rows: %w", err)
	}
	if results == nil {
		results = []discovery.DiscoveryResult{}
	}
	return results, nil
}

// loadTagsForRoute reuses the same tag query pattern from RouteRepo.
func loadTagsForRoute(ctx context.Context, pool *pgxpool.Pool, routeID string) ([]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT tag FROM routes.route_tag WHERE route_id = $1::uuid ORDER BY tag`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, rows.Err()
}
