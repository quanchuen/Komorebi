# Valhalla Routing Integration

**Date:** 2026-04-10
**Goal:** Wire Valhalla into the Go API so `POST /api/v1/routing/directions` returns a real multi-stop bicycle route with per-leg breakdown and GeoJSON geometry.
**Architecture:** Three-layer addition — `internal/infra/valhalla` (HTTP client) → `internal/app/routing_service.go` (use-case orchestration) → `internal/api/routing_handler.go` + router registration.
**Tech stack:** Go 1.25, chi v5, `paulmach/orb` (already in go.mod for geometry), Valhalla HTTP JSON API (self-hosted at `http://localhost:8002`).

---

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to execute the tasks below. Tasks 1–3 are independent (client, service, handler) and can be dispatched in parallel once you have confirmed the Valhalla container starts in Task 0. Task 4 (router wiring) has a hard dependency on Tasks 2 and 3 being complete. Task 5 (docker-compose verification) is the final gate — do not mark work done until it passes.

---

## Task 0 — Verify Valhalla container starts

**Goal:** Confirm the existing `docker-compose.yml` Valhalla service is reachable before writing any Go code.

**Files:**
- `/Users/lug/src/cyclist-map/docker-compose.yml`

- [ ] Start the valhalla service:
  ```bash
  docker compose up -d valhalla
  ```
- [ ] Wait for the tile build to complete (first run downloads ~1.5 GB PBF and builds tiles; can take 15–30 min — poll the health endpoint):
  ```bash
  until curl -sf http://localhost:8002/status | grep -q '"has_tiles":true'; do
    echo "waiting for valhalla tiles…"; sleep 30
  done
  echo "valhalla ready"
  ```
- [ ] Smoke-test a route between two Tokyo coordinates:
  ```bash
  curl -s -X POST http://localhost:8002/route \
    -H 'Content-Type: application/json' \
    -d '{
      "locations": [
        {"lon": 139.6917, "lat": 35.6895},
        {"lon": 139.7671, "lat": 35.6812}
      ],
      "costing": "bicycle",
      "directions_options": {"units": "km"}
    }' | jq '.trip.legs[0].summary'
  ```
  Expect a JSON object with `length` and `time` fields.

> If tiles are not yet built, the `/status` endpoint returns `"has_tiles": false`. The container auto-builds on first start via the `tile_urls` env var — just wait. Do not modify `docker-compose.yml` unless the container crashes (check `docker compose logs valhalla`).

---

## Task 1 — Valhalla HTTP client (`internal/infra/valhalla/`)

**Goal:** A typed Go client that sends Valhalla `/route` requests and returns parsed route data. No business logic — pure transport adapter.

**Files:**
- `internal/infra/valhalla/client.go` (new)
- `internal/infra/valhalla/client_test.go` (new)

### 1a — Write the test first

- [ ] Create `internal/infra/valhalla/client_test.go`:

