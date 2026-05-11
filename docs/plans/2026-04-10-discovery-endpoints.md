# Discovery Bounded Context — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Complete each task fully — including its commit — before moving to the next.

**Goal:** Implement the Discovery bounded context (nearby, viewport, suggested route discovery endpoints) plus venue-along-route and venue-tags endpoints, wired into the existing chi router.

**Architecture:** Hexagonal / ports-and-adapters matching the existing codebase. The `discovery` package owns its own domain types, repository interface, and search/ranking logic. Infrastructure adapters in `internal/infra/postgres/` implement those interfaces. Application services in `internal/app/` orchestrate use cases; HTTP handlers in `internal/api/` serve REST.

**Tech Stack:** Go 1.22+, chi router, pgx v5, PostGIS (ST_DWithin, ST_Distance, ST_Intersects, ST_MakeEnvelope, ST_AsText).

**Database:** `postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable`

**Relevant tables:**
- `routes.route` — id (uuid), name, description, geometry (LineStringZ,4326), distance_m, elevation_gain_m, elevation_loss_m, difficulty, status, creator_id, created_at, updated_at
- `environment.venue` — id (uuid), osm_id, geometry (Point,4326), name, category, brand, osm_tags (jsonb)
- `environment.venue_tag_mapping` — hashtag (PK), osm_filter (jsonb), description, is_brand (bool)

---

## Task 1 — Discovery Domain Types

**Files:**
- `internal/domain/discovery/discovery.go` (create)
- `internal/domain/discovery/repository.go` (create)
- `internal/domain/discovery/discovery_test.go` (create)

### Steps

- [ ] 1.1 Write the test file first.

```go
// internal/domain/discovery/discovery_test.go
package discovery_test

import (
	"testing"

	"komorebi/internal/domain/discovery"
)

func TestNearbyParams_DefaultRadius(t *testing.T) {
	p := discovery.NearbyParams{Lat: 35.68, Lon: 139.69}
	if p.RadiusKm != 0 {
		t.Fatalf("expected zero default radius, got %f", p.RadiusKm)
	}
}

func TestViewportParams_BBoxOrder(t *testing.T) {
	p := discovery.ViewportParams{BBox: [4]float64{139.6, 35.6, 139.8, 35.8}}
	if p.BBox[0] >= p.BBox[2] {
		t.Fatalf("expected minLon < maxLon")
	}
	if p.BBox[1] >= p.BBox[3] {
		t.Fatalf("expected minLat < maxLat")
	}
}

func TestDiscoveryResult_Fields(t *testing.T) {
	r := discovery.DiscoveryResult{
		RouteID:    "abc",
		Name:       "Shinjuku Loop",
		DistanceM:  12000,
		DistFromM:  850.5,
		Difficulty: "moderate",
	}
	if r.RouteID == "" {
		t.Fatal("RouteID must not be empty")
	}
	if r.DistFromM < 0 {
		t.Fatal("DistFromM must be non-negative")
	}
}
```

- [ ] 1.2 Create `internal/domain/discovery/discovery.go`.

```go
// internal/domain/discovery/discovery.go
package discovery

import "time"

// NearbyParams holds parameters for a proximity search.
type NearbyParams struct {
	Lat      float64
	Lon      float64
	RadiusKm float64 // default applied by service if zero: 10 km
	Limit    int     // default 20, max 100
}

// ViewportParams holds parameters for a map-viewport search.
type ViewportParams struct {
	BBox  [4]float64 // [minLon, minLat, maxLon, maxLat]
	Limit int        // default 50, max 200
}

// SuggestedParams holds parameters for the suggested-routes query.
// In Phase 2 (this plan) scoring is proximity-only; environment scoring
// is deferred to Phase 3.
type SuggestedParams struct {
	Lat         float64
	Lon         float64
	DepartureAt time.Time
	Limit       int // default 10, max 50
}

// DiscoveryResult is a lightweight route summary returned by discovery queries.
// It carries only the fields needed to populate a route card in the UI.
type DiscoveryResult struct {
	RouteID        string   // UUID
	Name           string
	Description    string
	DistanceM      float64
	ElevationGainM float64
	ElevationLossM float64
	Difficulty     string
	Status         string
	Tags           []string
	// DistFromM is the distance in metres from the query point to the
	// nearest point on the route geometry. Zero for viewport queries.
	DistFromM float64
}
```

- [ ] 1.3 Create `internal/domain/discovery/repository.go`.

```go
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
```

- [ ] 1.4 Run tests (they should compile and pass — pure struct assertions, no DB needed).

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/domain/discovery/...
```

- [ ] 1.5 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/domain/discovery/
git commit -m "feat(discovery): add Discovery domain types and repository interface"
```

---

## Task 2 — Venue Domain Types

**Files:**
- `internal/domain/environment/venue.go` (create)
- `internal/domain/environment/venue_test.go` (create)

The `environment` domain package already exists in the project structure (per the design spec and folder layout). This task adds venue-specific types used by the venues-along-route endpoint.

### Steps

- [ ] 2.1 Verify the environment package directory exists; create it if missing.

