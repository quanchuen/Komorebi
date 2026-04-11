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
type RoutingDirector interface {
	GetDirections(req app.DirectionsRequest) (*app.MultiDirectionsResult, error)
}

// RoutingHandler handles HTTP requests for the routing endpoints.
type RoutingHandler struct {
	svc RoutingDirector
}

func NewRoutingHandler(svc RoutingDirector) *RoutingHandler {
	return &RoutingHandler{svc: svc}
}

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

type alternativeJSON struct {
	Profile         string                `json:"profile"`
	Label           string                `json:"label"`
	TotalDistanceKm float64               `json:"total_distance_km"`
	TotalDurationS  float64               `json:"total_duration_s"`
	Legs            []legJSON             `json:"legs"`
	Geometry        app.GeoJSONLineString `json:"geometry"`
}

type directionsResponse struct {
	Alternatives []alternativeJSON `json:"alternatives"`
}

// Directions handles POST /api/v1/routing/directions.
// Returns up to 3 route alternatives: suggested, fast, avoid main roads.
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

	alts := make([]alternativeJSON, len(result.Alternatives))
	for i, a := range result.Alternatives {
		legs := make([]legJSON, len(a.Legs))
		for j, l := range a.Legs {
			legs[j] = legJSON{
				DistanceKm: l.DistanceKm,
				DurationS:  l.DurationS,
				ETAAt:      l.ETAAt.Format(time.RFC3339),
			}
		}
		alts[i] = alternativeJSON{
			Profile:         a.Profile,
			Label:           a.Label,
			TotalDistanceKm: a.TotalDistanceKm,
			TotalDurationS:  a.TotalDurationS,
			Legs:            legs,
			Geometry:        a.GeoJSON,
		}
	}

	writeJSON(w, http.StatusOK, directionsResponse{Alternatives: alts})
}