```go
package valhalla_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// minimalValhallaResponse returns a Valhalla /route JSON response with one leg.
func minimalValhallaResponse() string {
	return `{
		"trip": {
			"summary": {"length": 5.23, "time": 1245},
			"legs": [
				{
					"summary": {"length": 5.23, "time": 1245},
					"shape": "efo}Hqsk|YuA`Bk@nA"
				}
			]
		}
	}`
}

func TestClient_Route_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/route" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		// Verify costing field is present in request
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("could not decode request body: %v", err)
		}
		if body["costing"] != "bicycle" {
			t.Errorf("expected costing=bicycle, got %v", body["costing"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(minimalValhallaResponse()))
	}))
	defer srv.Close()

	client := valhalla.NewClient(srv.URL)

	stops := []valhalla.Location{
		{Lat: 35.6895, Lon: 139.6917},
		{Lat: 35.6812, Lon: 139.7671},
	}
	result, err := client.Route(stops)
	if err != nil {
		t.Fatalf("Route() returned error: %v", err)
	}
	if result.TotalDistanceKm < 0.01 {
		t.Errorf("expected non-zero distance, got %f", result.TotalDistanceKm)
	}
	if len(result.Legs) != 1 {
		t.Errorf("expected 1 leg, got %d", len(result.Legs))
	}
	if result.Legs[0].DurationS < 1 {
		t.Errorf("expected non-zero duration, got %f", result.Legs[0].DurationS)
	}
	// Decoded shape must be non-empty
	if len(result.Legs[0].Shape) == 0 {
		t.Error("expected decoded shape coordinates, got empty slice")
	}
}

func TestClient_Route_MultiStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		locs, ok := body["locations"].([]any)
		if !ok || len(locs) != 3 {
			t.Errorf("expected 3 locations in request, got %v", body["locations"])
		}
		// Second location should be a via-point (type=through or break_through)
		// For multi-stop, middle stops should be "through" type
		resp := `{
			"trip": {
				"summary": {"length": 10.5, "time": 2500},
				"legs": [
					{"summary": {"length": 5.0, "time": 1200}, "shape": "efo}Hqsk|YuA` + "`" + `Bk@nA"},
					{"summary": {"length": 5.5, "time": 1300}, "shape": "efo}Hqsk|YuA` + "`" + `Bk@nA"}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	client := valhalla.NewClient(srv.URL)
	stops := []valhalla.Location{
		{Lat: 35.6895, Lon: 139.6917},
		{Lat: 35.6900, Lon: 139.7000},
		{Lat: 35.6812, Lon: 139.7671},
	}
	result, err := client.Route(stops)
	if err != nil {
		t.Fatalf("Route() returned error: %v", err)
	}
	if len(result.Legs) != 2 {
		t.Errorf("expected 2 legs for 3 stops, got %d", len(result.Legs))
	}
}

func TestClient_Route_ValhallaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "No path found", "error_code": 442}`))
	}))
	defer srv.Close()

	client := valhalla.NewClient(srv.URL)
	stops := []valhalla.Location{
		{Lat: 35.6895, Lon: 139.6917},
		{Lat: 35.6812, Lon: 139.7671},
	}
	_, err := client.Route(stops)
	if err == nil {
		t.Fatal("expected error for Valhalla 400 response, got nil")
	}
}

func TestClient_Route_TooFewStops(t *testing.T) {
	client := valhalla.NewClient("http://localhost:8002")
	_, err := client.Route([]valhalla.Location{{Lat: 35.0, Lon: 139.0}})
	if err == nil {
		t.Fatal("expected error for single stop, got nil")
	}
}
```

### 1b — Write the implementation

- [ ] Create `internal/infra/valhalla/client.go`:

```go
package valhalla

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"
)

// ErrTooFewLocations is returned when fewer than 2 stops are provided.
var ErrTooFewLocations = errors.New("valhalla: at least 2 locations required")

// Location is a lat/lon coordinate to route through.
type Location struct {
	Lat float64
	Lon float64
}

// Leg is one segment of a multi-stop route (between consecutive stops).
type Leg struct {
	DistanceKm float64
	DurationS  float64
	// Shape is the decoded polyline6 geometry as [lon, lat] pairs.
	Shape [][2]float64
}

// RouteResult holds the parsed Valhalla response.
type RouteResult struct {
	TotalDistanceKm float64
	TotalDurationS  float64
	Legs            []Leg
}

// Client is an HTTP client for the Valhalla routing engine.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client targeting the given Valhalla base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Route requests a bicycle route through the given stops via Valhalla's /route endpoint.
// Intermediate stops (all but first and last) are sent as "through" type so Valhalla
// treats them as via-points rather than full break stops, keeping geometry continuous.
func (c *Client) Route(stops []Location) (*RouteResult, error) {
	if len(stops) < 2 {
		return nil, ErrTooFewLocations
	}

	locations := make([]map[string]any, len(stops))
	for i, s := range stops {
		loc := map[string]any{
			"lat": s.Lat,
			"lon": s.Lon,
		}
		// Middle stops are via-points; first and last are full break stops.
		if i > 0 && i < len(stops)-1 {
			loc["type"] = "through"
		} else {
			loc["type"] = "break"
		}
		locations[i] = loc
	}

	body := map[string]any{
		"locations": locations,
		"costing":   "bicycle",
		"costing_options": map[string]any{
			"bicycle": map[string]any{
				"bicycle_type":  "Road",
				"cycling_speed": 15,
				"use_roads":     0.5,
				"use_hills":     0.2,
			},
		},
		"directions_options": map[string]any{
			"units": "km",
		},
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("valhalla: marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/route", "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("valhalla: http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error     string `json:"error"`
			ErrorCode int    `json:"error_code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("valhalla: http %d: %s (code %d)", resp.StatusCode, errBody.Error, errBody.ErrorCode)
	}

	var raw struct {
		Trip struct {
			Summary struct {
				Length float64 `json:"length"`
				Time   float64 `json:"time"`
			} `json:"summary"`
			Legs []struct {
				Summary struct {
					Length float64 `json:"length"`
					Time   float64 `json:"time"`
				} `json:"summary"`
				Shape string `json:"shape"`
			} `json:"legs"`
		} `json:"trip"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("valhalla: decode response: %w", err)
	}

	result := &RouteResult{
		TotalDistanceKm: raw.Trip.Summary.Length,
		TotalDurationS:  raw.Trip.Summary.Time,
		Legs:            make([]Leg, len(raw.Trip.Legs)),
	}
	for i, l := range raw.Trip.Legs {
		result.Legs[i] = Leg{
			DistanceKm: l.Summary.Length,
			DurationS:  l.Summary.Time,
			Shape:      decodePolyline6(l.Shape),
		}
	}
	return result, nil
}

// decodePolyline6 decodes a Valhalla polyline6-encoded string into [lon, lat] pairs.
// Valhalla uses precision 6 (factor 1e6), encoding [lat, lon] pairs.
func decodePolyline6(encoded string) [][2]float64 {
	const factor = 1e6
	var coords [][2]float64
	var lat, lon int64
	i := 0
	for i < len(encoded) {
		lat += decodeChunk(encoded, &i)
		lon += decodeChunk(encoded, &i)
		coords = append(coords, [2]float64{
			math.Round(float64(lon)/factor*factor) / factor, // lon first (GeoJSON order)
			math.Round(float64(lat)/factor*factor) / factor,
		})
	}
	return coords
}

func decodeChunk(encoded string, i *int) int64 {
	var result int64
	var shift uint
	for *i < len(encoded) {
		b := int64(encoded[*i]) - 63
		*i++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		return ^(result >> 1)
	}
	return result >> 1
}
```

- [ ] Run the tests:
  ```bash
  cd /Users/lug/src/cyclist-map && go test ./internal/infra/valhalla/...
  ```
  All 4 tests must pass.

- [ ] Commit:
  ```bash
  cd /Users/lug/src/cyclist-map && git add internal/infra/valhalla/ && git commit -m "$(cat <<'EOF'
  feat: add Valhalla HTTP client with polyline6 decoding

  Introduces internal/infra/valhalla with a typed Client that posts bicycle
  route requests to Valhalla /route, handles multi-stop via-points, decodes
  polyline6 geometry, and surfaces Valhalla error codes as Go errors.

  Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 2 — Routing application service (`internal/app/routing_service.go`)

**Goal:** Orchestrate a routing request — validate stops, call the Valhalla client, return a structured result the HTTP handler can serialise. No database calls yet; venue resolution is a pass-through stub.

**Files:**
- `internal/app/routing_service.go` (new)
- `internal/app/routing_service_test.go` (new)

### 2a — Write the test first

- [ ] Create `internal/app/routing_service_test.go`:

```go
package app_test

import (
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// fakeValhallaClient is a test double for the valhalla.Client.
type fakeValhallaClient struct {
	result *valhalla.RouteResult
	err    error
}

func (f *fakeValhallaClient) Route(stops []valhalla.Location) (*valhalla.RouteResult, error) {
	return f.result, f.err
}

func twoStopPlan() []plan.StopPoint {
	return []plan.StopPoint{
		{ID: "a", Lat: 35.6895, Lon: 139.6917, Type: plan.StopManual, SortOrder: 0},
		{ID: "b", Lat: 35.6812, Lon: 139.7671, Type: plan.StopManual, SortOrder: 1},
	}
}

func fakeRouteResult() *valhalla.RouteResult {
	return &valhalla.RouteResult{
		TotalDistanceKm: 7.5,
		TotalDurationS:  1800,
		Legs: []valhalla.Leg{
			{
				DistanceKm: 7.5,
				DurationS:  1800,
				Shape: [][2]float64{
					{139.6917, 35.6895},
					{139.7000, 35.6850},
					{139.7671, 35.6812},
				},
			},
		},
	}
}

func TestRoutingService_GetDirections_Success(t *testing.T) {
	fake := &fakeValhallaClient{result: fakeRouteResult()}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops:       twoStopPlan(),
		DepartureAt: time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC),
		SpeedModel:  plan.SpeedModelElevation,
		Preferences: plan.Preferences{ShadeWeight: 0.5, GreeneryWeight: 0.5, WindWeight: 0.3},
	}

	result, err := svc.GetDirections(req)
	if err != nil {
		t.Fatalf("GetDirections() error: %v", err)
	}
	if result.TotalDistanceKm != 7.5 {
		t.Errorf("expected distance 7.5, got %f", result.TotalDistanceKm)
	}
	if len(result.Legs) != 1 {
		t.Errorf("expected 1 leg, got %d", len(result.Legs))
	}
	// GeoJSON geometry must be populated
	if len(result.GeoJSON.Coordinates) == 0 {
		t.Error("expected non-empty GeoJSON coordinates")
	}
}

func TestRoutingService_GetDirections_TooFewStops(t *testing.T) {
	fake := &fakeValhallaClient{result: fakeRouteResult()}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops:       twoStopPlan()[:1],
		DepartureAt: time.Now(),
	}
	_, err := svc.GetDirections(req)
	if err == nil {
		t.Fatal("expected error for fewer than 2 stops")
	}
}

func TestRoutingService_GetDirections_ClientError(t *testing.T) {
	fake := &fakeValhallaClient{err: fmt.Errorf("valhalla: http 442: No path found")}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops:       twoStopPlan(),
		DepartureAt: time.Now(),
	}
	_, err := svc.GetDirections(req)
	if err == nil {
		t.Fatal("expected error when client returns error")
	}
}