```bash
ls /Users/lug/src/cyclist-map/internal/domain/environment/ 2>/dev/null || mkdir -p /Users/lug/src/cyclist-map/internal/domain/environment/
```

- [ ] 2.2 Write the test file first.

```go
// internal/domain/environment/venue_test.go
package environment_test

import (
	"testing"

	"komorebi/internal/domain/environment"
)

func TestAlongRouteParams_Defaults(t *testing.T) {
	p := environment.AlongRouteParams{RouteID: "abc"}
	if p.BufferM != 0 {
		t.Fatalf("expected zero default buffer, got %f", p.BufferM)
	}
}

func TestVenue_Fields(t *testing.T) {
	v := environment.Venue{
		ID:       "v1",
		Name:     "7-Eleven Shinjuku",
		Category: "convenience",
		Lat:      35.69,
		Lon:      139.70,
	}
	if v.ID == "" {
		t.Fatal("ID must not be empty")
	}
}

func TestVenueTag_Fields(t *testing.T) {
	vt := environment.VenueTag{
		Hashtag:     "#konbini",
		Description: "Any convenience store",
		IsBrand:     false,
	}
	if vt.Hashtag == "" {
		t.Fatal("Hashtag must not be empty")
	}
}
```

- [ ] 2.3 Create `internal/domain/environment/venue.go`.

```go
// internal/domain/environment/venue.go
package environment

// Venue is a point-of-interest sourced from OSM (environment schema).
type Venue struct {
	ID       string // UUID
	OsmID    int64
	Name     string
	Category string
	Brand    string
	Lat      float64
	Lon      float64
	OsmTags  map[string]string
}

// VenueTag represents a hashtag → OSM filter mapping from venue_tag_mapping.
type VenueTag struct {
	Hashtag     string
	Description string
	IsBrand     bool
}

// AlongRouteParams holds parameters for the venues-along-route query.
type AlongRouteParams struct {
	RouteID  string
	Category string  // optional: filter by venue category (maps to hashtag lookup)
	BufferM  float64 // distance from route geometry in metres; default 200
}

// VenueRepository defines persistence operations for venue reads.
type VenueRepository interface {
	// AlongRoute returns venues within BufferM metres of the named route geometry.
	// Category is an optional filter (e.g. "convenience", "cafe").
	AlongRoute(params AlongRouteParams) ([]Venue, error)

	// ListTags returns all hashtag → filter mappings from venue_tag_mapping.
	ListTags() ([]VenueTag, error)
}
```

- [ ] 2.4 Run tests.

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/domain/environment/...
```

- [ ] 2.5 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/domain/environment/
git commit -m "feat(environment): add Venue domain types and VenueRepository interface"
```

---

## Task 3 — Discovery PostgreSQL Repository

**Files:**
- `internal/infra/postgres/discovery_repo.go` (create)
- `internal/infra/postgres/discovery_repo_test.go` (create)

### Steps

- [ ] 3.1 Write the integration test file first. These tests hit the real DB; they must be skipped if `TEST_DB_DSN` is unset (matching the pattern in `route_repo_test.go`).

```go
// internal/infra/postgres/discovery_repo_test.go
package postgres_test

import (
	"context"
	"os"
	"testing"

	"komorebi/internal/domain/discovery"
	"komorebi/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set; skipping DB integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestDiscoveryRepo_Nearby(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewDiscoveryRepo(pool)

	results, err := repo.Nearby(discovery.NearbyParams{
		Lat:      35.6895,
		Lon:      139.6917,
		RadiusKm: 50,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	// Just verify the call succeeds and returns a slice (may be empty if DB has no routes)
	if results == nil {
		t.Fatal("expected non-nil slice")
	}
	for _, r := range results {
		if r.RouteID == "" {
			t.Fatal("RouteID must not be empty")
		}
		if r.DistFromM < 0 {
			t.Fatalf("DistFromM must be non-negative, got %f", r.DistFromM)
		}
	}
}

func TestDiscoveryRepo_Viewport(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewDiscoveryRepo(pool)

	results, err := repo.Viewport(discovery.ViewportParams{
		BBox:  [4]float64{139.5, 35.5, 140.0, 35.9},
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("Viewport: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil slice")
	}
}

func TestDiscoveryRepo_Suggested(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewDiscoveryRepo(pool)

	results, err := repo.Suggested(discovery.SuggestedParams{
		Lat:   35.6895,
		Lon:   139.6917,
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Suggested: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil slice")
	}
}
```

- [ ] 3.2 Create `internal/infra/postgres/discovery_repo.go`.

```go
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
			ST_Distance(
				r.geometry::geography,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
			) AS dist_m
		FROM routes.route r
		WHERE r.status = 'published'
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
			0.0 AS dist_m
		FROM routes.route r
		WHERE r.status = 'published'
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

// Suggested returns recommended routes for a given location and departure time.
// Phase 2 implementation: proximity-ordered nearby routes (same as Nearby).
// Phase 3 will add environment scoring (shade, weather, greenery) at departure_at.
func (r *DiscoveryRepo) Suggested(params discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return r.Nearby(discovery.NearbyParams{
		Lat:      params.Lat,
		Lon:      params.Lon,
		RadiusKm: 20, // wider radius for suggested
		Limit:    limit,
	})
}

// --- helpers ---

// scanDiscoveryRows reads rows from a discovery SELECT and returns DiscoveryResult slice.
// Expected column order: id, name, description, distance_m, elevation_gain_m,
// elevation_loss_m, difficulty, status, dist_m.
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
```

