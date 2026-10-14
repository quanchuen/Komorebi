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
	ID          string              `json:"id"`
	UserID      string              `json:"user_id"`
	DepartureAt string              `json:"departure_at"`
	SpeedModel  string              `json:"speed_model"`
	Stops       []stopPointResponse `json:"stops"`
	Tasks       []planTaskResponse  `json:"tasks"`
	RouteWKT    string              `json:"route_wkt,omitempty"`
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