func TestRoutingService_GetDirections_MultiStop(t *testing.T) {
	fake := &fakeValhallaClient{
		result: &valhalla.RouteResult{
			TotalDistanceKm: 15.0,
			TotalDurationS:  3600,
			Legs: []valhalla.Leg{
				{DistanceKm: 7.5, DurationS: 1800, Shape: [][2]float64{{139.69, 35.68}, {139.70, 35.69}}},
				{DistanceKm: 7.5, DurationS: 1800, Shape: [][2]float64{{139.70, 35.69}, {139.77, 35.68}}},
			},
		},
	}
	svc := app.NewRoutingService(fake)

	req := app.DirectionsRequest{
		Stops: []plan.StopPoint{
			{ID: "a", Lat: 35.6895, Lon: 139.6917, Type: plan.StopManual, SortOrder: 0},
			{ID: "b", Lat: 35.6900, Lon: 139.7000, Type: plan.StopManual, SortOrder: 1},
			{ID: "c", Lat: 35.6812, Lon: 139.7671, Type: plan.StopManual, SortOrder: 2},
		},
		DepartureAt: time.Now(),
	}
	result, err := svc.GetDirections(req)
	if err != nil {
		t.Fatalf("GetDirections() error: %v", err)
	}
	if len(result.Legs) != 2 {
		t.Errorf("expected 2 legs, got %d", len(result.Legs))
	}
	// Full geometry should concatenate both legs' shapes
	if len(result.GeoJSON.Coordinates) < 3 {
		t.Errorf("expected merged geometry with at least 3 points, got %d", len(result.GeoJSON.Coordinates))
	}
}
```

> Note: the test file imports `fmt` — add it to the import block.

### 2b — Write the implementation

- [ ] Create `internal/app/routing_service.go`:

```go
package app

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"
)

