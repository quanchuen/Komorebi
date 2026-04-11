package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/route"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested route does not exist.
var ErrNotFound = errors.New("not found")

// RouteRepo implements route.Repository using PostgreSQL + PostGIS.
type RouteRepo struct {
	pool *pgxpool.Pool
}

// NewRouteRepo creates a new RouteRepo.
func NewRouteRepo(pool *pgxpool.Pool) *RouteRepo {
	return &RouteRepo{pool: pool}
}

// coordsToWKT encodes [][3]float64 as a LINESTRING Z WKT string.
func coordsToWKT(coords [][3]float64) string {
	pts := make([]string, len(coords))
	for i, c := range coords {
		pts[i] = fmt.Sprintf("%f %f %f", c[0], c[1], c[2])
	}
	return "LINESTRING Z(" + strings.Join(pts, ", ") + ")"
}

// wktToCoords parses a LINESTRING Z WKT string back to [][3]float64.
func wktToCoords(wkt string) ([][3]float64, error) {
	wkt = strings.TrimSpace(wkt)
	// Strip prefix variants
	for _, prefix := range []string{"LINESTRING Z(", "LINESTRING Z (", "LINESTRING(", "LINESTRING ("} {
		if strings.HasPrefix(wkt, prefix) {
			wkt = strings.TrimPrefix(wkt, prefix)
			break
		}
	}
	wkt = strings.TrimSuffix(wkt, ")")
	parts := strings.Split(wkt, ",")
	coords := make([][3]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var x, y, z float64
		n, err := fmt.Sscanf(p, "%f %f %f", &x, &y, &z)
		if err != nil || n < 2 {
			return nil, fmt.Errorf("wktToCoords: cannot parse %q", p)
		}
		coords = append(coords, [3]float64{x, y, z})
	}
	return coords, nil
}

// Create inserts a Route and its related waypoints, segments, and tags in a transaction.
func (r *RouteRepo) Create(rt *route.Route) error {
	ctx := context.Background()
	return r.withTx(ctx, func(tx pgx.Tx) error {
		wkt := coordsToWKT(rt.Geometry)
		_, err := tx.Exec(ctx, `
			INSERT INTO routes.route
				(id, name, description, geometry, distance_m, elevation_gain_m, elevation_loss_m,
				 difficulty, status, creator_id, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, ST_GeomFromText($4, 4326), $5, $6, $7,
			        $8::routes.difficulty, $9::routes.route_status,
			        $10::uuid, $11, $12)
		`, rt.ID, rt.Name, rt.Description, wkt,
			rt.DistanceM, rt.ElevationGainM, rt.ElevationLossM,
			string(rt.Difficulty), string(rt.Status),
			nullableUUID(rt.CreatorID), rt.CreatedAt, rt.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert route: %w", err)
		}
		if err := insertWaypoints(ctx, tx, rt.ID, rt.Waypoints); err != nil {
			return err
		}
		if err := insertSegments(ctx, tx, rt.ID, rt.Segments); err != nil {
			return err
		}
		return insertTags(ctx, tx, rt.ID, rt.Tags)
	})
}

