# Phase 4: RoutePlan + Venue Hashtag Resolution — Implementation Plan

**Date:** 2026-04-10
**Author:** Claude Code
**Status:** Draft

---

## Context

The domain layer already has `RoutePlan`, `StopPoint`, and `PlanTask` with their full type hierarchies in `internal/domain/plan/`. The `VenueTagMapping` and `Venue` types are in `internal/domain/environment/`. `VenueRepo` can query venues along a named route with `ST_DWithin`.

This phase wires everything together:

1. A PostgreSQL-backed `PlanRepo` persists plans, stops, and tasks to the `plan` schema.
2. A `VenueResolutionService` parses hashtags from task descriptions, resolves them against `venue_tag_mapping`, and snaps to the nearest matching venue within a 200 m buffer of the current route geometry.
3. A `PlanService` orchestrates the full plan lifecycle — create, add/remove stops (with re-route via Valhalla), add tasks (with venue resolution), and read.
4. Six HTTP handlers wire the REST API defined in the design spec.
5. `router.go` and `main.go` are updated to register the new wiring.

---

## Scope

| File | Action |
|------|--------|
| `internal/infra/postgres/plan_repo.go` | New — CRUD for `route_plan`, `stop_point`, `plan_task` |
| `internal/infra/postgres/plan_repo_test.go` | New — integration tests against live DB |
| `internal/app/venue_resolution_service.go` | New — hashtag parse, tag lookup, spatial snap |
| `internal/app/venue_resolution_service_test.go` | New — unit tests with stubs |
| `internal/app/plan_service.go` | New — CreatePlan, AddStop, RemoveStop, AddTask, GetPlan |
| `internal/app/plan_service_test.go` | New — unit tests with stubs |
| `internal/api/plan_handler.go` | New — six HTTP handlers |
| `internal/api/plan_handler_test.go` | New — handler unit tests |
| `internal/api/router.go` | Modify — add six plan routes |
| `cmd/api/main.go` | Modify — wire PlanRepo, VenueResolutionService, PlanService, PlanHandler |

**Out of scope:** Auth middleware (plans are created with a hardcoded `user_id` in V1), green-wave / environment overlay on plan segments (covered in environment-aware routing plan).

---

## Prerequisites

- `plan.route_plan`, `plan.stop_point`, `plan_task` tables exist (from PostGIS schema migration).
- `environment.venue` and `environment.venue_tag_mapping` tables exist.
- `routes.route` has a `geometry` column queryable via `ST_DWithin`.
- `go-chi/chi/v5` and `jackc/pgx/v5` are in `go.mod`.
- `internal/domain/plan` types compile as-is (confirmed by existing tests).

---

## Step Checklist

- [ ] Step 1 — Infra: `plan_repo.go` — CRUD for plans, stops, and tasks
- [ ] Step 2 — App: `venue_resolution_service.go` — hashtag parsing and venue snapping
- [ ] Step 3 — App: `plan_service.go` — plan lifecycle orchestration
- [ ] Step 4 — API: `plan_handler.go` — six HTTP handlers
- [ ] Step 5 — Wire: `router.go` and `main.go`
- [ ] Step 6 — Integration tests: `plan_repo_test.go`
- [ ] Step 7 — Self-review

---

## Step 1 — Infra: PostgreSQL Plan Repository

**File:** `internal/infra/postgres/plan_repo.go`

### Design notes

- All three tables (`route_plan`, `stop_point`, `plan_task`) are written inside a single transaction for `Create` and `Update` to maintain aggregate consistency.
- `GetByID` loads the plan row then issues two separate queries for stops and tasks (same pattern as `loadWaypoints` / `loadSegments` in `route_repo.go`).
- `Delete` cascades via FK; a single `DELETE FROM plan.route_plan` is sufficient.
- `geometry` for `stop_point` is stored as `POINT(lon lat)` WKT (SRID 4326), retrieved with `ST_Y(geometry) AS lat, ST_X(geometry) AS lon`.
- Nullable FK `venue_id` and `resolved_venue_id` are handled with `*string` pointers from pgx and converted to empty string for the domain type.
- The `plan.stop_type` and `plan.task_status` Postgres enums must match the domain constants (`manual`, `venue_resolved`, `waypoint`; `unresolved`, `matched`, `completed`).

```go
// internal/infra/postgres/plan_repo.go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"komorebi/internal/domain/plan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlanRepo implements plan.Repository using PostgreSQL.
type PlanRepo struct {
	pool *pgxpool.Pool
}

// NewPlanRepo creates a new PlanRepo.
func NewPlanRepo(pool *pgxpool.Pool) *PlanRepo {
	return &PlanRepo{pool: pool}
}

// Create inserts a RoutePlan with its stops and tasks in a single transaction.
func (r *PlanRepo) Create(p *plan.RoutePlan) error {
	ctx := context.Background()
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO plan.route_plan
				(id, user_id, departure_at, speed_model,
				 shade_weight, greenery_weight, wind_weight, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, $3, $4::plan.speed_model,
			        $5, $6, $7, $8, $9)
		`,
			p.ID, nullableUUID(p.UserID), p.DepartureAt,
			string(p.SpeedModel),
			p.Preferences.ShadeWeight, p.Preferences.GreeneryWeight, p.Preferences.WindWeight,
			p.CreatedAt, p.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert route_plan: %w", err)
		}
		if err := insertStops(ctx, tx, p.ID, p.Stops); err != nil {
			return err
		}
		return insertTasks(ctx, tx, p.ID, p.Tasks)
	})
}

// GetByID fetches a RoutePlan by UUID, including stops and tasks.
// Returns ErrNotFound if no row exists.
func (r *PlanRepo) GetByID(id string) (*plan.RoutePlan, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(user_id::text, ''),
		       departure_at, speed_model,
		       shade_weight, greenery_weight, wind_weight,
		       created_at, updated_at
		FROM plan.route_plan
		WHERE id = $1::uuid
	`, id)

	var p plan.RoutePlan
	var speedModel string
	err := row.Scan(
		&p.ID, &p.UserID,
		&p.DepartureAt, &speedModel,
		&p.Preferences.ShadeWeight, &p.Preferences.GreeneryWeight, &p.Preferences.WindWeight,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("GetByID plan: %w", err)
	}
	p.SpeedModel = plan.SpeedModel(speedModel)

	stops, err := r.loadStops(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Stops = stops

	tasks, err := r.loadTasks(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Tasks = tasks

	return &p, nil
}

// Update replaces all fields, stops, and tasks for an existing plan in a transaction.
func (r *PlanRepo) Update(p *plan.RoutePlan) error {
	ctx := context.Background()
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE plan.route_plan SET
				departure_at   = $2,
				speed_model    = $3::plan.speed_model,
				shade_weight   = $4,
				greenery_weight = $5,
				wind_weight    = $6,
				updated_at     = $7
			WHERE id = $1::uuid
		`,
			p.ID, p.DepartureAt, string(p.SpeedModel),
			p.Preferences.ShadeWeight, p.Preferences.GreeneryWeight, p.Preferences.WindWeight,
			p.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("update route_plan: %w", err)
		}
		for _, tbl := range []string{"plan.stop_point", "plan.plan_task"} {
			if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE plan_id = $1::uuid", tbl), p.ID); err != nil {
				return fmt.Errorf("delete from %s: %w", tbl, err)
			}
		}
		if err := insertStops(ctx, tx, p.ID, p.Stops); err != nil {
			return err
		}
		return insertTasks(ctx, tx, p.ID, p.Tasks)
	})
}