// ErrTooFewStops is returned when a directions request has fewer than 2 stops.
var ErrTooFewDirectionStops = errors.New("routing: at least 2 stops required")

// ValhallaRouter is the interface the RoutingService uses to call the routing engine.
// Defined here so tests can inject a fake without importing the infra package.
type ValhallaRouter interface {
	Route(stops []valhalla.Location) (*valhalla.RouteResult, error)
}

// DirectionsRequest carries the inputs for a multi-stop route calculation.
type DirectionsRequest struct {
	Stops       []plan.StopPoint
	DepartureAt time.Time
	SpeedModel  plan.SpeedModel
	Preferences plan.Preferences
}

// LegResult is the per-leg breakdown returned to callers.
type LegResult struct {
	DistanceKm float64
	DurationS  float64
	// ETAAt is the projected arrival time at the end of this leg.
	ETAAt time.Time
}

// GeoJSONLineString represents a GeoJSON LineString geometry.
type GeoJSONLineString struct {
	Type        string      `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// DirectionsResult is the structured output of a routing request.
type DirectionsResult struct {
	TotalDistanceKm float64
	TotalDurationS  float64
	Legs            []LegResult
	// GeoJSON is the merged full-route geometry as a GeoJSON LineString.
	GeoJSON GeoJSONLineString
}

// RoutingService orchestrates multi-stop bicycle routing via Valhalla.
type RoutingService struct {
	router ValhallaRouter
}

// NewRoutingService creates a RoutingService backed by the given router client.
func NewRoutingService(router ValhallaRouter) *RoutingService {
	return &RoutingService{router: router}
}

// GetDirections resolves stops to coordinates, calls Valhalla, and returns the
// route geometry with per-leg ETAs.
//
// Venue-typed stops are passed through as-is in this initial implementation;
// venue resolution (snapping to nearest OSM match) is a future concern.
func (s *RoutingService) GetDirections(req DirectionsRequest) (*DirectionsResult, error) {
	if len(req.Stops) < 2 {
		return nil, ErrTooFewDirectionStops
	}

	// Sort by SortOrder to guarantee sequence before sending to Valhalla.
	stops := make([]plan.StopPoint, len(req.Stops))
	copy(stops, req.Stops)
	sort.Slice(stops, func(i, j int) bool {
		return stops[i].SortOrder < stops[j].SortOrder
	})

	// Translate domain stops to Valhalla locations.
	locs := make([]valhalla.Location, len(stops))
	for i, sp := range stops {
		locs[i] = valhalla.Location{Lat: sp.Lat, Lon: sp.Lon}
	}

	raw, err := s.router.Route(locs)
	if err != nil {
		return nil, fmt.Errorf("routing: valhalla error: %w", err)
	}

	// Build per-leg results with projected ETAs.
	legs := make([]LegResult, len(raw.Legs))
	elapsed := 0.0
	for i, l := range raw.Legs {
		elapsed += l.DurationS
		legs[i] = LegResult{
			DistanceKm: l.DistanceKm,
			DurationS:  l.DurationS,
			ETAAt:      req.DepartureAt.Add(time.Duration(elapsed) * time.Second),
		}
	}

	// Merge all leg shapes into a single LineString.
	// Deduplicate the shared point between consecutive legs (last point of leg N
	// equals first point of leg N+1).
	var merged [][2]float64
	for i, l := range raw.Legs {
		if i == 0 {
			merged = append(merged, l.Shape...)
		} else if len(l.Shape) > 0 {
			merged = append(merged, l.Shape[1:]...)
		}
	}

	return &DirectionsResult{
		TotalDistanceKm: raw.TotalDistanceKm,
		TotalDurationS:  raw.TotalDurationS,
		Legs:            legs,
		GeoJSON: GeoJSONLineString{
			Type:        "LineString",
			Coordinates: merged,
		},
	}, nil
}
```

- [ ] Run the tests:
  ```bash
  cd /Users/lug/src/cyclist-map && go test ./internal/app/... -run TestRoutingService
  ```
  All 4 tests must pass.

- [ ] Commit:
  ```bash
  cd /Users/lug/src/cyclist-map && git add internal/app/routing_service.go internal/app/routing_service_test.go && git commit -m "$(cat <<'EOF'
  feat: add RoutingService orchestrating multi-stop Valhalla routing

  Translates domain StopPoints to Valhalla locations, merges per-leg
  shapes into a GeoJSON LineString, and computes per-leg ETAs from
  departure time. Venue resolution is a pass-through stub for now.

  Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 3 — Routing HTTP handler (`internal/api/routing_handler.go`)

**Goal:** Expose `POST /api/v1/routing/directions` as a JSON endpoint that accepts the spec-defined request body and returns route geometry, legs, and totals.

**Files:**
- `internal/api/routing_handler.go` (new)
- `internal/api/routing_handler_test.go` (new)

### 3a — Write the test first

- [ ] Create `internal/api/routing_handler_test.go`:

```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
)

