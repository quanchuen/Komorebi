// internal/api/discovery_handler.go
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/discovery"
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