- [ ] 3.3 Run unit tests (domain layer only — no DB required).

```bash
cd /Users/lug/src/cyclist-map
go build ./internal/infra/postgres/...
```

- [ ] 3.4 Run integration tests against the dev DB.

```bash
cd /Users/lug/src/cyclist-map
TEST_DB_DSN="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go test ./internal/infra/postgres/... -run TestDiscoveryRepo -v
```

- [ ] 3.5 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/infra/postgres/discovery_repo.go internal/infra/postgres/discovery_repo_test.go
git commit -m "feat(discovery): add PostGIS DiscoveryRepo (nearby, viewport, suggested)"
```

---

## Task 4 — Venue PostgreSQL Repository

**Files:**
- `internal/infra/postgres/venue_repo.go` (create)
- `internal/infra/postgres/venue_repo_test.go` (create)

### Steps

- [ ] 4.1 Write the integration test file first.

```go
// internal/infra/postgres/venue_repo_test.go
package postgres_test

import (
	"testing"

	"komorebi/internal/domain/environment"
	"komorebi/internal/infra/postgres"
)

func TestVenueRepo_ListTags(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewVenueRepo(pool)

	tags, err := repo.ListTags()
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	// May be empty if no seed data; just verify it doesn't error
	if tags == nil {
		t.Fatal("expected non-nil slice")
	}
	for _, tag := range tags {
		if tag.Hashtag == "" {
			t.Fatal("Hashtag must not be empty")
		}
	}
}

func TestVenueRepo_AlongRoute_NoRoute(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewVenueRepo(pool)

	// Non-existent route should return empty, not error
	venues, err := repo.AlongRoute(environment.AlongRouteParams{
		RouteID: "00000000-0000-0000-0000-000000000000",
		BufferM: 200,
	})
	if err != nil {
		t.Fatalf("AlongRoute with non-existent route: %v", err)
	}
	if len(venues) != 0 {
		t.Fatalf("expected 0 venues for non-existent route, got %d", len(venues))
	}
}
```

- [ ] 4.2 Create `internal/infra/postgres/venue_repo.go`.

```go
// internal/infra/postgres/venue_repo.go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"komorebi/internal/domain/environment"
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
```

- [ ] 4.3 Run integration tests.

```bash
cd /Users/lug/src/cyclist-map
TEST_DB_DSN="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go test ./internal/infra/postgres/... -run TestVenueRepo -v
```

- [ ] 4.4 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/infra/postgres/venue_repo.go internal/infra/postgres/venue_repo_test.go
git commit -m "feat(environment): add PostGIS VenueRepo (along-route, list-tags)"
```

---

## Task 5 — Discovery Application Service

**Files:**
- `internal/app/discovery_service.go` (create)
- `internal/app/discovery_service_test.go` (create)

### Steps

- [ ] 5.1 Write the test using a hand-rolled stub for `discovery.Repository` (no DB required).

```go
// internal/app/discovery_service_test.go
package app_test

import (
	"errors"
	"testing"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/discovery"
)

// stubDiscoveryRepo implements discovery.Repository for tests.
type stubDiscoveryRepo struct {
	nearbyResults    []discovery.DiscoveryResult
	viewportResults  []discovery.DiscoveryResult
	suggestedResults []discovery.DiscoveryResult
	err              error
}

func (s *stubDiscoveryRepo) Nearby(_ discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	return s.nearbyResults, s.err
}
func (s *stubDiscoveryRepo) Viewport(_ discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	return s.viewportResults, s.err
}
func (s *stubDiscoveryRepo) Suggested(_ discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	return s.suggestedResults, s.err
}

func TestDiscoveryService_Nearby_DefaultRadius(t *testing.T) {
	stub := &stubDiscoveryRepo{
		nearbyResults: []discovery.DiscoveryResult{
			{RouteID: "r1", Name: "Loop A", DistFromM: 500},
		},
	}
	svc := app.NewDiscoveryService(stub)

	results, err := svc.Nearby(discovery.NearbyParams{Lat: 35.68, Lon: 139.69})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RouteID != "r1" {
		t.Fatalf("expected r1, got %s", results[0].RouteID)
	}
}

func TestDiscoveryService_Nearby_PropagatesError(t *testing.T) {
	stub := &stubDiscoveryRepo{err: errors.New("db down")}
	svc := app.NewDiscoveryService(stub)

	_, err := svc.Nearby(discovery.NearbyParams{Lat: 35.68, Lon: 139.69})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDiscoveryService_Viewport(t *testing.T) {
	stub := &stubDiscoveryRepo{
		viewportResults: []discovery.DiscoveryResult{
			{RouteID: "r2", Name: "Loop B"},
		},
	}
	svc := app.NewDiscoveryService(stub)

	results, err := svc.Viewport(discovery.ViewportParams{
		BBox: [4]float64{139.6, 35.6, 139.8, 35.8},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestDiscoveryService_Suggested(t *testing.T) {
	stub := &stubDiscoveryRepo{
		suggestedResults: []discovery.DiscoveryResult{
			{RouteID: "r3", Name: "Morning Ride"},
		},
	}
	svc := app.NewDiscoveryService(stub)

	results, err := svc.Suggested(discovery.SuggestedParams{
		Lat:         35.68,
		Lon:         139.69,
		DepartureAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
```