// Delete removes a plan by ID. Child rows cascade via FK.
func (r *PlanRepo) Delete(id string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, "DELETE FROM plan.route_plan WHERE id = $1::uuid", id)
	return err
}

// --- helpers ---

func (r *PlanRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

func (r *PlanRepo) loadStops(ctx context.Context, planID string) ([]plan.StopPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id,
		       ST_Y(geometry) AS lat,
		       ST_X(geometry) AS lon,
		       type,
		       sort_order,
		       COALESCE(venue_id::text, ''),
		       COALESCE(resolved_name, '')
		FROM plan.stop_point
		WHERE plan_id = $1::uuid
		ORDER BY sort_order
	`, planID)
	if err != nil {
		return nil, fmt.Errorf("loadStops: %w", err)
	}
	defer rows.Close()
	var stops []plan.StopPoint
	for rows.Next() {
		var sp plan.StopPoint
		var stopType string
		if err := rows.Scan(
			&sp.ID, &sp.Lat, &sp.Lon,
			&stopType, &sp.SortOrder,
			&sp.VenueID, &sp.ResolvedName,
		); err != nil {
			return nil, fmt.Errorf("scan stop_point row: %w", err)
		}
		sp.Type = plan.StopType(stopType)
		stops = append(stops, sp)
	}
	if stops == nil {
		stops = []plan.StopPoint{}
	}
	return stops, rows.Err()
}

func (r *PlanRepo) loadTasks(ctx context.Context, planID string) ([]plan.PlanTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id,
		       description,
		       COALESCE(hashtag, ''),
		       status,
		       COALESCE(resolved_venue_id::text, '')
		FROM plan.plan_task
		WHERE plan_id = $1::uuid
		ORDER BY id
	`, planID)
	if err != nil {
		return nil, fmt.Errorf("loadTasks: %w", err)
	}
	defer rows.Close()
	var tasks []plan.PlanTask
	for rows.Next() {
		var t plan.PlanTask
		var status string
		if err := rows.Scan(
			&t.ID, &t.Description, &t.Hashtag,
			&status, &t.ResolvedVenueID,
		); err != nil {
			return nil, fmt.Errorf("scan plan_task row: %w", err)
		}
		t.Status = plan.TaskStatus(status)
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []plan.PlanTask{}
	}
	return tasks, rows.Err()
}

func insertStops(ctx context.Context, tx pgx.Tx, planID string, stops []plan.StopPoint) error {
	for _, sp := range stops {
		pt := fmt.Sprintf("POINT(%f %f)", sp.Lon, sp.Lat)
		if sp.ID == "" {
			sp.ID = genUUID()
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO plan.stop_point
				(id, plan_id, geometry, type, sort_order, venue_id, resolved_name)
			VALUES ($1::uuid, $2::uuid, ST_GeomFromText($3, 4326),
			        $4::plan.stop_type, $5, $6, $7)
		`,
			sp.ID, planID, pt,
			string(sp.Type), sp.SortOrder,
			nullableUUID(sp.VenueID), sp.ResolvedName,
		)
		if err != nil {
			return fmt.Errorf("insert stop_point: %w", err)
		}
	}
	return nil
}

func insertTasks(ctx context.Context, tx pgx.Tx, planID string, tasks []plan.PlanTask) error {
	for _, t := range tasks {
		if t.ID == "" {
			t.ID = genUUID()
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO plan.plan_task
				(id, plan_id, description, hashtag, status, resolved_venue_id)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5::plan.task_status, $6)
		`,
			t.ID, planID, t.Description,
			nullableStr(t.Hashtag),
			string(t.Status),
			nullableUUID(t.ResolvedVenueID),
		)
		if err != nil {
			return fmt.Errorf("insert plan_task: %w", err)
		}
	}
	return nil
}

// nullableStr returns nil for empty string, otherwise a pointer.
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

---

## Step 2 — App: Venue Resolution Service

**File:** `internal/app/venue_resolution_service.go`

### Design notes

- Hashtag extraction uses a simple `strings.Fields` split followed by a prefix `#` check. The first hashtag wins per task description; if none is present, the task stays `unresolved`.
- Tag lookup calls `VenueRepo.ListTags()` on every resolution. At current scale this is a small table; caching can be added later if needed.
- Spatial snapping requires querying venues along a geometry — not a named route ID. We need a new method `NearestVenueAlongLine` on the venue repository that accepts a WKT LINESTRING and a category filter derived from the OSM filter in the `VenueTagMapping`.
- The OSM filter in `VenueTagMapping.OSMFilter` is a `map[string]string`. The resolution service maps this to a SQL WHERE clause by iterating key-value pairs and building `AND v.osm_tags->>'key' = 'value'` conditions. Brand matching uses `ILIKE '%value%'` when `IsBrand` is true.

### New domain method needed

Add a `NearestAlongLine` method to `VenueRepository` in `internal/domain/environment/venue.go`:

```go
// NearestAlongLineParams holds parameters for finding the nearest venue along a route line.
type NearestAlongLineParams struct {
	// RouteWKT is the LINESTRING WKT of the current route geometry (SRID 4326).
	RouteWKT  string
	// OSMFilter maps OSM tag keys to expected values (all must match).
	OSMFilter map[string]string
	// IsBrand when true uses ILIKE matching on brand field values.
	IsBrand   bool
	// BufferM is the search corridor in metres; defaults to 200.
	BufferM   float64
}

// NearestAlongLine returns the single closest venue matching the OSM filter
// within BufferM metres of the given route geometry.
// Returns nil, nil when no match is found.
func (VenueRepository) NearestAlongLine(params NearestAlongLineParams) (*Venue, error)
```

Update the interface:

```go
// VenueRepository defines persistence operations for venue reads.
type VenueRepository interface {
	AlongRoute(params AlongRouteParams) ([]Venue, error)
	ListTags() ([]VenueTag, error)
	// NearestAlongLine returns the single nearest venue matching the OSM filter
	// within BufferM metres of the route geometry WKT.
	// Returns nil, nil when no matching venue exists within the corridor.
	NearestAlongLine(params NearestAlongLineParams) (*Venue, error)
}
```

