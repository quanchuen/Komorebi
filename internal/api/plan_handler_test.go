// internal/api/plan_handler_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/domain/plan"
	"github.com/go-chi/chi/v5"
)

// --- stub plan director ---

type stubPlanDirector struct {
	plan *plan.RoutePlan
	err  error
}

func (s *stubPlanDirector) CreatePlan(_ app.CreatePlanRequest) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) GetPlan(_ string) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) AddStop(_ app.AddStopRequest) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) RemoveStop(_, _ string) (*plan.RoutePlan, error) {
	return s.plan, s.err
}
func (s *stubPlanDirector) AddTask(_ app.AddTaskRequest) (*plan.RoutePlan, error) {
	return s.plan, s.err
}

func sampleHandlerPlan() *plan.RoutePlan {
	p := plan.NewRoutePlan("user-1")
	p.SpeedModel = plan.SpeedModelFlat
	p.Stops = []plan.StopPoint{}
	p.Tasks = []plan.PlanTask{}
	return p
}

// --- tests ---

func TestPlanHandler_CreatePlan_OK(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
	h := api.NewPlanHandler(stub)

	body := `{"user_id":"user-1","speed_model":"flat"}`
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
	if resp["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestPlanHandler_CreatePlan_MissingUserID(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
	h := api.NewPlanHandler(stub)

	body := `{"speed_model":"flat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreatePlan(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPlanHandler_GetPlan_OK(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Get("/api/v1/plans/{id}", h.GetPlan)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans/test-plan-id", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlanHandler_GetPlan_NotFound(t *testing.T) {
	stub := &stubPlanDirector{err: app.ErrPlanNotFound}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Get("/api/v1/plans/{id}", h.GetPlan)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans/nonexistent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestPlanHandler_AddStop_OK(t *testing.T) {
	p := sampleHandlerPlan()
	p.Stops = []plan.StopPoint{{ID: "s1", Lat: 35.67, Lon: 139.65, Type: plan.StopManual}}
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
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stops, ok := resp["stops"].([]any)
	if !ok || len(stops) != 1 {
		t.Fatalf("expected 1 stop in response, got %v", resp["stops"])
	}
}

func TestPlanHandler_AddStop_MissingLatLon(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Post("/api/v1/plans/{id}/stops", h.AddStop)

	body := `{"type":"manual"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans/plan-1/stops", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPlanHandler_RemoveStop_OK(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
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

func TestPlanHandler_RemoveStop_NotFound(t *testing.T) {
	stub := &stubPlanDirector{err: app.ErrPlanNotFound}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Delete("/api/v1/plans/{id}/stops/{stop_id}", h.RemoveStop)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/plans/bad-id/stops/stop-1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestPlanHandler_AddTask_OK(t *testing.T) {
	p := sampleHandlerPlan()
	p.Tasks = []plan.PlanTask{{ID: "t1", Description: "Stop at #cafe", Hashtag: "#cafe", Status: plan.TaskUnresolved}}
	stub := &stubPlanDirector{plan: p}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Post("/api/v1/plans/{id}/tasks", h.AddTask)

	body := `{"description":"Stop at #cafe"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans/plan-1/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlanHandler_AddTask_MissingDescription(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
	h := api.NewPlanHandler(stub)

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

func TestPlanHandler_CreatePlanFromRoute_OK(t *testing.T) {
	stub := &stubPlanDirector{plan: sampleHandlerPlan()}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Post("/api/v1/routes/{id}/plans", h.CreatePlanFromRoute)

	body := `{"user_id":"user-1","speed_model":"flat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-123/plans", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPlanHandler_CreatePlanFromRoute_ServiceError(t *testing.T) {
	stub := &stubPlanDirector{err: errors.New("some error")}
	h := api.NewPlanHandler(stub)

	r := chi.NewRouter()
	r.Post("/api/v1/routes/{id}/plans", h.CreatePlanFromRoute)

	body := `{"user_id":"user-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-123/plans", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