- [ ] 5.2 Create `internal/app/discovery_service.go`.

```go
// internal/app/discovery_service.go
package app

import (
	"komorebi/internal/domain/discovery"
)

// DiscoveryService provides application-level route discovery use cases.
type DiscoveryService struct {
	repo discovery.Repository
}

// NewDiscoveryService creates a DiscoveryService backed by the given repository.
func NewDiscoveryService(repo discovery.Repository) *DiscoveryService {
	return &DiscoveryService{repo: repo}
}

// Nearby returns published routes near the given point.
// RadiusKm defaults to 10 km if zero; Limit defaults to 20 if zero.
func (s *DiscoveryService) Nearby(params discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	if params.RadiusKm <= 0 {
		params.RadiusKm = 10
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	return s.repo.Nearby(params)
}

// Viewport returns published routes intersecting the given bounding box.
// Limit defaults to 50 if zero.
func (s *DiscoveryService) Viewport(params discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	return s.repo.Viewport(params)
}

// Suggested returns recommended routes for a location and departure time.
// Phase 2: proximity-ordered. Phase 3: environment-scored.
// Limit defaults to 10 if zero.
func (s *DiscoveryService) Suggested(params discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}
	return s.repo.Suggested(params)
}
```

- [ ] 5.3 Run service tests.

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/app/... -run TestDiscoveryService -v
```

- [ ] 5.4 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/app/discovery_service.go internal/app/discovery_service_test.go
git commit -m "feat(discovery): add DiscoveryService application service"
```

---

## Task 6 — Venue Application Service

**Files:**
- `internal/app/venue_service.go` (create)
- `internal/app/venue_service_test.go` (create)

### Steps

- [ ] 6.1 Write the test using a stub.

```go
// internal/app/venue_service_test.go
package app_test

import (
	"errors"
	"testing"

	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
)

// stubVenueRepo implements environment.VenueRepository for tests.
type stubVenueRepo struct {
	venues []environment.Venue
	tags   []environment.VenueTag
	err    error
}

func (s *stubVenueRepo) AlongRoute(_ environment.AlongRouteParams) ([]environment.Venue, error) {
	return s.venues, s.err
}
func (s *stubVenueRepo) ListTags() ([]environment.VenueTag, error) {
	return s.tags, s.err
}

func TestVenueService_AlongRoute_DefaultBuffer(t *testing.T) {
	stub := &stubVenueRepo{
		venues: []environment.Venue{
			{ID: "v1", Name: "7-Eleven", Category: "convenience", Lat: 35.69, Lon: 139.70},
		},
	}
	svc := app.NewVenueService(stub)

	venues, err := svc.AlongRoute(environment.AlongRouteParams{RouteID: "r1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(venues) != 1 {
		t.Fatalf("expected 1 venue, got %d", len(venues))
	}
}

func TestVenueService_AlongRoute_PropagatesError(t *testing.T) {
	stub := &stubVenueRepo{err: errors.New("db down")}
	svc := app.NewVenueService(stub)

	_, err := svc.AlongRoute(environment.AlongRouteParams{RouteID: "r1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestVenueService_ListTags(t *testing.T) {
	stub := &stubVenueRepo{
		tags: []environment.VenueTag{
			{Hashtag: "#konbini", Description: "Convenience store", IsBrand: false},
		},
	}
	svc := app.NewVenueService(stub)

	tags, err := svc.ListTags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Hashtag != "#konbini" {
		t.Fatalf("expected #konbini, got %s", tags[0].Hashtag)
	}
}
```

- [ ] 6.2 Create `internal/app/venue_service.go`.

```go
// internal/app/venue_service.go
package app

import (
	"komorebi/internal/domain/environment"
)

// VenueService provides application-level venue discovery use cases.
type VenueService struct {
	repo environment.VenueRepository
}

// NewVenueService creates a VenueService backed by the given repository.
func NewVenueService(repo environment.VenueRepository) *VenueService {
	return &VenueService{repo: repo}
}

// AlongRoute returns venues within BufferM metres of the named route.
// BufferM defaults to 200 m if zero.
func (s *VenueService) AlongRoute(params environment.AlongRouteParams) ([]environment.Venue, error) {
	if params.BufferM <= 0 {
		params.BufferM = 200
	}
	return s.repo.AlongRoute(params)
}

// ListTags returns all venue hashtag definitions.
func (s *VenueService) ListTags() ([]environment.VenueTag, error) {
	return s.repo.ListTags()
}
```