### New infra method

Add `NearestAlongLine` to `VenueRepo` in `internal/infra/postgres/venue_repo.go`:

```go
// NearestAlongLine returns the nearest venue matching the OSM filter within
// BufferM metres of the route geometry WKT, ordered by distance ASC LIMIT 1.
func (r *VenueRepo) NearestAlongLine(params environment.NearestAlongLineParams) (*environment.Venue, error) {
	bufferM := params.BufferM
	if bufferM <= 0 {
		bufferM = 200
	}

	ctx := context.Background()

	// Build dynamic tag filter from OSMFilter map.
	// Each key-value pair becomes: v.osm_tags->>'key' = 'value'
	// Brand entries use ILIKE on the brand column.
	args := []any{params.RouteWKT, bufferM}
	filterClauses := []string{}
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
			COALESCE(v.osm_tags, '{}')
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
```

### Resolution service

```go
// internal/app/venue_resolution_service.go
package app

import (
	"strings"

	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
)

// VenueResolver is the interface the resolution service uses to look up venues.
// Satisfied by *postgres.VenueRepo.
type VenueResolver interface {
	ListTags() ([]environment.VenueTag, error)
	NearestAlongLine(params environment.NearestAlongLineParams) (*environment.Venue, error)
}

// VenueTagLookup is the interface for fetching full VenueTagMapping records (including OSMFilter).
// Separate from VenueResolver because ListTags only returns VenueTag (no OSMFilter).
// We need GetTagMapping to resolve the filter.
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
```

**Note:** The `fmt` import needs to be added to `venue_resolution_service.go`.

### New repository method: GetTagMapping

`GetTagMapping` returns the full `VenueTagMapping` (including `OSMFilter`). Add to `VenueRepository` interface:

```go
// GetTagMapping returns the VenueTagMapping for the given hashtag, or nil if not found.
GetTagMapping(hashtag string) (*VenueTagMapping, error)
```

Implement in `VenueRepo`:

```go
// GetTagMapping fetches the full tag mapping (including osm_filter JSON) for a hashtag.
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
```

---

## Step 3 — App: Plan Service

**File:** `internal/app/plan_service.go`

### Design notes

- `CreatePlan` accepts a `CreatePlanRequest`. When `SourceRouteID` is non-empty, the service loads the curated route's waypoints, converts them to `StopWaypoint` stops, and pre-populates the plan. It then calls `reroute` to compute initial geometry.
- `AddStop` appends the stop, calls `reroute`, persists the updated plan.
- `RemoveStop` delegates to `plan.RoutePlan.RemoveStop`, calls `reroute`, persists.
- `AddTask` appends the task, calls `resolveTask` (which calls `VenueResolutionService.ResolveTask` with the current route WKT), then persists.
- `reroute` calls `RoutingService.GetDirections` with the current stops; the returned `GeoJSON` WKT is stored in the plan's `RouteWKT` field (add this field to the domain type).
- `GetPlan` delegates to `PlanRepo.GetByID`.

### Domain type extension

Add `RouteWKT string` to `RoutePlan` in `internal/domain/plan/plan.go` (stores the current computed route as a WKT LINESTRING, used for venue resolution snapping):

```go
// RoutePlan aggregates a planned cycling outing.
type RoutePlan struct {
	ID          string
	UserID      string
	DepartureAt time.Time
	SpeedModel  SpeedModel
	Preferences Preferences
	Stops       []StopPoint
	Tasks       []PlanTask
	// RouteWKT holds the last-computed Valhalla route as a LINESTRING WKT (SRID 4326).
	// Empty until the first route computation.
	RouteWKT  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

Also add `route_wkt TEXT` column to `plan.route_plan` in the repo queries (and in a migration if needed).

### Plan Repository interface extension

Add `route_wkt` handling to `PlanRepo.Create`, `GetByID`, and `Update`:

In `Create`:
```sql
INSERT INTO plan.route_plan
    (id, user_id, departure_at, speed_model,
     shade_weight, greenery_weight, wind_weight, route_wkt, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, $3, $4::plan.speed_model,
        $5, $6, $7, $8, $9, $10)
```

In `GetByID` SELECT: add `COALESCE(route_wkt, '')` to the scan.

In `Update` SET: add `route_wkt = $8` (renumbering subsequent params).

### RouteGeometryReader interface

The plan service needs to read a curated route's geometry to pre-populate stops when creating from a template. Define a minimal interface:

```go
// RouteGeometryReader can load a Route's waypoints for plan seeding.
type RouteGeometryReader interface {
	GetByID(id string) (*routedomain.Route, error)
}
```

Import `routedomain "komorebi/internal/domain/route"`.

### Service code

```go
// internal/app/plan_service.go
package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"komorebi/internal/domain/plan"
	routedomain "komorebi/internal/domain/route"
)

// PlanRepository is the persistence interface used by PlanService.
// Satisfied by *postgres.PlanRepo.
type PlanRepository interface {
	Create(p *plan.RoutePlan) error
	GetByID(id string) (*plan.RoutePlan, error)
	Update(p *plan.RoutePlan) error
	Delete(id string) error
}

// CreatePlanRequest holds the inputs for creating a new plan.
type CreatePlanRequest struct {
	UserID        string
	DepartureAt   time.Time
	SpeedModel    plan.SpeedModel
	Preferences   plan.Preferences
	// SourceRouteID when non-empty seeds the plan from a curated route's waypoints.
	SourceRouteID string
}

// AddStopRequest holds inputs for adding a stop.
type AddStopRequest struct {
	PlanID string
	Stop   plan.StopPoint
}

// AddTaskRequest holds inputs for adding a task.
type AddTaskRequest struct {
	PlanID      string
	Description string
	Hashtag     string
}

// PlanService orchestrates the RoutePlan lifecycle.
type PlanService struct {
	repo       PlanRepository
	routeRepo  RouteGeometryReader
	routing    *RoutingService
	resolution *VenueResolutionService
}

// NewPlanService creates a PlanService.
func NewPlanService(
	repo PlanRepository,
	routeRepo RouteGeometryReader,
	routing *RoutingService,
	resolution *VenueResolutionService,
) *PlanService {
	return &PlanService{
		repo:       repo,
		routeRepo:  routeRepo,
		routing:    routing,
		resolution: resolution,
	}
}