// fakeRoutingService is a test double for app.RoutingService.
type fakeRoutingService struct {
	result *app.DirectionsResult
	err    error
}

func (f *fakeRoutingService) GetDirections(req app.DirectionsRequest) (*app.DirectionsResult, error) {
	return f.result, f.err
}

func goodDirectionsResult() *app.DirectionsResult {
	return &app.DirectionsResult{
		TotalDistanceKm: 7.5,
		TotalDurationS:  1800,
		Legs: []app.LegResult{
			{
				DistanceKm: 7.5,
				DurationS:  1800,
				ETAAt:      time.Date(2026, 4, 10, 14, 30, 0, 0, time.UTC),
			},
		},
		GeoJSON: app.GeoJSONLineString{
			Type:        "LineString",
			Coordinates: [][2]float64{{139.6917, 35.6895}, {139.7671, 35.6812}},
		},
	}
}

func validDirectionsBody() []byte {
	body := map[string]any{
		"stops": []map[string]any{
			{"type": "manual", "lat": 35.6895, "lon": 139.6917},
			{"type": "manual", "lat": 35.6812, "lon": 139.7671},
		},
		"departure_at": "2026-04-10T14:00:00+09:00",
		"speed_model":  "elevation",
		"preferences": map[string]any{
			"shade": 0.8, "greenery": 0.5, "wind": 0.6,
		},
	}
	b, _ := json.Marshal(body)
	return b
}