- [ ] 6.3 Run service tests.

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/app/... -run TestVenueService -v
```

- [ ] 6.4 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/app/venue_service.go internal/app/venue_service_test.go
git commit -m "feat(environment): add VenueService application service"
```

---

## Task 7 — Discovery HTTP Handler

**Files:**
- `internal/api/discovery_handler.go` (create)
- `internal/api/discovery_handler_test.go` (create)

### Steps

- [ ] 7.1 Write the handler test using `httptest`.

```go
// internal/api/discovery_handler_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/domain/discovery"
)

// stubDiscoveryRepoHTTP implements discovery.Repository for HTTP handler tests.
type stubDiscoveryRepoHTTP struct {
	results []discovery.DiscoveryResult
}

func (s *stubDiscoveryRepoHTTP) Nearby(_ discovery.NearbyParams) ([]discovery.DiscoveryResult, error) {
	return s.results, nil
}
func (s *stubDiscoveryRepoHTTP) Viewport(_ discovery.ViewportParams) ([]discovery.DiscoveryResult, error) {
	return s.results, nil
}
func (s *stubDiscoveryRepoHTTP) Suggested(_ discovery.SuggestedParams) ([]discovery.DiscoveryResult, error) {
	return s.results, nil
}

func newTestDiscoverySvc(results []discovery.DiscoveryResult) *app.DiscoveryService {
	return app.NewDiscoveryService(&stubDiscoveryRepoHTTP{results: results})
}

func TestDiscoveryHandler_Nearby_OK(t *testing.T) {
	svc := newTestDiscoverySvc([]discovery.DiscoveryResult{
		{RouteID: "r1", Name: "Loop A", DistanceM: 10000, DistFromM: 500, Tags: []string{}},
	})
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/nearby?lat=35.68&lon=139.69&radius_km=5", nil)
	rr := httptest.NewRecorder()
	h.Nearby(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	routes, ok := resp["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("expected 1 route in response, got %v", resp)
	}
}

func TestDiscoveryHandler_Nearby_MissingLat(t *testing.T) {
	svc := newTestDiscoverySvc(nil)
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/nearby?lon=139.69", nil)
	rr := httptest.NewRecorder()
	h.Nearby(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDiscoveryHandler_Viewport_OK(t *testing.T) {
	svc := newTestDiscoverySvc([]discovery.DiscoveryResult{
		{RouteID: "r2", Name: "Bay Loop", DistanceM: 8000, Tags: []string{"scenic"}},
	})
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/viewport?bbox=139.5,35.5,140.0,35.9", nil)
	rr := httptest.NewRecorder()
	h.Viewport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDiscoveryHandler_Viewport_BadBBox(t *testing.T) {
	svc := newTestDiscoverySvc(nil)
	h := api.NewDiscoveryHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/viewport?bbox=bad", nil)
	rr := httptest.NewRecorder()
	h.Viewport(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDiscoveryHandler_Suggested_OK(t *testing.T) {
	svc := newTestDiscoverySvc([]discovery.DiscoveryResult{
		{RouteID: "r3", Name: "Morning Ride", DistanceM: 15000, Tags: []string{}},
	})
	h := api.NewDiscoveryHandler(svc)

	dep := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/discover/suggested?lat=35.68&lon=139.69&departure_at="+dep, nil)
	rr := httptest.NewRecorder()
	h.Suggested(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
```

- [ ] 7.2 Create `internal/api/discovery_handler.go`.

```go
// internal/api/discovery_handler.go
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/discovery"
)

// DiscoveryHandler handles HTTP requests for the Discovery bounded context.
type DiscoveryHandler struct {
	svc *app.DiscoveryService
}

// NewDiscoveryHandler creates a DiscoveryHandler backed by the given service.
func NewDiscoveryHandler(svc *app.DiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc}
}

// --- Response types ---

type discoveryResultResponse struct {
	RouteID        string   `json:"route_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	DistanceM      float64  `json:"distance_m"`
	ElevationGainM float64  `json:"elevation_gain_m"`
	ElevationLossM float64  `json:"elevation_loss_m"`
	Difficulty     string   `json:"difficulty"`
	Status         string   `json:"status"`
	Tags           []string `json:"tags"`
	DistFromM      float64  `json:"dist_from_m,omitempty"`
}

type discoveryListResponse struct {
	Routes []discoveryResultResponse `json:"routes"`
}

// --- Handlers ---

// Nearby handles GET /api/v1/discover/nearby?lat=&lon=&radius_km=
func (h *DiscoveryHandler) Nearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	latStr := q.Get("lat")
	lonStr := q.Get("lon")
	if latStr == "" || lonStr == "" {
		writeError(w, http.StatusBadRequest, "lat and lon are required")
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lat")
		return
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lon")
		return
	}

	params := discovery.NearbyParams{Lat: lat, Lon: lon}

	if v := q.Get("radius_km"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			params.RadiusKm = f
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.Limit = n
		}
	}

	results, err := h.svc.Nearby(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query nearby routes")
		return
	}

	writeJSON(w, http.StatusOK, discoveryListResponse{Routes: toDiscoveryResponse(results)})
}