// CreatePlan builds a new RoutePlan, optionally seeded from a curated route.
func (s *PlanService) CreatePlan(req CreatePlanRequest) (*plan.RoutePlan, error) {
	p := plan.NewRoutePlan(req.UserID)
	p.DepartureAt = req.DepartureAt
	p.SpeedModel = req.SpeedModel
	p.Preferences = req.Preferences

	if req.SourceRouteID != "" {
		rt, err := s.routeRepo.GetByID(req.SourceRouteID)
		if err != nil {
			return nil, fmt.Errorf("load source route: %w", err)
		}
		for i, wp := range rt.Waypoints {
			p.AddStop(plan.StopPoint{
				ID:           plan.NewStopID(),
				Lat:          wp.Lat,
				Lon:          wp.Lon,
				Type:         plan.StopWaypoint,
				SortOrder:    i,
				ResolvedName: wp.Name,
			})
		}
		if len(p.Stops) >= 2 {
			if err := s.reroute(p); err != nil {
				// Non-fatal: plan is valid even without an initial route line.
				p.RouteWKT = ""
			}
		}
	}

	if err := s.repo.Create(p); err != nil {
		return nil, fmt.Errorf("persist plan: %w", err)
	}
	return p, nil
}

// GetPlan returns a RoutePlan with resolved stops and tasks.
func (s *PlanService) GetPlan(id string) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}

// AddStop appends a stop to the plan and triggers a re-route.
func (s *PlanService) AddStop(req AddStopRequest) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(req.PlanID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("load plan for AddStop: %w", err)
	}

	stop := req.Stop
	if stop.ID == "" {
		stop.ID = plan.NewStopID()
	}
	stop.SortOrder = len(p.Stops)
	p.AddStop(stop)

	if len(p.Stops) >= 2 {
		_ = s.reroute(p) // non-fatal
	}

	if err := s.repo.Update(p); err != nil {
		return nil, fmt.Errorf("persist plan after AddStop: %w", err)
	}
	return p, nil
}

// RemoveStop removes a stop from the plan, renumbers sort_order, and triggers re-route.
func (s *PlanService) RemoveStop(planID, stopID string) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(planID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("load plan for RemoveStop: %w", err)
	}

	p.RemoveStop(stopID)
	renumberStops(p)

	if len(p.Stops) >= 2 {
		_ = s.reroute(p) // non-fatal
	}

	if err := s.repo.Update(p); err != nil {
		return nil, fmt.Errorf("persist plan after RemoveStop: %w", err)
	}
	return p, nil
}

// AddTask appends a task, resolves its hashtag if present, and persists.
func (s *PlanService) AddTask(req AddTaskRequest) (*plan.RoutePlan, error) {
	p, err := s.repo.GetByID(req.PlanID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("load plan for AddTask: %w", err)
	}

	t := plan.PlanTask{
		ID:          plan.NewTaskID(),
		Description: req.Description,
		Hashtag:     req.Hashtag,
		Status:      plan.TaskUnresolved,
	}

	if p.RouteWKT != "" {
		t, _ = s.resolution.ResolveTask(t, p.RouteWKT) // resolution failure is non-fatal
	}

	p.AddTask(t)

	if err := s.repo.Update(p); err != nil {
		return nil, fmt.Errorf("persist plan after AddTask: %w", err)
	}
	return p, nil
}

// --- internal helpers ---

// reroute calls Valhalla and stores the computed WKT on the plan.
func (s *PlanService) reroute(p *plan.RoutePlan) error {
	result, err := s.routing.GetDirections(DirectionsRequest{
		Stops:       p.Stops,
		DepartureAt: p.DepartureAt,
		SpeedModel:  p.SpeedModel,
		Preferences: p.Preferences,
	})
	if err != nil {
		return err
	}
	p.RouteWKT = geoJSONToWKT(result.GeoJSON)
	return nil
}

// geoJSONToWKT converts a GeoJSONLineString to a LINESTRING WKT string (SRID 4326).
// Format: LINESTRING(lon lat, lon lat, ...)
func geoJSONToWKT(g GeoJSONLineString) string {
	if len(g.Coordinates) == 0 {
		return ""
	}
	pts := make([]string, len(g.Coordinates))
	for i, c := range g.Coordinates {
		pts[i] = fmt.Sprintf("%f %f", c[0], c[1])
	}
	return "LINESTRING(" + strings.Join(pts, ", ") + ")"
}

// renumberStops reassigns SortOrder 0..N-1 after a stop is removed.
func renumberStops(p *plan.RoutePlan) {
	for i := range p.Stops {
		p.Stops[i].SortOrder = i
	}
}

// ErrPlanNotFound is returned when the requested plan does not exist.
var ErrPlanNotFound = errors.New("plan not found")
```

### ID generators for StopPoint and PlanTask

Add two package-level helpers to `internal/domain/plan/plan.go` (or a new `ids.go` file in the same package):

```go
// NewStopID generates a new random UUID for a StopPoint.
func NewStopID() string { return newID() }

// NewTaskID generates a new random UUID for a PlanTask.
func NewTaskID() string { return newID() }
```

---

## Step 4 — API: Plan Handler

**File:** `internal/api/plan_handler.go`

### Design notes

- The handler owns request decoding, validation, and HTTP status mapping.
- It delegates all business logic to `PlanService`.
- `ErrPlanNotFound` maps to 404.
- All stop/task IDs in the URL (`{stop_id}`) are extracted with `chi.URLParam`.
- The response for every mutating endpoint is the full updated plan (same shape as `GET /plans/:id`).

```go
// internal/api/plan_handler.go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/plan"
	"github.com/go-chi/chi/v5"
)

// PlanDirector is the interface the handler uses to manage plans.
// Accepting an interface keeps the handler testable.
type PlanDirector interface {
	CreatePlan(req app.CreatePlanRequest) (*plan.RoutePlan, error)
	GetPlan(id string) (*plan.RoutePlan, error)
	AddStop(req app.AddStopRequest) (*plan.RoutePlan, error)
	RemoveStop(planID, stopID string) (*plan.RoutePlan, error)
	AddTask(req app.AddTaskRequest) (*plan.RoutePlan, error)
}

// PlanHandler handles HTTP requests for the plan endpoints.
type PlanHandler struct {
	svc PlanDirector
}

// NewPlanHandler creates a PlanHandler backed by the given service.
func NewPlanHandler(svc PlanDirector) *PlanHandler {
	return &PlanHandler{svc: svc}
}

// --- Request / Response types ---

type createPlanRequest struct {
	UserID      string  `json:"user_id"`
	DepartureAt string  `json:"departure_at"`
	SpeedModel  string  `json:"speed_model"`
	ShadeWeight float64 `json:"shade_weight"`
	GreenWeight float64 `json:"greenery_weight"`
	WindWeight  float64 `json:"wind_weight"`
}

type addStopRequest struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Type string  `json:"type"`
}

type addTaskRequest struct {
	Description string `json:"description"`
	Hashtag     string `json:"hashtag,omitempty"`
}