// GetByID fetches a Route by its UUID, including waypoints, segments, and tags.
func (r *RouteRepo) GetByID(id string) (*route.Route, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, description,
		       ST_AsText(geometry) AS geometry_wkt,
		       distance_m, elevation_gain_m, elevation_loss_m,
		       difficulty, status, creator_id,
		       created_at, updated_at
		FROM routes.route
		WHERE id = $1::uuid
	`, id)

	rt, err := scanRoute(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("GetByID: %w", err)
	}

	wps, err := r.loadWaypoints(ctx, id)
	if err != nil {
		return nil, err
	}
	rt.Waypoints = wps

	segs, err := r.loadSegments(ctx, id)
	if err != nil {
		return nil, err
	}
	rt.Segments = segs

	tags, err := r.loadTags(ctx, id)
	if err != nil {
		return nil, err
	}
	rt.Tags = tags

	return rt, nil
}

// Update replaces all route fields, waypoints, segments and tags in a transaction.
func (r *RouteRepo) Update(rt *route.Route) error {
	ctx := context.Background()
	return r.withTx(ctx, func(tx pgx.Tx) error {
		wkt := coordsToWKT(rt.Geometry)
		_, err := tx.Exec(ctx, `
			UPDATE routes.route SET
				name = $2,
				description = $3,
				geometry = ST_GeomFromText($4, 4326),
				distance_m = $5,
				elevation_gain_m = $6,
				elevation_loss_m = $7,
				difficulty = $8::routes.difficulty,
				status = $9::routes.route_status,
				updated_at = $10
			WHERE id = $1::uuid
		`, rt.ID, rt.Name, rt.Description, wkt,
			rt.DistanceM, rt.ElevationGainM, rt.ElevationLossM,
			string(rt.Difficulty), string(rt.Status), rt.UpdatedAt)
		if err != nil {
			return fmt.Errorf("update route: %w", err)
		}
		for _, tbl := range []string{"routes.waypoint", "routes.route_segment", "routes.route_tag"} {
			if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE route_id = $1::uuid", tbl), rt.ID); err != nil {
				return fmt.Errorf("delete from %s: %w", tbl, err)
			}
		}
		if err := insertWaypoints(ctx, tx, rt.ID, rt.Waypoints); err != nil {
			return err
		}
		if err := insertSegments(ctx, tx, rt.ID, rt.Segments); err != nil {
			return err
		}
		return insertTags(ctx, tx, rt.ID, rt.Tags)
	})
}

// List returns a filtered, paginated list of routes.
func (r *RouteRepo) List(params route.ListParams) (route.ListResult, error) {
	ctx := context.Background()
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	args := []any{}
	argN := 1
	conditions := []string{}

	if params.BBox != [4]float64{} {
		conditions = append(conditions, fmt.Sprintf(
			"ST_Intersects(r.geometry, ST_MakeEnvelope($%d, $%d, $%d, $%d, 4326))",
			argN, argN+1, argN+2, argN+3,
		))
		args = append(args, params.BBox[0], params.BBox[1], params.BBox[2], params.BBox[3])
		argN += 4
	}

	if params.Difficulty != "" {
		conditions = append(conditions, fmt.Sprintf("r.difficulty = $%d::routes.difficulty", argN))
		args = append(args, string(params.Difficulty))
		argN++
	}

	if params.Surface != "" {
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM routes.route_segment rs WHERE rs.route_id = r.id AND rs.surface_type = $%d::routes.surface_type)",
			argN,
		))
		args = append(args, string(params.Surface))
		argN++
	}

	for _, tag := range params.Tags {
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM routes.route_tag rt2 WHERE rt2.route_id = r.id AND rt2.tag = $%d)",
			argN,
		))
		args = append(args, tag)
		argN++
	}

	if params.MinDistM > 0 {
		conditions = append(conditions, fmt.Sprintf("r.distance_m >= $%d", argN))
		args = append(args, params.MinDistM)
		argN++
	}
	if params.MaxDistM > 0 {
		conditions = append(conditions, fmt.Sprintf("r.distance_m <= $%d", argN))
		args = append(args, params.MaxDistM)
		argN++
	}

	if params.Cursor != "" {
		cursorAt, cursorID, err := decodeCursor(params.Cursor)
		if err == nil {
			conditions = append(conditions, fmt.Sprintf(
				"(r.created_at, r.id::text) < ($%d, $%d)",
				argN, argN+1,
			))
			args = append(args, cursorAt, cursorID)
			argN += 2
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.name, r.description,
		       ST_AsText(r.geometry) AS geometry_wkt,
		       r.distance_m, r.elevation_gain_m, r.elevation_loss_m,
		       r.difficulty, r.status, r.creator_id,
		       r.created_at, r.updated_at
		FROM routes.route r
		%s
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $%d
	`, where, argN)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return route.ListResult{}, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var routes []*route.Route
	for rows.Next() {
		rt, err := scanRoute(rows)
		if err != nil {
			return route.ListResult{}, fmt.Errorf("scan route row: %w", err)
		}
		routes = append(routes, rt)
	}
	if err := rows.Err(); err != nil {
		return route.ListResult{}, fmt.Errorf("list rows: %w", err)
	}

	var nextCursor string
	if len(routes) > limit {
		last := routes[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		routes = routes[:limit]
	}

	for _, rt := range routes {
		tags, err := r.loadTags(ctx, rt.ID)
		if err == nil {
			rt.Tags = tags
		}
	}

	return route.ListResult{Routes: routes, NextCursor: nextCursor}, nil
}

// Delete removes a route by ID (cascades to child tables).
func (r *RouteRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, "DELETE FROM routes.route WHERE id = $1::uuid", id)
	return err
}

// --- helpers ---

func (r *RouteRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *RouteRepo) loadWaypoints(ctx context.Context, routeID string) ([]route.Waypoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ST_Y(geometry) AS lat, ST_X(geometry) AS lon, name, type, sort_order
		FROM routes.waypoint WHERE route_id = $1::uuid ORDER BY sort_order
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("loadWaypoints: %w", err)
	}
	defer rows.Close()
	var wps []route.Waypoint
	for rows.Next() {
		var wp route.Waypoint
		var wpType string
		if err := rows.Scan(&wp.ID, &wp.Lat, &wp.Lon, &wp.Name, &wpType, &wp.SortOrder); err != nil {
			return nil, err
		}
		wp.Type = route.WaypointType(wpType)
		wps = append(wps, wp)
	}
	if wps == nil {
		wps = []route.Waypoint{}
	}
	return wps, rows.Err()
}

