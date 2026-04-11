package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
	"github.com/cyclist-map/cyclist-map/internal/domain/plan"
	"github.com/cyclist-map/cyclist-map/internal/domain/route"
	"github.com/cyclist-map/cyclist-map/internal/infra/postgres"
)

// ConditionsQuerier is the read interface the handler needs from the route service.
type ConditionsQuerier interface {
	GetByID(id string) (*route.Route, error)
}

// ConditionsComputer is the interface the handler uses to project conditions.
type ConditionsComputer interface {
	GetRouteConditions(ctx context.Context, req app.RouteConditionsRequest) ([]app.SegmentConditionsResult, error)
}

// ConditionsHandler handles the route conditions and preview endpoints.
type ConditionsHandler struct {
	routes ConditionsQuerier
	env    ConditionsComputer
}

// NewConditionsHandler creates a ConditionsHandler.
func NewConditionsHandler(routes ConditionsQuerier, env ConditionsComputer) *ConditionsHandler {
	return &ConditionsHandler{routes: routes, env: env}
}

// --- Response types ---

type greenWaveJSON struct {
	SpeedKmh float64 `json:"speed_kmh"`
	LengthKm float64 `json:"length_km,omitempty"`
}

type segmentConditionJSON struct {
	Km          float64        `json:"km"`
	ETA         string         `json:"eta"`
	Shade       float64        `json:"shade"`
	WindBenefit float64        `json:"wind_benefit"`
	Precip      float64        `json:"precip"`
	GreenWave   *greenWaveJSON `json:"green_wave,omitempty"`
	Signals     int            `json:"signals"`
	Colors      struct {
		Shade string `json:"shade"`
		Wind  string `json:"wind"`
		Rain  string `json:"rain"`
	} `json:"colors"`
}

type routeConditionsResponse struct {
	RouteID  string                 `json:"route_id"`
	Segments []segmentConditionJSON `json:"segments"`
}

// RouteConditions handles GET /api/v1/routes/:id/conditions
func (h *ConditionsHandler) RouteConditions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	departureAt := time.Now()
	if raw := r.URL.Query().Get("departure_at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339 format")
			return
		}
		departureAt = parsed
	}

	speedModel := plan.SpeedModelElevation
	if r.URL.Query().Get("speed_model") == string(plan.SpeedModelFlat) {
		speedModel = plan.SpeedModelFlat
	}

	rt, err := h.routes.GetByID(id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load route")
		return
	}

	conditions, err := h.env.GetRouteConditions(r.Context(), app.RouteConditionsRequest{
		Route:       rt,
		DepartureAt: departureAt,
		SpeedModel:  speedModel,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute conditions")
		return
	}

	segs := make([]segmentConditionJSON, len(conditions))
	for i, c := range conditions {
		sj := segmentConditionJSON{
			Km:          c.Km,
			ETA:         c.ETA.Format("15:04"),
			Shade:       c.Shade,
			WindBenefit: c.WindBenefit,
			Precip:      c.Precip,
			Signals:     c.SignalCount,
		}
		sj.Colors.Shade = c.ShadeColor
		sj.Colors.Wind = c.WindColor
		sj.Colors.Rain = c.RainColor
		if c.GreenWave != nil {
			sj.GreenWave = &greenWaveJSON{SpeedKmh: c.GreenWave.TargetSpeedKmh}
		}
		segs[i] = sj
	}

	writeJSON(w, http.StatusOK, routeConditionsResponse{
		RouteID:  id,
		Segments: segs,
	})
}

// --- Preview handler ---

// ConditionsPreviewQuerier is the repo interface for the preview endpoint.
type ConditionsPreviewQuerier interface {
	ConditionsPreview(ctx context.Context, bbox [4]float64, at time.Time) ([]postgres.ConditionsPreviewCell, error)
}

// PreviewHandler handles the heatmap preview endpoint.
type PreviewHandler struct {
	repo ConditionsPreviewQuerier
}

// NewPreviewHandler creates a PreviewHandler.
func NewPreviewHandler(repo ConditionsPreviewQuerier) *PreviewHandler {
	return &PreviewHandler{repo: repo}
}

type previewCellJSON struct {
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	Shade       float64 `json:"shade"`
	WindBenefit float64 `json:"wind_benefit"`
	Precip      float64 `json:"precip"`
	Colors      struct {
		Shade string `json:"shade"`
		Wind  string `json:"wind"`
		Rain  string `json:"rain"`
	} `json:"colors"`
}

// ConditionsPreview handles GET /api/v1/routing/conditions/preview
func (h *PreviewHandler) ConditionsPreview(w http.ResponseWriter, r *http.Request) {
	bboxStr := r.URL.Query().Get("bbox")
	if bboxStr == "" {
		writeError(w, http.StatusBadRequest, "bbox parameter required (minLon,minLat,maxLon,maxLat)")
		return
	}
	var bbox [4]float64
	_, err := fmt.Sscanf(bboxStr, "%f,%f,%f,%f", &bbox[0], &bbox[1], &bbox[2], &bbox[3])
	if err != nil {
		writeError(w, http.StatusBadRequest, "bbox must be minLon,minLat,maxLon,maxLat")
		return
	}

	departureAt := time.Now()
	if raw := r.URL.Query().Get("departure_at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "departure_at must be RFC3339 format")
			return
		}
		departureAt = parsed
	}

	cells, err := h.repo.ConditionsPreview(r.Context(), bbox, departureAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load conditions preview")
		return
	}

	out := make([]previewCellJSON, len(cells))
	for i, c := range cells {
		pj := previewCellJSON{
			Lon:         c.Lon,
			Lat:         c.Lat,
			Shade:       c.Shade,
			WindBenefit: c.WindBenefit,
			Precip:      c.Precip,
		}
		pj.Colors.Shade = environment.ShadeColor(c.Shade)
		pj.Colors.Wind = environment.WindColor(c.WindBenefit)
		pj.Colors.Rain = environment.RainColor(c.Precip)
		out[i] = pj
	}

	writeJSON(w, http.StatusOK, map[string]any{"cells": out})
}
