package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
	"komorebi/internal/domain/route"
	"komorebi/internal/infra/postgres"
)

// --- stubs ---

type stubRouteGetter struct {
	rt  *route.Route
	err error
}

func (s *stubRouteGetter) GetByID(_ string) (*route.Route, error) {
	return s.rt, s.err
}

type stubConditionsComputer struct {
	results []app.SegmentConditionsResult
	err     error
}

func (s *stubConditionsComputer) GetRouteConditions(_ context.Context, _ app.RouteConditionsRequest) ([]app.SegmentConditionsResult, error) {
	return s.results, s.err
}

func routeConditionsRequest(routeID, departureAt, speedModel string) *http.Request {
	url := "/api/v1/routes/" + routeID + "/conditions"
	if departureAt != "" || speedModel != "" {
		url += "?"
		if departureAt != "" {
			url += "departure_at=" + departureAt
		}
		if speedModel != "" {
			if departureAt != "" {
				url += "&"
			}
			url += "speed_model=" + speedModel
		}
	}
	r := httptest.NewRequest(http.MethodGet, url, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", routeID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestConditionsHandler_RouteNotFound(t *testing.T) {
	h := NewConditionsHandler(
		&stubRouteGetter{err: postgres.ErrNotFound},
		&stubConditionsComputer{},
	)
	w := httptest.NewRecorder()
	h.RouteConditions(w, routeConditionsRequest("bad-id", "", ""))
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestConditionsHandler_Success(t *testing.T) {
	rt := &route.Route{ID: "route-1"}
	departure := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	comp := &stubConditionsComputer{
		results: []app.SegmentConditionsResult{
			{
				SegmentConditions: environment.SegmentConditions{
					Km:          0,
					Shade:       0.7,
					WindBenefit: 0.3,
					Precip:      0.0,
					ETA:         departure,
					SignalCount: 1,
				},
				ShadeColor: "#abc123",
				WindColor:  "#22c55e",
				RainColor:  "#f8fafc",
			},
		},
	}
	h := NewConditionsHandler(&stubRouteGetter{rt: rt}, comp)
	req := routeConditionsRequest("route-1", "2026-04-10T14:00:00Z", "elevation")
	w := httptest.NewRecorder()
	h.RouteConditions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp routeConditionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RouteID != "route-1" {
		t.Errorf("route_id: got %q, want %q", resp.RouteID, "route-1")
	}
	if len(resp.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d", len(resp.Segments))
	}
	seg := resp.Segments[0]
	if seg.Shade != 0.7 {
		t.Errorf("shade: got %v, want 0.7", seg.Shade)
	}
	if seg.Colors.Shade != "#abc123" {
		t.Errorf("color.shade: got %q, want #abc123", seg.Colors.Shade)
	}
}

func TestConditionsHandler_InvalidDepartureAt(t *testing.T) {
	h := NewConditionsHandler(&stubRouteGetter{rt: &route.Route{ID: "x"}}, &stubConditionsComputer{})
	req := routeConditionsRequest("x", "not-a-date", "")
	w := httptest.NewRecorder()
	h.RouteConditions(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