func (r *RouteRepo) loadSegments(ctx context.Context, routeID string) ([]route.Segment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ST_AsText(geometry), surface_type, grade_percent, segment_order
		FROM routes.route_segment WHERE route_id = $1::uuid ORDER BY segment_order
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("loadSegments: %w", err)
	}
	defer rows.Close()
	var segs []route.Segment
	for rows.Next() {
		var seg route.Segment
		var surfType, wkt string
		if err := rows.Scan(&seg.ID, &wkt, &surfType, &seg.GradePercent, &seg.SegmentOrder); err != nil {
			return nil, err
		}
		seg.SurfaceType = route.SurfaceType(surfType)
		coords, err := wktToCoords(wkt)
		if err != nil {
			return nil, err
		}
		seg.Geometry = coords
		segs = append(segs, seg)
	}
	if segs == nil {
		segs = []route.Segment{}
	}
	return segs, rows.Err()
}

func (r *RouteRepo) loadTags(ctx context.Context, routeID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tag FROM routes.route_tag WHERE route_id = $1::uuid ORDER BY tag
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("loadTags: %w", err)
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, rows.Err()
}

func insertWaypoints(ctx context.Context, tx pgx.Tx, routeID string, wps []route.Waypoint) error {
	for _, wp := range wps {
		pt := fmt.Sprintf("POINT(%f %f)", wp.Lon, wp.Lat)
		if wp.ID == "" {
			wp.ID = genUUID()
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO routes.waypoint (id, route_id, geometry, name, type, sort_order)
			VALUES ($1::uuid, $2::uuid, ST_GeomFromText($3, 4326), $4, $5::routes.waypoint_type, $6)
		`, wp.ID, routeID, pt, wp.Name, string(wp.Type), wp.SortOrder)
		if err != nil {
			return fmt.Errorf("insert waypoint: %w", err)
		}
	}
	return nil
}

func insertSegments(ctx context.Context, tx pgx.Tx, routeID string, segs []route.Segment) error {
	for _, seg := range segs {
		wkt := coordsToWKT(seg.Geometry)
		if seg.ID == "" {
			seg.ID = genUUID()
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO routes.route_segment (id, route_id, geometry, surface_type, grade_percent, segment_order)
			VALUES ($1::uuid, $2::uuid, ST_GeomFromText($3, 4326), $4::routes.surface_type, $5, $6)
		`, seg.ID, routeID, wkt, string(seg.SurfaceType), seg.GradePercent, seg.SegmentOrder)
		if err != nil {
			return fmt.Errorf("insert segment: %w", err)
		}
	}
	return nil
}

func insertTags(ctx context.Context, tx pgx.Tx, routeID string, tags []string) error {
	for _, tag := range tags {
		_, err := tx.Exec(ctx, `
			INSERT INTO routes.route_tag (route_id, tag) VALUES ($1::uuid, $2)
			ON CONFLICT DO NOTHING
		`, routeID, tag)
		if err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}
	return nil
}

// scanRoute scans one route row (without waypoints/segments/tags).
func scanRoute(row pgx.Row) (*route.Route, error) {
	var rt route.Route
	var geomWKT string
	var diff, status string
	var creatorID *string
	err := row.Scan(
		&rt.ID, &rt.Name, &rt.Description,
		&geomWKT,
		&rt.DistanceM, &rt.ElevationGainM, &rt.ElevationLossM,
		&diff, &status, &creatorID,
		&rt.CreatedAt, &rt.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	rt.Difficulty = route.Difficulty(diff)
	rt.Status = route.Status(status)
	if creatorID != nil {
		rt.CreatorID = *creatorID
	}
	coords, err := wktToCoords(geomWKT)
	if err != nil {
		return nil, fmt.Errorf("parse geometry: %w", err)
	}
	rt.Geometry = coords
	rt.Tags = []string{}
	rt.Waypoints = []route.Waypoint{}
	rt.Segments = []route.Segment{}
	return &rt, nil
}

// nullableUUID returns nil if id is empty, otherwise a pointer to id.
func nullableUUID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}

// genUUID generates a random UUID v4 string.
func genUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// cursorData is JSON-encoded into the pagination cursor token.
type cursorData struct {
	CreatedAt time.Time `json:"ca"`
	ID        string    `json:"id"`
}

func encodeCursor(createdAt time.Time, id string) string {
	b, _ := json.Marshal(cursorData{CreatedAt: createdAt, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(cursor string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	var cd cursorData
	if err := json.Unmarshal(b, &cd); err != nil {
		return time.Time{}, "", err
	}
	return cd.CreatedAt, cd.ID, nil
}