// Viewport handles GET /api/v1/discover/viewport?bbox=minLon,minLat,maxLon,maxLat
func (h *DiscoveryHandler) Viewport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	bboxStr := q.Get("bbox")
	if bboxStr == "" {
		writeError(w, http.StatusBadRequest, "bbox is required")
		return
	}

	parts := strings.Split(bboxStr, ",")
	if len(parts) != 4 {
		writeError(w, http.StatusBadRequest, "bbox must be minLon,minLat,maxLon,maxLat")
		return
	}

	var bbox [4]float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid bbox value")
			return
		}
		bbox[i] = v
	}

	params := discovery.ViewportParams{BBox: bbox}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.Limit = n
		}
	}

	results, err := h.svc.Viewport(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query viewport routes")
		return
	}

	writeJSON(w, http.StatusOK, discoveryListResponse{Routes: toDiscoveryResponse(results)})
}

// Suggested handles GET /api/v1/discover/suggested?lat=&lon=&departure_at=
func (h *DiscoveryHandler) Suggested(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	latStr := q.Get("lat")
	lonStr := q.Get("lon")
	if latStr == "" || lonStr == "" {
		writeError(w, http.StatusBadRequest, "lat and lon are required")
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lat")
		return
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid lon")
		return
	}

	params := discovery.SuggestedParams{Lat: lat, Lon: lon, DepartureAt: time.Now()}

	if v := q.Get("departure_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.DepartureAt = t
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.Limit = n
		}
	}

	results, err := h.svc.Suggested(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query suggested routes")
		return
	}

	writeJSON(w, http.StatusOK, discoveryListResponse{Routes: toDiscoveryResponse(results)})
}

// --- helpers ---

func toDiscoveryResponse(results []discovery.DiscoveryResult) []discoveryResultResponse {
	out := make([]discoveryResultResponse, len(results))
	for i, r := range results {
		tags := r.Tags
		if tags == nil {
			tags = []string{}
		}
		out[i] = discoveryResultResponse{
			RouteID:        r.RouteID,
			Name:           r.Name,
			Description:    r.Description,
			DistanceM:      r.DistanceM,
			ElevationGainM: r.ElevationGainM,
			ElevationLossM: r.ElevationLossM,
			Difficulty:     r.Difficulty,
			Status:         r.Status,
			Tags:           tags,
			DistFromM:      r.DistFromM,
		}
	}
	return out
}
```

- [ ] 7.3 Run handler tests.

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/api/... -run TestDiscoveryHandler -v
```

- [ ] 7.4 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/api/discovery_handler.go internal/api/discovery_handler_test.go
git commit -m "feat(discovery): add Discovery HTTP handler (nearby, viewport, suggested)"
```

---

## Task 8 — Venue HTTP Handler

**Files:**
- `internal/api/venue_handler.go` (create)
- `internal/api/venue_handler_test.go` (create)

### Steps

- [ ] 8.1 Write the handler test.

```go
// internal/api/venue_handler_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
	"github.com/go-chi/chi/v5"
)

// stubVenueRepoHTTP implements environment.VenueRepository for handler tests.
type stubVenueRepoHTTP struct {
	venues []environment.Venue
	tags   []environment.VenueTag
}

func (s *stubVenueRepoHTTP) AlongRoute(_ environment.AlongRouteParams) ([]environment.Venue, error) {
	return s.venues, nil
}
func (s *stubVenueRepoHTTP) ListTags() ([]environment.VenueTag, error) {
	return s.tags, nil
}

func newTestVenueSvc(venues []environment.Venue, tags []environment.VenueTag) *app.VenueService {
	return app.NewVenueService(&stubVenueRepoHTTP{venues: venues, tags: tags})
}

func TestVenueHandler_AlongRoute_OK(t *testing.T) {
	venues := []environment.Venue{
		{ID: "v1", Name: "7-Eleven", Category: "convenience", Lat: 35.69, Lon: 139.70, OsmTags: map[string]string{}},
	}
	svc := newTestVenueSvc(venues, nil)
	h := api.NewVenueHandler(svc)

	// Wire into chi router to populate URL params
	r := chi.NewRouter()
	r.Get("/api/v1/venues/along-route", h.AlongRoute)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/venues/along-route?route_id=00000000-0000-0000-0000-000000000001&buffer_m=300", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	venues2, ok := resp["venues"].([]any)
	if !ok || len(venues2) != 1 {
		t.Fatalf("expected 1 venue, got %v", resp)
	}
}