type stopPointResponse struct {
	ID           string  `json:"id"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Type         string  `json:"type"`
	SortOrder    int     `json:"sort_order"`
	VenueID      string  `json:"venue_id,omitempty"`
	ResolvedName string  `json:"resolved_name,omitempty"`
}

type planTaskResponse struct {
	ID              string `json:"id"`
	Description     string `json:"description"`
	Hashtag         string `json:"hashtag,omitempty"`
	Status          string `json:"status"`
	ResolvedVenueID string `json:"resolved_venue_id,omitempty"`
}

type planResponse struct {
	ID          string             `json:"id"`
	UserID      string             `json:"user_id"`
	DepartureAt string             `json:"departure_at"`
	SpeedModel  string             `json:"speed_model"`
	Stops       []stopPointResponse `json:"stops"`
	Tasks       []planTaskResponse  `json:"tasks"`
	RouteWKT    string             `json:"route_wkt,omitempty"`
}

// --- Handler methods ---

// CreatePlan handles POST /api/v1/plans
func (h *PlanHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	departureAt := time.Now()
	if req.DepartureAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.DepartureAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339")
			return
		}
		departureAt = parsed
	}

	speedModel := plan.SpeedModelElevation
	if req.SpeedModel == string(plan.SpeedModelFlat) {
		speedModel = plan.SpeedModelFlat
	}

	p, err := h.svc.CreatePlan(app.CreatePlanRequest{
		UserID:      req.UserID,
		DepartureAt: departureAt,
		SpeedModel:  speedModel,
		Preferences: plan.Preferences{
			ShadeWeight:    req.ShadeWeight,
			GreeneryWeight: req.GreenWeight,
			WindWeight:     req.WindWeight,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create plan")
		return
	}

	writeJSON(w, http.StatusCreated, toPlanResponse(p))
}

// CreatePlanFromRoute handles POST /api/v1/routes/:id/plans
func (h *PlanHandler) CreatePlanFromRoute(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route id is required")
		return
	}

	var req createPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	departureAt := time.Now()
	if req.DepartureAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.DepartureAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339")
			return
		}
		departureAt = parsed
	}

	speedModel := plan.SpeedModelElevation
	if req.SpeedModel == string(plan.SpeedModelFlat) {
		speedModel = plan.SpeedModelFlat
	}

	p, err := h.svc.CreatePlan(app.CreatePlanRequest{
		UserID:        req.UserID,
		DepartureAt:   departureAt,
		SpeedModel:    speedModel,
		SourceRouteID: routeID,
		Preferences: plan.Preferences{
			ShadeWeight:    req.ShadeWeight,
			GreeneryWeight: req.GreenWeight,
			WindWeight:     req.WindWeight,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create plan from route")
		return
	}

	writeJSON(w, http.StatusCreated, toPlanResponse(p))
}

// GetPlan handles GET /api/v1/plans/:id
func (h *PlanHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetPlan(id)
	if err != nil {
		if errors.Is(err, app.ErrPlanNotFound) {
			writeError(w, http.StatusNotFound, "plan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get plan")
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(p))
}

// AddStop handles POST /api/v1/plans/:id/stops
func (h *PlanHandler) AddStop(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "id")
	var req addStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Lat == 0 && req.Lon == 0 {
		writeError(w, http.StatusBadRequest, "lat and lon are required")
		return
	}

	stopType := plan.StopManual
	if req.Type != "" {
		stopType = plan.StopType(req.Type)
	}

	p, err := h.svc.AddStop(app.AddStopRequest{
		PlanID: planID,
		Stop: plan.StopPoint{
			Lat:  req.Lat,
			Lon:  req.Lon,
			Type: stopType,
		},
	})
	if err != nil {
		if errors.Is(err, app.ErrPlanNotFound) {
			writeError(w, http.StatusNotFound, "plan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add stop")
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(p))
}

// RemoveStop handles DELETE /api/v1/plans/:id/stops/:stop_id
func (h *PlanHandler) RemoveStop(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "id")
	stopID := chi.URLParam(r, "stop_id")

	p, err := h.svc.RemoveStop(planID, stopID)
	if err != nil {
		if errors.Is(err, app.ErrPlanNotFound) {
			writeError(w, http.StatusNotFound, "plan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to remove stop")
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(p))
}

// AddTask handles POST /api/v1/plans/:id/tasks
func (h *PlanHandler) AddTask(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "id")
	var req addTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}

	p, err := h.svc.AddTask(app.AddTaskRequest{
		PlanID:      planID,
		Description: req.Description,
		Hashtag:     req.Hashtag,
	})
	if err != nil {
		if errors.Is(err, app.ErrPlanNotFound) {
			writeError(w, http.StatusNotFound, "plan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add task")
		return
	}
	writeJSON(w, http.StatusOK, toPlanResponse(p))
}

// --- response helper ---

func toPlanResponse(p *plan.RoutePlan) planResponse {
	stops := make([]stopPointResponse, len(p.Stops))
	for i, s := range p.Stops {
		stops[i] = stopPointResponse{
			ID:           s.ID,
			Lat:          s.Lat,
			Lon:          s.Lon,
			Type:         string(s.Type),
			SortOrder:    s.SortOrder,
			VenueID:      s.VenueID,
			ResolvedName: s.ResolvedName,
		}
	}
	tasks := make([]planTaskResponse, len(p.Tasks))
	for i, t := range p.Tasks {
		tasks[i] = planTaskResponse{
			ID:              t.ID,
			Description:     t.Description,
			Hashtag:         t.Hashtag,
			Status:          string(t.Status),
			ResolvedVenueID: t.ResolvedVenueID,
		}
	}
	return planResponse{
		ID:          p.ID,
		UserID:      p.UserID,
		DepartureAt: p.DepartureAt.Format(time.RFC3339),
		SpeedModel:  string(p.SpeedModel),
		Stops:       stops,
		Tasks:       tasks,
		RouteWKT:    p.RouteWKT,
	}
}
```

---

## Step 5 — Wire: Router and main.go

### `internal/api/router.go`

Add `planH *PlanHandler` parameter and register the six routes:

```go
func NewRouter(
	routeSvc *app.RouteService,
	discoverySvc *app.DiscoveryService,
	venueSvc *app.VenueService,
	routingH *RoutingHandler,
	weatherH *WeatherHandler,
	conditionsH *ConditionsHandler,
	previewH *PreviewHandler,
	planH *PlanHandler,          // NEW
) *chi.Mux {
	// ... existing setup unchanged ...

	r.Route("/api/v1", func(r chi.Router) {
		// ... existing routes unchanged ...

		// Plans
		r.Post("/plans", planH.CreatePlan)
		r.Get("/plans/{id}", planH.GetPlan)
		r.Post("/plans/{id}/stops", planH.AddStop)
		r.Post("/plans/{id}/tasks", planH.AddTask)
		r.Delete("/plans/{id}/stops/{stop_id}", planH.RemoveStop)
		r.Post("/routes/{id}/plans", planH.CreatePlanFromRoute)
	})
	return r
}
```

### `cmd/api/main.go`

Add three new dependency constructions after the existing `routingSvc` line, then pass `planHandler` to `NewRouter`:

```go
// Plan dependencies
planRepo := postgres.NewPlanRepo(pool)
venueResolutionSvc := app.NewVenueResolutionService(venueRepo, venueRepo)
planSvc := app.NewPlanService(planRepo, routeRepo, routingSvc, venueResolutionSvc)
planHandler := api.NewPlanHandler(planSvc)

router := api.NewRouter(
	routeSvc, discoverySvc, venueSvc,
	routingHandler, weatherHandler, conditionsHandler, previewHandler,
	planHandler,   // NEW
)
```

Note: `venueRepo` satisfies both `VenueResolver` and `VenueTagLookup` because `*postgres.VenueRepo` will implement all three methods (`ListTags`, `NearestAlongLine`, `GetTagMapping`) after Step 2.

---

## Step 6 — Integration Tests: plan_repo_test.go

**File:** `internal/infra/postgres/plan_repo_test.go`

Uses `newTestPool(t)` (defined in `discovery_repo_test.go`) and `TEST_DB_DSN` env var — matches all other integration test files.

```go
// internal/infra/postgres/plan_repo_test.go
package postgres_test

import (
	"testing"
	"time"

	"komorebi/internal/domain/plan"
	"komorebi/internal/infra/postgres"
)

func samplePlan(t *testing.T) *plan.RoutePlan {
	t.Helper()
	p := plan.NewRoutePlan("00000000-0000-0000-0000-000000000001")
	p.DepartureAt = time.Now().UTC().Truncate(time.Second)
	p.SpeedModel = plan.SpeedModelElevation
	p.Preferences = plan.Preferences{ShadeWeight: 0.5, GreeneryWeight: 0.3, WindWeight: 0.2}
	p.AddStop(plan.StopPoint{
		ID: plan.NewStopID(), Lat: 35.6762, Lon: 139.6503,
		Type: plan.StopManual, SortOrder: 0, ResolvedName: "Origin",
	})
	p.AddStop(plan.StopPoint{
		ID: plan.NewStopID(), Lat: 35.6895, Lon: 139.6917,
		Type: plan.StopManual, SortOrder: 1, ResolvedName: "Destination",
	})
	return p
}

func TestPlanRepo_CreateAndGetByID(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, p.ID)
	}
	if len(got.Stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(got.Stops))
	}
}

func TestPlanRepo_GetByID_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	_, err := repo.GetByID("00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestPlanRepo_Update_ReplaceStops(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	// Add a third stop and update.
	p.AddStop(plan.StopPoint{
		ID: plan.NewStopID(), Lat: 35.700, Lon: 139.700,
		Type: plan.StopManual, SortOrder: 2,
	})
	if err := repo.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID after Update: %v", err)
	}
	if len(got.Stops) != 3 {
		t.Errorf("expected 3 stops after update, got %d", len(got.Stops))
	}
}

func TestPlanRepo_Update_WithTask(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	p.AddTask(plan.PlanTask{
		ID:          plan.NewTaskID(),
		Description: "Buy coffee at #cafe",
		Hashtag:     "#cafe",
		Status:      plan.TaskUnresolved,
	})
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(p.ID) })

	got, err := repo.GetByID(p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(got.Tasks))
	}
	if got.Tasks[0].Hashtag != "#cafe" {
		t.Errorf("expected hashtag #cafe, got %q", got.Tasks[0].Hashtag)
	}
	if got.Tasks[0].Status != plan.TaskUnresolved {
		t.Errorf("expected status unresolved, got %q", got.Tasks[0].Status)
	}
}

func TestPlanRepo_Delete(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewPlanRepo(pool)

	p := samplePlan(t)
	if err := repo.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByID(p.ID)
	if err == nil {
		t.Fatal("expected ErrNotFound after delete, got nil")
	}
}
```

### Unit tests: venue_resolution_service_test.go

```go
// internal/app/venue_resolution_service_test.go
package app_test

import (
	"testing"

	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/plan"
)

// stubVenueResolver satisfies app.VenueResolver for tests.
type stubVenueResolver struct {
	venue *environment.Venue
	err   error
}

func (s *stubVenueResolver) ListTags() ([]environment.VenueTag, error) { return nil, nil }
func (s *stubVenueResolver) NearestAlongLine(_ environment.NearestAlongLineParams) (*environment.Venue, error) {
	return s.venue, s.err
}

// stubVenueTagLookup satisfies app.VenueTagLookup for tests.
type stubVenueTagLookup struct {
	mapping *environment.VenueTagMapping
	err     error
}

func (s *stubVenueTagLookup) GetTagMapping(_ string) (*environment.VenueTagMapping, error) {
	return s.mapping, s.err
}

func TestVenueResolutionService_ResolveTask_MatchFound(t *testing.T) {
	resolver := &stubVenueResolver{
		venue: &environment.Venue{ID: "v-cafe-1", Name: "Blue Bottle", Category: "cafe"},
	}
	lookup := &stubVenueTagLookup{
		mapping: &environment.VenueTagMapping{
			Hashtag:   "#cafe",
			OSMFilter: map[string]string{"amenity": "cafe"},
			IsBrand:   false,
		},
	}
	svc := app.NewVenueResolutionService(resolver, lookup)

	task := plan.PlanTask{
		ID:          "t1",
		Description: "Grab coffee at #cafe",
		Status:      plan.TaskUnresolved,
	}
	const routeWKT = "LINESTRING(139.65 35.67, 139.70 35.69)"
	result, err := svc.ResolveTask(task, routeWKT)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != plan.TaskMatched {
		t.Errorf("expected status matched, got %q", result.Status)
	}
	if result.ResolvedVenueID != "v-cafe-1" {
		t.Errorf("expected venue v-cafe-1, got %q", result.ResolvedVenueID)
	}
	if result.Hashtag != "#cafe" {
		t.Errorf("expected hashtag #cafe, got %q", result.Hashtag)
	}
}

func TestVenueResolutionService_ResolveTask_NoVenueFound(t *testing.T) {
	resolver := &stubVenueResolver{venue: nil}
	lookup := &stubVenueTagLookup{
		mapping: &environment.VenueTagMapping{
			Hashtag:   "#bike-shop",
			OSMFilter: map[string]string{"shop": "bicycle"},
		},
	}
	svc := app.NewVenueResolutionService(resolver, lookup)

	task := plan.PlanTask{ID: "t2", Description: "Find #bike-shop", Status: plan.TaskUnresolved}
	result, err := svc.ResolveTask(task, "LINESTRING(139.65 35.67, 139.70 35.69)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved when no venue found, got %q", result.Status)
	}
}

func TestVenueResolutionService_ResolveTask_UnknownHashtag(t *testing.T) {
	resolver := &stubVenueResolver{}
	lookup := &stubVenueTagLookup{mapping: nil} // nil = unknown hashtag
	svc := app.NewVenueResolutionService(resolver, lookup)

	task := plan.PlanTask{ID: "t3", Description: "Stop at #unknown-tag", Status: plan.TaskUnresolved}
	result, err := svc.ResolveTask(task, "LINESTRING(139.65 35.67, 139.70 35.69)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved for unknown hashtag, got %q", result.Status)
	}
}

func TestVenueResolutionService_ResolveTask_NoHashtag(t *testing.T) {
	resolver := &stubVenueResolver{}
	lookup := &stubVenueTagLookup{}
	svc := app.NewVenueResolutionService(resolver, lookup)

	task := plan.PlanTask{ID: "t4", Description: "Just a note with no tag", Status: plan.TaskUnresolved}
	result, err := svc.ResolveTask(task, "LINESTRING(139.65 35.67, 139.70 35.69)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != plan.TaskUnresolved {
		t.Errorf("expected unresolved for no hashtag, got %q", result.Status)
	}
}
```

### Unit tests: plan_service_test.go

```go
// internal/app/plan_service_test.go
package app_test

import (
	"errors"
	"testing"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/plan"
	routedomain "komorebi/internal/domain/route"
)

// --- stubs ---

type stubPlanRepo struct {
	plans map[string]*plan.RoutePlan
	err   error
}

func newStubPlanRepo() *stubPlanRepo {
	return &stubPlanRepo{plans: map[string]*plan.RoutePlan{}}
}

func (s *stubPlanRepo) Create(p *plan.RoutePlan) error {
	if s.err != nil {
		return s.err
	}
	copy := *p
	s.plans[p.ID] = &copy
	return nil
}
func (s *stubPlanRepo) GetByID(id string) (*plan.RoutePlan, error) {
	if s.err != nil {
		return nil, s.err
	}
	p, ok := s.plans[id]
	if !ok {
		return nil, postgres.ErrNotFound
	}
	copy := *p
	return &copy, nil
}
func (s *stubPlanRepo) Update(p *plan.RoutePlan) error {
	if s.err != nil {
		return s.err
	}
	copy := *p
	s.plans[p.ID] = &copy
	return nil
}
func (s *stubPlanRepo) Delete(id string) error {
	delete(s.plans, id)
	return nil
}

type stubRouteReader struct {
	route *routedomain.Route
	err   error
}

func (s *stubRouteReader) GetByID(_ string) (*routedomain.Route, error) {
	return s.route, s.err
}

type noopResolutionService struct{}

func (n *noopResolutionService) ResolveTask(t plan.PlanTask, _ string) (plan.PlanTask, error) {
	return t, nil
}

// noopRoutingService satisfies ValhallaRouter for testing
type noopRouter struct{}

func (n *noopRouter) Route(_ []valhalla.Location) (*valhalla.RouteResult, error) {
	return &valhalla.RouteResult{
		TotalDistanceKm: 5.0,
		TotalDurationS:  1200,
		Legs: []valhalla.Leg{
			{DistanceKm: 5.0, DurationS: 1200, Shape: [][2]float64{{139.65, 35.67}, {139.70, 35.69}}},
		},
	}, nil
}

// --- tests ---

func TestPlanService_CreatePlan_Empty(t *testing.T) {
	repo := newStubPlanRepo()
	routeReader := &stubRouteReader{}
	routingSvc := app.NewRoutingService(&noopRouter{})
	resolveSvc := app.NewVenueResolutionService(&stubVenueResolver{}, &stubVenueTagLookup{})
	svc := app.NewPlanService(repo, routeReader, routingSvc, resolveSvc)

	p, err := svc.CreatePlan(app.CreatePlanRequest{
		UserID:      "user-1",
		DepartureAt: time.Now(),
		SpeedModel:  plan.SpeedModelElevation,
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if p.ID == "" {
		t.Error("expected non-empty plan ID")
	}
	if p.UserID != "user-1" {
		t.Errorf("expected userID user-1, got %q", p.UserID)
	}
}

func TestPlanService_AddStop_TriggersReroute(t *testing.T) {
	repo := newStubPlanRepo()
	routingSvc := app.NewRoutingService(&noopRouter{})
	resolveSvc := app.NewVenueResolutionService(&stubVenueResolver{}, &stubVenueTagLookup{})
	svc := app.NewPlanService(repo, &stubRouteReader{}, routingSvc, resolveSvc)

	p, _ := svc.CreatePlan(app.CreatePlanRequest{UserID: "u1", DepartureAt: time.Now(), SpeedModel: plan.SpeedModelFlat})

	// Add two stops; the second triggers reroute.
	p, err := svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.67, Lon: 139.65, Type: plan.StopManual}})
	if err != nil {
		t.Fatalf("AddStop #1: %v", err)
	}
	p, err = svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.69, Lon: 139.70, Type: plan.StopManual}})
	if err != nil {
		t.Fatalf("AddStop #2: %v", err)
	}
	if p.RouteWKT == "" {
		t.Error("expected RouteWKT populated after two stops and reroute")
	}
	if len(p.Stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(p.Stops))
	}
}

func TestPlanService_RemoveStop(t *testing.T) {
	repo := newStubPlanRepo()
	routingSvc := app.NewRoutingService(&noopRouter{})
	resolveSvc := app.NewVenueResolutionService(&stubVenueResolver{}, &stubVenueTagLookup{})
	svc := app.NewPlanService(repo, &stubRouteReader{}, routingSvc, resolveSvc)

	p, _ := svc.CreatePlan(app.CreatePlanRequest{UserID: "u1", DepartureAt: time.Now(), SpeedModel: plan.SpeedModelFlat})
	p, _ = svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.67, Lon: 139.65, Type: plan.StopManual}})
	p, _ = svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.69, Lon: 139.70, Type: plan.StopManual}})
	p, _ = svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.71, Lon: 139.72, Type: plan.StopManual}})

	thirdStopID := p.Stops[2].ID
	p, err := svc.RemoveStop(p.ID, thirdStopID)
	if err != nil {
		t.Fatalf("RemoveStop: %v", err)
	}
	if len(p.Stops) != 2 {
		t.Errorf("expected 2 stops after remove, got %d", len(p.Stops))
	}
}

func TestPlanService_AddTask_ResolvesHashtag(t *testing.T) {
	repo := newStubPlanRepo()
	routingSvc := app.NewRoutingService(&noopRouter{})

	// Resolver returns a venue match.
	resolver := &stubVenueResolver{
		venue: &environment.Venue{ID: "v-1", Name: "FamilyMart"},
	}
	lookup := &stubVenueTagLookup{
		mapping: &environment.VenueTagMapping{
			Hashtag:   "#konbini",
			OSMFilter: map[string]string{"shop": "convenience"},
		},
	}
	resolveSvc := app.NewVenueResolutionService(resolver, lookup)
	svc := app.NewPlanService(repo, &stubRouteReader{}, routingSvc, resolveSvc)

	p, _ := svc.CreatePlan(app.CreatePlanRequest{UserID: "u1", DepartureAt: time.Now(), SpeedModel: plan.SpeedModelFlat})
	p, _ = svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.67, Lon: 139.65, Type: plan.StopManual}})
	p, _ = svc.AddStop(app.AddStopRequest{PlanID: p.ID, Stop: plan.StopPoint{Lat: 35.69, Lon: 139.70, Type: plan.StopManual}})

	p, err := svc.AddTask(app.AddTaskRequest{
		PlanID:      p.ID,
		Description: "Buy snacks at #konbini",
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if len(p.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(p.Tasks))
	}
	if p.Tasks[0].Status != plan.TaskMatched {
		t.Errorf("expected task matched, got %q", p.Tasks[0].Status)
	}
}

func TestPlanService_GetPlan_NotFound(t *testing.T) {
	repo := newStubPlanRepo()
	svc := app.NewPlanService(repo, &stubRouteReader{}, app.NewRoutingService(&noopRouter{}),
		app.NewVenueResolutionService(&stubVenueResolver{}, &stubVenueTagLookup{}))

	_, err := svc.GetPlan("non-existent-id")
	if !errors.Is(err, app.ErrPlanNotFound) {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}
```

### Handler tests: plan_handler_test.go

```go
// internal/api/plan_handler_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/domain/plan"
	"github.com/go-chi/chi/v5"
)

// stubPlanDirector satisfies api.PlanDirector for tests.
type stubPlanDirector struct {
	plan *plan.RoutePlan
	err  error
}

func (s *stubPlanDirector) CreatePlan(_ app.CreatePlanRequest) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) GetPlan(_ string) (*plan.RoutePlan, error) { return s.plan, s.err }
func (s *stubPlanDirector) AddStop(_ app.AddStopRequest) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) RemoveStop(_, _ string) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) AddTask(_ app.AddTaskRequest) (*plan.RoutePlan, error) {
	return s.plan, s.err
}

func samplePlanDomain() *plan.RoutePlan {
	p := plan.NewRoutePlan("user-99")
	p.DepartureAt = time.Now().UTC()
	p.SpeedModel = plan.SpeedModelElevation
	return p
}

func TestPlanHandler_CreatePlan_OK(t *testing.T) {
	stub := &stubPlanDirector{plan: samplePlanDomain()}
	h := api.NewPlanHandler(stub)

	body := `{"user_id":"user-99","departure_at":"2026-04-10T09:00:00Z","speed_model":"elevation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CreatePlan(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["id"] == "" {
		t.Error("expected non-empty id in response")
	}
}

