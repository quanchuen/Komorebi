// internal/infra/postgres/signal_repo.go
package postgres

import (
	"context"
	"fmt"

	"komorebi/internal/domain/environment"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SignalRepo implements environment.SignalRepository using PostGIS.
type SignalRepo struct {
	pool *pgxpool.Pool
}

// NewSignalRepo creates a new SignalRepo.
func NewSignalRepo(pool *pgxpool.Pool) *SignalRepo {
	return &SignalRepo{pool: pool}
}

// CountAlongRoute returns per-segment signal counts for the named route.
//
// For each segment in routes.route_segment (ordered by segment_order) it counts
// traffic signals within BufferM metres using ST_DWithin on geography casts.
// geography cast is safe here: traffic_signal already has a GiST index on
// the point geometry, and we are querying per segment (short linestrings), so
// the geography distance calculation is bounded.
func (r *SignalRepo) CountAlongRoute(params environment.RouteSignalParams) ([]environment.SegmentSignalCount, error) {
	bufM := params.BufferM
	if bufM <= 0 {
		bufM = 30
	}

	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT
			rs.segment_order,
			COUNT(ts.osm_node_id) AS signal_count
		FROM routes.route_segment rs
		LEFT JOIN environment.traffic_signal ts
			ON ST_DWithin(
				rs.geometry::geography,
				ts.geometry::geography,
				$2
			)
		WHERE rs.route_id = $1::uuid
		GROUP BY rs.segment_order
		ORDER BY rs.segment_order ASC
	`, params.RouteID, bufM)
	if err != nil {
		return nil, fmt.Errorf("signal.CountAlongRoute query: %w", err)
	}
	defer rows.Close()

	var counts []environment.SegmentSignalCount
	for rows.Next() {
		var sc environment.SegmentSignalCount
		if err := rows.Scan(&sc.SegmentOrder, &sc.Count); err != nil {
			return nil, fmt.Errorf("scan signal count row: %w", err)
		}
		counts = append(counts, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("signal count rows: %w", err)
	}
	if counts == nil {
		counts = []environment.SegmentSignalCount{}
	}
	return counts, nil
}

// TotalAlongRoute counts all traffic signals within BufferM metres of the full
// route geometry. Uses geography cast for metre-accurate distance.
//
// Unlike CountAlongRoute this does not join route_segment — it operates on the
// single route geometry row in routes.route, which is a complete LINESTRING Z.
// The GiST index on traffic_signal.geometry makes this efficient despite the
// geography cast: the bounding-box filter (&&) from the index eliminates almost
// all 75K signals before the distance check runs.
func (r *SignalRepo) TotalAlongRoute(params environment.RouteSignalParams) (int, error) {
	bufM := params.BufferM
	if bufM <= 0 {
		bufM = 30
	}

	ctx := context.Background()
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(ts.osm_node_id)
		FROM routes.route rt
		JOIN environment.traffic_signal ts
			ON ST_DWithin(
				rt.geometry::geography,
				ts.geometry::geography,
				$2
			)
		WHERE rt.id = $1::uuid
	`, params.RouteID, bufM).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("signal.TotalAlongRoute: %w", err)
	}
	return count, nil
}
