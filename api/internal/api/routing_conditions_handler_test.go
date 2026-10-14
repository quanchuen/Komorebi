package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"komorebi/internal/app"
	"komorebi/internal/domain/environment"
)

// recordingConditionsComputer captures the request so tests can assert the
// ad-hoc route was segmented before reaching the environment service.
type recordingConditionsComputer struct {
	results []app.SegmentConditionsResult
	err     error
	got     *app.RouteConditionsRequest
}

func (s *recordingConditionsComputer) GetRouteConditions(_ context.Context, req app.RouteConditionsRequest) ([]app.SegmentConditionsResult, error) {
	s.got = &req
	return s.results, s.err
}

func postRoutingConditions(t *testing.T, h *ConditionsHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.RoutingConditions(rec, req)
	return rec
}

func TestRoutingConditionsHappyPath(t *testing.T) {
	env := &recordingConditionsComputer{results: []app.SegmentConditionsResult{
		{
			SegmentConditions: environment.SegmentConditions{
				Km:          0,
				Shade:       0.7,
				WindBenefit: 0.2,
				Precip:      0,
				ETA:         time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC),
				SignalCount: 3,
			},
			ShadeColor: "#111111",
			WindColor:  "#222222",
			RainColor:  "#333333",
		},
	}}
	h := NewConditionsHandler(&stubRouteGetter{}, env)

	body := `{
		"geometry": {"type": "LineString", "coordinates": [[139.70,35.60],[139.70,35.61],[139.70,35.62]]},
		"elevation_profile": [{"distance_m":0,"elevation_m":10},{"distance_m":2200,"elevation_m":30}],
		"departure_at": "2026-08-05T09:00:00Z"
	}`
	rec := postRoutingConditions(t, h, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	if env.got == nil {
		t.Fatal("environment service not called")
	}
	if len(env.got.Route.Segments) == 0 {
		t.Fatal("ad-hoc route has no segments")
	}
	if !env.got.DepartureAt.Equal(time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("departure not passed through: %v", env.got.DepartureAt)
	}

	var resp struct {
		Segments []struct {
			Shade   float64 `json:"shade"`
			Signals int     `json:"signals"`
			Colors  struct {
				Shade string `json:"shade"`
			} `json:"colors"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Segments) != 1 || resp.Segments[0].Shade != 0.7 || resp.Segments[0].Signals != 3 {
		t.Fatalf("unexpected segments: %+v", resp.Segments)
	}
	if resp.Segments[0].Colors.Shade != "#111111" {
		t.Fatalf("colors not mapped: %+v", resp.Segments[0].Colors)
	}
}

func TestRoutingConditionsRejectsBadInput(t *testing.T) {
	h := NewConditionsHandler(&stubRouteGetter{}, &recordingConditionsComputer{})

	cases := map[string]string{
		"malformed json": `{`,
		"too few coords": `{"geometry":{"type":"LineString","coordinates":[[139.7,35.6]]}}`,
		"bad departure":  `{"geometry":{"type":"LineString","coordinates":[[139.7,35.6],[139.7,35.61]]},"departure_at":"tomorrow"}`,
	}
	for name, body := range cases {
		if rec := postRoutingConditions(t, h, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, rec.Code)
		}
	}
}

func TestRoutingConditionsComputeFailure(t *testing.T) {
	h := NewConditionsHandler(&stubRouteGetter{}, &recordingConditionsComputer{err: context.DeadlineExceeded})
	body := `{"geometry":{"type":"LineString","coordinates":[[139.7,35.6],[139.7,35.61]]}}`
	if rec := postRoutingConditions(t, h, body); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}
