// internal/infra/postgres/shadow_repo.go
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ShadowRepo implements environment.ShadowRepository using PostGIS.
type ShadowRepo struct {
	pool *pgxpool.Pool
}

// NewShadowRepo creates a new ShadowRepo.
func NewShadowRepo(pool *pgxpool.Pool) *ShadowRepo {
	return &ShadowRepo{pool: pool}
}

// ForRoute returns shadow grid cells intersecting a buffered route geometry
// at the given hour slot and month.
//
// The route buffer is computed in geography space (metres) using ST_Buffer on
// the geography cast, which is accurate for Tokyo latitudes. We cast the result
// back to geometry so the GiST index on cell_geometry is exercised by
// ST_Intersects.
//
// SQL pattern:
//
//	SELECT id, ST_AsText(cell_geometry), hour_slot, month, shade_coverage
//	FROM environment.shadow_grid
//	WHERE hour_slot = $1
//	  AND month     = $2
//	  AND ST_Intersects(
//	        cell_geometry,
//	        ST_Buffer(
//	          ST_GeomFromText($3, 4326)::geography,
//	          $4
//	        )::geometry
//	      )
func (r *ShadowRepo) ForRoute(params environment.ShadowParams) ([]environment.ShadowGrid, error) {
	bufferM := params.BufferM
	if bufferM <= 0 {
		bufferM = 50
	}

	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			ST_AsText(cell_geometry) AS cell_wkt,
			hour_slot,
			month,
			shade_coverage
		FROM environment.shadow_grid
		WHERE hour_slot = $1
		  AND month     = $2
		  AND ST_Intersects(
		        cell_geometry,
		        ST_Buffer(
		          ST_GeomFromText($3, 4326)::geography,
		          $4
		        )::geometry
		      )
		ORDER BY id
	`, params.HourSlot, params.Month, params.RouteGeometryWKT, bufferM)
	if err != nil {
		return nil, fmt.Errorf("shadow.ForRoute query: %w", err)
	}
	defer rows.Close()

	var cells []environment.ShadowGrid
	for rows.Next() {
		var sg environment.ShadowGrid
		var sgID int64
		var cellWKT string
		if err := rows.Scan(&sgID, &cellWKT, &sg.HourSlot, &sg.Month, &sg.ShadeCoverage); err != nil {
			return nil, fmt.Errorf("scan shadow row: %w", err)
		}
		sg.ID = fmt.Sprintf("%d", sgID)
		coords, err := ParsePolygonWKT(cellWKT)
		if err != nil {
			return nil, fmt.Errorf("parse shadow cell geometry: %w", err)
		}
		sg.CellGeometry = coords
		cells = append(cells, sg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("shadow rows: %w", err)
	}
	if cells == nil {
		cells = []environment.ShadowGrid{}
	}
	return cells, nil
}

// ParsePolygonWKT parses a WKT POLYGON string into [][2]float64 ring coordinates.
// Handles "POLYGON((lon lat, ...))" and "POLYGON ((lon lat, ...))" formats
// produced by PostGIS ST_AsText.
func ParsePolygonWKT(wkt string) ([][2]float64, error) {
	wkt = strings.TrimSpace(wkt)
	for _, prefix := range []string{"POLYGON((", "POLYGON (("} {
		if strings.HasPrefix(wkt, prefix) {
			wkt = strings.TrimPrefix(wkt, prefix)
			break
		}
	}
	wkt = strings.TrimSuffix(wkt, "))")
	parts := strings.Split(wkt, ",")
	coords := make([][2]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var x, y float64
		if _, err := fmt.Sscanf(p, "%f %f", &x, &y); err != nil {
			return nil, fmt.Errorf("parsePolygonWKT: cannot parse %q", p)
		}
		coords = append(coords, [2]float64{x, y})
	}
	return coords, nil
}