func TestPlanHandler_CreatePlan_MissingUserID(t *testing.T) {
	h := api.NewPlanHandler(&stubPlanDirector{plan: samplePlanDomain()})
	body := `{"departure_at":"2026-04-10T09:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.CreatePlan(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPlanHandler_GetPlan_NotFound(t *testing.T) {
	stub := &stubPlanDirector{err: app.ErrPlanNotFound}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Get("/api/v1/plans/{id}", h.GetPlan)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans/does-not-exist", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestPlanHandler_AddStop_OK(t *testing.T) {
	p := samplePlanDomain()
	p.AddStop(plan.StopPoint{ID: "s1", Lat: 35.67, Lon: 139.65, Type: plan.StopManual})
	stub := &stubPlanDirector{plan: p}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Post("/api/v1/plans/{id}/stops", h.AddStop)

	body := `{"lat":35.67,"lon":139.65,"type":"manual"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans/plan-1/stops", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlanHandler_RemoveStop_OK(t *testing.T) {
	stub := &stubPlanDirector{plan: samplePlanDomain()}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Delete("/api/v1/plans/{id}/stops/{stop_id}", h.RemoveStop)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/plans/plan-1/stops/stop-1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlanHandler_AddTask_MissingDescription(t *testing.T) {
	h := api.NewPlanHandler(&stubPlanDirector{plan: samplePlanDomain()})

	r := chi.NewRouter()
	r.Post("/api/v1/plans/{id}/tasks", h.AddTask)
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans/plan-1/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
```

---

## Step 7 — Self-Review

Before marking this plan complete, verify:

- [ ] `plan.route_plan` has a `route_wkt TEXT` column — if missing, write a migration `000NNN_plan_route_wkt.up.sql` adding `ALTER TABLE plan.route_plan ADD COLUMN IF NOT EXISTS route_wkt TEXT;`.
- [ ] `plan.stop_type` enum values in Postgres match `manual`, `venue_resolved`, `waypoint` exactly.
- [ ] `plan.task_status` enum values in Postgres match `unresolved`, `matched`, `completed` exactly.
- [ ] `plan.speed_model` enum values match `elevation`, `flat`.
- [ ] `environment.venue_tag_mapping.osm_filter` column is JSONB (not JSON) — the `GetTagMapping` query scans it as `[]byte` which works for both.
- [ ] All new methods added to `VenueRepository` interface (`GetTagMapping`, `NearestAlongLine`) are implemented on `*postgres.VenueRepo` before compiling.
- [ ] The `stubPlanRepo` in `plan_service_test.go` imports `postgres` package for `ErrNotFound` — alternatively, wrap with a local `ErrNotFound` sentinel in the plan domain or re-use `app.ErrPlanNotFound` from a check inside the stub. The cleanest approach is to make `stubPlanRepo.GetByID` return `app.ErrPlanNotFound` directly (no postgres import needed in the test).
- [ ] `plan_service_test.go` imports `valhalla` for `noopRouter` — add `"komorebi/internal/infra/valhalla"` to imports.
- [ ] `venue_resolution_service.go` imports `"fmt"` (noted inline).
- [ ] Run `go build ./...` — zero compile errors.
- [ ] Run `go test ./internal/domain/plan/... ./internal/app/... ./internal/api/...` — all pass.
- [ ] Run `go test ./internal/infra/postgres/... -run TestPlanRepo` with `TEST_DB_DSN` set — all pass against live DB.

---

## Migration Note

If `route_wkt` does not exist on `plan.route_plan`, create:

```sql
-- migrations/000NNN_plan_route_wkt.up.sql
ALTER TABLE plan.route_plan ADD COLUMN IF NOT EXISTS route_wkt TEXT;
```

```sql
-- migrations/000NNN_plan_route_wkt.down.sql
ALTER TABLE plan.route_plan DROP COLUMN IF EXISTS route_wkt;
```

Check the highest existing migration number in `migrations/` and use the next sequential number.