func TestRoutingHandler_Directions_Success(t *testing.T) {
	fake := &fakeRoutingService{result: goodDirectionsResult()}
	handler := api.NewRoutingHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader(validDirectionsBody()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if resp["total_distance_km"] == nil {
		t.Error("response missing total_distance_km")
	}
	if resp["geometry"] == nil {
		t.Error("response missing geometry")
	}
	legs, ok := resp["legs"].([]any)
	if !ok || len(legs) == 0 {
		t.Error("response missing or empty legs")
	}
}

func TestRoutingHandler_Directions_InvalidBody(t *testing.T) {
	fake := &fakeRoutingService{result: goodDirectionsResult()}
	handler := api.NewRoutingHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRoutingHandler_Directions_TooFewStops(t *testing.T) {
	fake := &fakeRoutingService{result: goodDirectionsResult()}
	handler := api.NewRoutingHandler(fake)

	body := map[string]any{
		"stops":        []map[string]any{{"type": "manual", "lat": 35.68, "lon": 139.69}},
		"departure_at": "2026-04-10T14:00:00+09:00",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRoutingHandler_Directions_ServiceError(t *testing.T) {
	fake := &fakeRoutingService{err: fmt.Errorf("valhalla unreachable")}
	handler := api.NewRoutingHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/directions", bytes.NewReader(validDirectionsBody()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Directions(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}
```

> Note: add `"fmt"` to imports.

### 3b — Write the implementation

- [ ] Create `internal/api/routing_handler.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
)

// RoutingDirector is the interface the handler uses to request directions.
// Accepting an interface keeps the handler testable without the real service.
type RoutingDirector interface {
	GetDirections(req app.DirectionsRequest) (*app.DirectionsResult, error)
}

// RoutingHandler handles HTTP requests for the routing endpoints.
type RoutingHandler struct {
	svc RoutingDirector
}

// NewRoutingHandler creates a RoutingHandler backed by the given service.
func NewRoutingHandler(svc RoutingDirector) *RoutingHandler {
	return &RoutingHandler{svc: svc}
}

// --- Request / Response types ---

type directionsStopJSON struct {
	Type    string  `json:"type"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Hashtag string  `json:"hashtag,omitempty"`
}

type directionsPreferencesJSON struct {
	Shade    float64 `json:"shade"`
	Greenery float64 `json:"greenery"`
	Wind     float64 `json:"wind"`
}

type directionsRequest struct {
	Stops       []directionsStopJSON      `json:"stops"`
	DepartureAt string                    `json:"departure_at"`
	SpeedModel  string                    `json:"speed_model"`
	Preferences directionsPreferencesJSON `json:"preferences"`
}

type legJSON struct {
	DistanceKm float64 `json:"distance_km"`
	DurationS  float64 `json:"duration_s"`
	ETAAt      string  `json:"eta_at"`
}

type directionsResponse struct {
	TotalDistanceKm float64           `json:"total_distance_km"`
	TotalDurationS  float64           `json:"total_duration_s"`
	Legs            []legJSON         `json:"legs"`
	Geometry        app.GeoJSONLineString `json:"geometry"`
}

// --- Handler ---

// Directions handles POST /api/v1/routing/directions.
func (h *RoutingHandler) Directions(w http.ResponseWriter, r *http.Request) {
	var req directionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Stops) < 2 {
		writeError(w, http.StatusBadRequest, "at least 2 stops are required")
		return
	}

	departureAt := time.Now()
	if req.DepartureAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.DepartureAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339 format")
			return
		}
		departureAt = parsed
	}

	speedModel := plan.SpeedModelElevation
	if req.SpeedModel == string(plan.SpeedModelFlat) {
		speedModel = plan.SpeedModelFlat
	}

	stops := make([]plan.StopPoint, len(req.Stops))
	for i, s := range req.Stops {
		stops[i] = plan.StopPoint{
			Lat:       s.Lat,
			Lon:       s.Lon,
			Type:      plan.StopType(s.Type),
			SortOrder: i,
		}
	}

	result, err := h.svc.GetDirections(app.DirectionsRequest{
		Stops:       stops,
		DepartureAt: departureAt,
		SpeedModel:  speedModel,
		Preferences: plan.Preferences{
			ShadeWeight:    req.Preferences.Shade,
			GreeneryWeight: req.Preferences.Greenery,
			WindWeight:     req.Preferences.Wind,
		},
	})
	if err != nil {
		if errors.Is(err, app.ErrTooFewDirectionStops) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, "routing engine error")
		return
	}

	legs := make([]legJSON, len(result.Legs))
	for i, l := range result.Legs {
		legs[i] = legJSON{
			DistanceKm: l.DistanceKm,
			DurationS:  l.DurationS,
			ETAAt:      l.ETAAt.Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, directionsResponse{
		TotalDistanceKm: result.TotalDistanceKm,
		TotalDurationS:  result.TotalDurationS,
		Legs:            legs,
		Geometry:        result.GeoJSON,
	})
}
```

- [ ] Run the tests:
  ```bash
  cd /Users/lug/src/cyclist-map && go test ./internal/api/... -run TestRoutingHandler
  ```
  All 4 tests must pass.

- [ ] Commit:
  ```bash
  cd /Users/lug/src/cyclist-map && git add internal/api/routing_handler.go internal/api/routing_handler_test.go && git commit -m "$(cat <<'EOF'
  feat: add RoutingHandler for POST /api/v1/routing/directions

  Decodes the spec-defined stop/preferences request body, delegates to
  RoutingDirector interface, and returns GeoJSON geometry with per-leg
  ETAs. Returns 502 on routing engine failures, 400 on bad input.

  Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 4 — Wire everything into the router and main

**Goal:** Register the new endpoint on the chi router, inject the real Valhalla client through the service, and confirm the binary compiles.

**Files:**
- `internal/api/router.go` (edit)
- `cmd/api/main.go` (edit — add RoutingService construction)

### Steps

- [ ] Read `cmd/api/main.go` to understand how services are wired today, then extend it.

- [ ] Edit `internal/api/router.go` — add `RoutingHandler` parameter and register the route:

```go
// NewRouter creates the chi router with all API routes wired up.
func NewRouter(routeSvc *app.RouteService, routingHandler *api.RoutingHandler) *chi.Mux {
```

> The current signature is `func NewRouter(routeSvc *app.RouteService) *chi.Mux`. Change it to also accept `*RoutingHandler` (note: it's in the same package, so just `*RoutingHandler`).

  Exact edit to `internal/api/router.go`:

```go
// NewRouter creates the chi router with all API routes wired up.
func NewRouter(routeSvc *app.RouteService, rh *RoutingHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	routeH := &RouteHandler{svc: routeSvc}
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/routes", routeH.List)
		r.Post("/routes", routeH.Create)
		r.Get("/routes/{id}", routeH.Get)
		r.Patch("/routes/{id}", routeH.Update)
		r.Delete("/routes/{id}", routeH.Archive)

		r.Post("/routing/directions", rh.Directions)
	})
	return r
}
```

- [ ] Edit `cmd/api/main.go` to construct `valhalla.Client`, `RoutingService`, and `RoutingHandler`, then pass `RoutingHandler` to `NewRouter`. The Valhalla URL must come from an environment variable with a default:

```go
valhallaURL := os.Getenv("VALHALLA_URL")
if valhallaURL == "" {
    valhallaURL = "http://localhost:8002"
}
valhallaClient := valhalla.NewClient(valhallaURL)
routingSvc := app.NewRoutingService(valhallaClient)
routingHandler := api.NewRoutingHandler(routingSvc)

router := api.NewRouter(routeSvc, routingHandler)
```

  Add imports: `"github.com/cyclist-map/cyclist-map/internal/infra/valhalla"` and ensure `app` and `api` are already imported.

- [ ] Add `VALHALLA_URL` to `docker-compose.yml` under the `api` service environment:

```yaml
services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable
      VALHALLA_URL: http://valhalla:8002
    depends_on:
      - valhalla
```

  Note: use the Docker service name `valhalla` (not `localhost`) so the API container can reach Valhalla over the Docker network.

- [ ] Verify the binary compiles with no errors:
  ```bash
  cd /Users/lug/src/cyclist-map && go build ./...
  ```

- [ ] Run the full test suite to check nothing regressed:
  ```bash
  cd /Users/lug/src/cyclist-map && go test ./...
  ```

- [ ] Commit:
  ```bash
  cd /Users/lug/src/cyclist-map && git add internal/api/router.go cmd/api/main.go docker-compose.yml && git commit -m "$(cat <<'EOF'
  feat: wire Valhalla routing into chi router and main

  Constructs valhalla.Client, RoutingService, and RoutingHandler in main,
  registers POST /api/v1/routing/directions on the chi router, and
  configures VALHALLA_URL env var in docker-compose with service-name
  networking so the api container can reach the valhalla sidecar.

  Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
  EOF
  )"
  ```

---

## Task 5 — End-to-end docker-compose verification

**Goal:** Prove the fully integrated stack works: Valhalla container up, API container built and running, real HTTP round-trip returning route geometry.

**Files:**
- `docker-compose.yml` (read-only verification)

### Steps

- [ ] Build and start all services:
  ```bash
  cd /Users/lug/src/cyclist-map && docker compose up -d --build
  ```

- [ ] Wait for the API to be ready (it starts quickly; Valhalla may still be building tiles from Task 0):
  ```bash
  until curl -sf http://localhost:8080/api/v1/routes > /dev/null; do
    echo "waiting for api…"; sleep 5
  done
  echo "api ready"
  ```

- [ ] Wait for Valhalla tiles (if not already confirmed in Task 0):
  ```bash
  until curl -sf http://localhost:8002/status | grep -q '"has_tiles":true'; do
    echo "waiting for valhalla…"; sleep 30
  done
  echo "valhalla ready"
  ```

- [ ] Fire a real directions request and assert the response contains geometry:
  ```bash
  RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/routing/directions \
    -H 'Content-Type: application/json' \
    -d '{
      "stops": [
        {"type": "manual", "lat": 35.6895, "lon": 139.6917},
        {"type": "manual", "lat": 35.6812, "lon": 139.7671}
      ],
      "departure_at": "2026-04-10T14:00:00+09:00",
      "speed_model": "elevation",
      "preferences": {"shade": 0.8, "greenery": 0.5, "wind": 0.6}
    }')

  echo "$RESPONSE" | jq .

  # Assert non-zero distance
  DIST=$(echo "$RESPONSE" | jq '.total_distance_km')
  if [ -z "$DIST" ] || [ "$DIST" = "null" ] || [ "$DIST" = "0" ]; then
    echo "FAIL: expected non-zero total_distance_km, got: $DIST"
    exit 1
  fi

  # Assert geometry is present
  COORD_COUNT=$(echo "$RESPONSE" | jq '.geometry.coordinates | length')
  if [ "$COORD_COUNT" -lt 2 ]; then
    echo "FAIL: expected at least 2 geometry coordinates, got: $COORD_COUNT"
    exit 1
  fi

  echo "PASS: directions response valid — distance=${DIST}km, coords=${COORD_COUNT}"
  ```

- [ ] Test a multi-stop request (3 stops):
  ```bash
  curl -s -X POST http://localhost:8080/api/v1/routing/directions \
    -H 'Content-Type: application/json' \
    -d '{
      "stops": [
        {"type": "manual", "lat": 35.6895, "lon": 139.6917},
        {"type": "manual", "lat": 35.6950, "lon": 139.7200},
        {"type": "manual", "lat": 35.6812, "lon": 139.7671}
      ],
      "departure_at": "2026-04-10T14:00:00+09:00",
      "speed_model": "elevation",
      "preferences": {"shade": 0.5, "greenery": 0.5, "wind": 0.3}
    }' | jq '{legs: (.legs | length), distance: .total_distance_km}'
  ```
  Expect `"legs": 2` and a non-zero distance.

- [ ] Confirm error handling — request with 1 stop returns 400:
  ```bash
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/api/v1/routing/directions \
    -H 'Content-Type: application/json' \
    -d '{"stops": [{"type": "manual", "lat": 35.68, "lon": 139.69}], "departure_at": "2026-04-10T14:00:00+09:00"}')
  [ "$STATUS" = "400" ] && echo "PASS: single-stop correctly returns 400" || echo "FAIL: expected 400, got $STATUS"
  ```

---

## Self-Review Checklist

Before declaring this work complete, verify every item:

**Code quality**
- [ ] `go vet ./...` passes with no warnings
- [ ] `go test ./...` passes — all tests green
- [ ] `go build ./...` produces no errors
- [ ] No `interface{}` / `any` used in domain or app layers (only in infra JSON marshalling)
- [ ] Valhalla client timeout is set (30s) — no hanging requests possible
- [ ] `resp.Body.Close()` called in all HTTP response paths including error branches

**Design conformance**
- [ ] `ValhallaRouter` interface defined in `internal/app/` — infra package is not imported by app tests
- [ ] `RoutingDirector` interface defined in `internal/api/` — app package is not directly depended on by handler tests
- [ ] Stops sorted by `SortOrder` before sending to Valhalla (multi-stop order is deterministic)
- [ ] Middle stops sent as `type=through` (via-points), not `break`, so Valhalla treats them as pass-through
- [ ] Polyline6 decoder handles empty string (returns empty slice, not nil panic)
- [ ] `ErrTooFewDirectionStops` exposed from `internal/app/` for handler to check with `errors.Is`

**HTTP contract (spec compliance)**
- [ ] `POST /api/v1/routing/directions` registered on the chi router under `/api/v1`
- [ ] Request body matches spec: `stops[]`, `departure_at`, `speed_model`, `preferences{shade,greenery,wind}`
- [ ] Response includes: `total_distance_km`, `total_duration_s`, `legs[]`, `geometry` (GeoJSON LineString)
- [ ] `legs[]` items include: `distance_km`, `duration_s`, `eta_at` (RFC3339)
- [ ] 400 returned for fewer than 2 stops or malformed body
- [ ] 502 returned when Valhalla is unreachable or returns an error (not 500 — this is a bad gateway)
- [ ] `departure_at` defaults to `time.Now()` when omitted (not a hard error)

**Docker / infrastructure**
- [ ] `VALHALLA_URL` env var in `docker-compose.yml` uses Docker service name `valhalla:8002` (not `localhost`)
- [ ] `api` service declares `depends_on: [valhalla]` in `docker-compose.yml`
- [ ] `VALHALLA_URL` env var read in `main.go` with `http://localhost:8002` as fallback for local dev outside Docker

**End-to-end**
- [ ] Real HTTP round-trip to `POST /api/v1/routing/directions` returns non-zero `total_distance_km`
- [ ] 3-stop request returns `"legs": 2`
- [ ] Single-stop request returns HTTP 400