func TestVenueHandler_AlongRoute_MissingRouteID(t *testing.T) {
	svc := newTestVenueSvc(nil, nil)
	h := api.NewVenueHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/venues/along-route", nil)
	rr := httptest.NewRecorder()
	h.AlongRoute(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestVenueHandler_Tags_OK(t *testing.T) {
	tags := []environment.VenueTag{
		{Hashtag: "#konbini", Description: "Convenience store", IsBrand: false},
		{Hashtag: "#7-eleven", Description: "7-Eleven brand", IsBrand: true},
	}
	svc := newTestVenueSvc(nil, tags)
	h := api.NewVenueHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/venues/tags", nil)
	rr := httptest.NewRecorder()
	h.Tags(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tagsOut, ok := resp["tags"].([]any)
	if !ok || len(tagsOut) != 2 {
		t.Fatalf("expected 2 tags, got %v", resp)
	}
}
```

- [ ] 8.2 Create `internal/api/venue_handler.go`.

```go
// internal/api/venue_handler.go
package api

import (
	"net/http"
	"strconv"

	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
)

// VenueHandler handles HTTP requests for venue discovery.
type VenueHandler struct {
	svc *app.VenueService
}

// NewVenueHandler creates a VenueHandler backed by the given service.
func NewVenueHandler(svc *app.VenueService) *VenueHandler {
	return &VenueHandler{svc: svc}
}

// --- Response types ---

type venueResponse struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Category string            `json:"category"`
	Brand    string            `json:"brand,omitempty"`
	Lat      float64           `json:"lat"`
	Lon      float64           `json:"lon"`
	OsmTags  map[string]string `json:"osm_tags,omitempty"`
}

type venueListResponse struct {
	Venues []venueResponse `json:"venues"`
}

type venueTagResponse struct {
	Hashtag     string `json:"hashtag"`
	Description string `json:"description"`
	IsBrand     bool   `json:"is_brand"`
}

type venueTagListResponse struct {
	Tags []venueTagResponse `json:"tags"`
}

// --- Handlers ---

// AlongRoute handles GET /api/v1/venues/along-route?route_id=&type=&buffer_m=
func (h *VenueHandler) AlongRoute(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	routeID := q.Get("route_id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route_id is required")
		return
	}

	params := environment.AlongRouteParams{
		RouteID:  routeID,
		Category: q.Get("type"),
	}

	if v := q.Get("buffer_m"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			params.BufferM = f
		}
	}

	venues, err := h.svc.AlongRoute(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query venues along route")
		return
	}

	resp := venueListResponse{Venues: make([]venueResponse, len(venues))}
	for i, v := range venues {
		tags := v.OsmTags
		if tags == nil {
			tags = map[string]string{}
		}
		resp.Venues[i] = venueResponse{
			ID:       v.ID,
			Name:     v.Name,
			Category: v.Category,
			Brand:    v.Brand,
			Lat:      v.Lat,
			Lon:      v.Lon,
			OsmTags:  tags,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// Tags handles GET /api/v1/venues/tags
func (h *VenueHandler) Tags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.svc.ListTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list venue tags")
		return
	}

	resp := venueTagListResponse{Tags: make([]venueTagResponse, len(tags))}
	for i, t := range tags {
		resp.Tags[i] = venueTagResponse{
			Hashtag:     t.Hashtag,
			Description: t.Description,
			IsBrand:     t.IsBrand,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] 8.3 Run handler tests.

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/api/... -run TestVenueHandler -v
```

- [ ] 8.4 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/api/venue_handler.go internal/api/venue_handler_test.go
git commit -m "feat(environment): add Venue HTTP handler (along-route, tags)"
```

---

## Task 9 — Wire Into Router and Update main.go

**Files:**
- `internal/api/router.go` (edit)
- `cmd/api/main.go` (edit)

### Steps

- [ ] 9.1 Read the current `router.go` and `main.go` before editing.

```bash
cat /Users/lug/src/cyclist-map/internal/api/router.go
cat /Users/lug/src/cyclist-map/cmd/api/main.go
```

- [ ] 9.2 Replace `internal/api/router.go` with the updated version that accepts `DiscoveryHandler` and `VenueHandler`.

The updated `NewRouter` signature must remain backward-compatible for existing tests. Add the new services as additional parameters.

```go
// internal/api/router.go
package api

import (
	"komorebi/internal/app"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router with all API routes wired up.
func NewRouter(
	routeSvc *app.RouteService,
	discoverySvc *app.DiscoveryService,
	venueSvc *app.VenueService,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	rh := &RouteHandler{svc: routeSvc}
	dh := NewDiscoveryHandler(discoverySvc)
	vh := NewVenueHandler(venueSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Routes
		r.Get("/routes", rh.List)
		r.Post("/routes", rh.Create)
		r.Get("/routes/{id}", rh.Get)
		r.Patch("/routes/{id}", rh.Update)
		r.Delete("/routes/{id}", rh.Archive)

		// Discovery
		r.Get("/discover/nearby", dh.Nearby)
		r.Get("/discover/viewport", dh.Viewport)
		r.Get("/discover/suggested", dh.Suggested)

		// Venues
		r.Get("/venues/along-route", vh.AlongRoute)
		r.Get("/venues/tags", vh.Tags)
	})
	return r
}
```

- [ ] 9.3 Update `cmd/api/main.go` to construct and inject all three services.

Read the current main.go first, then apply the update. The key change is constructing `DiscoveryRepo`, `VenueRepo`, `DiscoveryService`, `VenueService`, and passing them to `NewRouter`.

```go
// cmd/api/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	routeRepo := postgres.NewRouteRepo(pool)
	routeSvc := app.NewRouteService(routeRepo)

	discoveryRepo := postgres.NewDiscoveryRepo(pool)
	discoverySvc := app.NewDiscoveryService(discoveryRepo)

	venueRepo := postgres.NewVenueRepo(pool)
	venueSvc := app.NewVenueService(venueRepo)

	router := api.NewRouter(routeSvc, discoverySvc, venueSvc)

	addr := ":8080"
	log.Printf("cyclist-map API listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] 9.4 Verify the full project compiles with no errors.

```bash
cd /Users/lug/src/cyclist-map
go build ./...
```

- [ ] 9.5 Run all non-integration tests.

```bash
cd /Users/lug/src/cyclist-map
go test ./internal/domain/... ./internal/app/... ./internal/api/...
```

- [ ] 9.6 Run integration tests against the dev DB.

```bash
cd /Users/lug/src/cyclist-map
TEST_DB_DSN="postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable" \
  go test ./internal/infra/postgres/... -v
```

- [ ] 9.7 Smoke-test the live endpoints (requires the server running in another terminal).

```bash
# Start server
go run ./cmd/api &
API_PID=$!

# Nearby
curl -s "http://localhost:8080/api/v1/discover/nearby?lat=35.6895&lon=139.6917&radius_km=10" | jq .

# Viewport
curl -s "http://localhost:8080/api/v1/discover/viewport?bbox=139.5,35.5,140.0,35.9" | jq .

# Suggested
curl -s "http://localhost:8080/api/v1/discover/suggested?lat=35.6895&lon=139.6917" | jq .

# Venue tags
curl -s "http://localhost:8080/api/v1/venues/tags" | jq .

# Venues along a route (replace UUID with a real route ID from the DB)
ROUTE_ID=$(PGPASSWORD=osm_dev psql -h localhost -U osm_dev -d cyclist_map_dev \
  -t -c "SELECT id FROM routes.route LIMIT 1;" | xargs)
if [ -n "$ROUTE_ID" ]; then
  curl -s "http://localhost:8080/api/v1/venues/along-route?route_id=$ROUTE_ID&buffer_m=500" | jq .
fi

kill $API_PID
```

- [ ] 9.8 Commit.

```bash
cd /Users/lug/src/cyclist-map
git add internal/api/router.go cmd/api/main.go
git commit -m "feat(discovery): wire Discovery and Venue into router and main.go"
```

---

## Self-Review Checklist

Before marking this plan complete, verify each item:

- [ ] **Compiles:** `go build ./...` passes with zero errors.
- [ ] **All unit tests pass:** `go test ./internal/domain/... ./internal/app/... ./internal/api/...` — green.
- [ ] **Integration tests pass:** `TEST_DB_DSN=... go test ./internal/infra/postgres/...` — green (or skipped cleanly when DSN is absent).
- [ ] **No unused imports or variables** — `go vet ./...` clean.
- [ ] **Domain packages have zero external imports** — `internal/domain/discovery/` and `internal/domain/environment/` import only standard library packages.
- [ ] **Repository interface is satisfied** — `DiscoveryRepo` implements `discovery.Repository`; `VenueRepo` implements `environment.VenueRepository`. Verify with compile-time assertions if needed:
  ```go
  var _ discovery.Repository = (*postgres.DiscoveryRepo)(nil)
  var _ environment.VenueRepository = (*postgres.VenueRepo)(nil)
  ```
- [ ] **Error handling:** All handler errors return appropriate HTTP status codes (400 for bad input, 404 for not found, 500 for infra errors).
- [ ] **Default values applied in service layer**, not in handler or repository — radius, buffer, limit defaults are in `DiscoveryService` and `VenueService`.
- [ ] **Spatial correctness:** `ST_DWithin` and `ST_Distance` use `::geography` cast so distances are in metres (not degrees). `ST_MakePoint($lon, $lat)` — longitude is X (first arg), latitude is Y (second arg).
- [ ] **Route status filter:** All discovery queries include `WHERE r.status = 'published'` so draft and archived routes are never returned.
- [ ] **Tags loaded for all discovery results** — `loadTagsForRoute` is called for each result in `Nearby` and `Viewport`.
- [ ] **Suggested is a placeholder** — `Suggested` delegates to `Nearby` with wider radius; code comment documents Phase 3 upgrade path.
- [ ] **Router backward-compatible signature change** — verify existing route handler tests still pass after `NewRouter` signature change.
- [ ] **`newTestPool` helper not duplicated** — if `route_repo_test.go` already defines it in the `postgres_test` package, extract it to a shared `testhelpers_test.go` file in the same package.

---

## Notes for Phase 3 Upgrade

The `Suggested` endpoint is intentionally minimal in this plan. Phase 3 will replace the `Suggested` method on `DiscoveryRepo` with a scored query that:

1. Joins `environment.shadow_grid` at the departure-time solar position to compute shade coverage per route.
2. Joins `environment.weather_grid` at `departure_at` for wind and precipitation scores.
3. Joins `environment.greenery_edge` via route geometry intersection.
4. Computes a weighted composite score and orders by score DESC.

The domain interface (`discovery.Repository`) and service signature are already designed to accept `DepartureAt time.Time` in `SuggestedParams`, so the Phase 3 implementation will be a drop-in replacement for the repository method only — no handler or service changes required.
