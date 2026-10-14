// internal/infra/postgres/venue_repo.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"komorebi/internal/domain/environment"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VenueRepo implements environment.VenueRepository using PostGIS.
type VenueRepo struct {
	pool *pgxpool.Pool
}

// NewVenueRepo creates a new VenueRepo.
func NewVenueRepo(pool *pgxpool.Pool) *VenueRepo {
	return &VenueRepo{pool: pool}
}

// AlongRoute returns venues within BufferM metres of the named route geometry.
// Uses ST_DWithin on the route geometry (geography cast for metre-accurate distance).
// Category filter is applied when params.Category is non-empty.
func (r *VenueRepo) AlongRoute(params environment.AlongRouteParams) ([]environment.Venue, error) {
	bufferM := params.BufferM
	if bufferM <= 0 {
		bufferM = 200
	}

	ctx := context.Background()

	args := []any{params.RouteID, bufferM}
	categoryClause := ""
	if params.Category != "" {
		args = append(args, params.Category)
		categoryClause = fmt.Sprintf("AND v.category = $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT
			v.id,
			v.osm_id,
			v.name,
			v.category,
			COALESCE(v.brand, ''),
			ST_Y(v.geometry) AS lat,
			ST_X(v.geometry) AS lon,
			COALESCE(v.osm_tags, '{}')
		FROM environment.venue v
		JOIN routes.route r ON r.id = $1::uuid
		WHERE ST_DWithin(
			v.geometry::geography,
			r.geometry::geography,
			$2
		)
		%s
		ORDER BY v.name ASC
	`, categoryClause)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("venue.AlongRoute query: %w", err)
	}
	defer rows.Close()

	var venues []environment.Venue
	for rows.Next() {
		var v environment.Venue
		var osmID *int64
		var tagsJSON []byte
		if err := rows.Scan(
			&v.ID, &osmID, &v.Name, &v.Category, &v.Brand,
			&v.Lat, &v.Lon, &tagsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan venue row: %w", err)
		}
		if osmID != nil {
			v.OsmID = *osmID
		}
		if len(tagsJSON) > 0 {
			if err := json.Unmarshal(tagsJSON, &v.OsmTags); err != nil {
				v.OsmTags = map[string]string{}
			}
		} else {
			v.OsmTags = map[string]string{}
		}
		venues = append(venues, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("venue rows: %w", err)
	}
	if venues == nil {
		venues = []environment.Venue{}
	}
	return venues, nil
}

// ListTags returns all rows from environment.venue_tag_mapping ordered by hashtag.
func (r *VenueRepo) ListTags() ([]environment.VenueTag, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT hashtag, COALESCE(description, ''), is_brand
		FROM environment.venue_tag_mapping
		ORDER BY hashtag ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("venue.ListTags query: %w", err)
	}
	defer rows.Close()

	var tags []environment.VenueTag
	for rows.Next() {
		var vt environment.VenueTag
		if err := rows.Scan(&vt.Hashtag, &vt.Description, &vt.IsBrand); err != nil {
			return nil, fmt.Errorf("scan tag row: %w", err)
		}
		tags = append(tags, vt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tag rows: %w", err)
	}
	if tags == nil {
		tags = []environment.VenueTag{}
	}
	return tags, nil
}

// GetTagMapping fetches the full tag mapping (including osm_filter JSON) for a hashtag.
// Returns nil, nil when the hashtag is not found.
func (r *VenueRepo) GetTagMapping(hashtag string) (*environment.VenueTagMapping, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT hashtag, osm_filter, COALESCE(description, ''), is_brand
		FROM environment.venue_tag_mapping
		WHERE hashtag = $1
	`, hashtag)

	var m environment.VenueTagMapping
	var filterJSON []byte
	err := row.Scan(&m.Hashtag, &filterJSON, &m.Description, &m.IsBrand)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("GetTagMapping scan: %w", err)
	}
	if len(filterJSON) > 0 {
		if err := json.Unmarshal(filterJSON, &m.OSMFilter); err != nil {
			m.OSMFilter = map[string]string{}
		}
	} else {
		m.OSMFilter = map[string]string{}
	}
	return &m, nil
}

// NearestAlongLine returns the nearest venue matching the OSM filter within
// BufferM metres of the route geometry WKT, ordered by distance ASC LIMIT 1.
// Returns nil, nil when no matching venue exists within the corridor.
func (r *VenueRepo) NearestAlongLine(params environment.NearestAlongLineParams) (*environment.Venue, error) {
	bufferM := params.BufferM
	if bufferM <= 0 {
		bufferM = 200
	}

	ctx := context.Background()

	args := []any{params.RouteWKT, bufferM}
	var filterClauses []string
	for k, v := range params.OSMFilter {
		if k == "brand" && params.IsBrand {
			args = append(args, "%"+v+"%")
			filterClauses = append(filterClauses,
				fmt.Sprintf("v.brand ILIKE $%d", len(args)))
		} else {
			args = append(args, v)
			filterClauses = append(filterClauses,
				fmt.Sprintf("v.osm_tags->>'%s' = $%d", k, len(args)))
		}
	}

	filterSQL := ""
	if len(filterClauses) > 0 {
		filterSQL = "AND " + strings.Join(filterClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			v.id,
			v.osm_id,
			v.name,
			v.category,
			COALESCE(v.brand, ''),
			ST_Y(v.geometry) AS lat,
			ST_X(v.geometry) AS lon,
			COALESCE(v.osm_tags::text, '{}')
		FROM environment.venue v
		WHERE ST_DWithin(
			v.geometry::geography,
			ST_GeomFromText($1, 4326)::geography,
			$2
		)
		%s
		ORDER BY v.geometry::geography <-> ST_GeomFromText($1, 4326)::geography
		LIMIT 1
	`, filterSQL)

	row := r.pool.QueryRow(ctx, query, args...)

	var v environment.Venue
	var osmID *int64
	var tagsJSON []byte
	err := row.Scan(
		&v.ID, &osmID, &v.Name, &v.Category, &v.Brand,
		&v.Lat, &v.Lon, &tagsJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("NearestAlongLine scan: %w", err)
	}
	if osmID != nil {
		v.OsmID = *osmID
	}
	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &v.OsmTags); err != nil {
			v.OsmTags = map[string]string{}
		}
	} else {
		v.OsmTags = map[string]string{}
	}
	return &v, nil
}
