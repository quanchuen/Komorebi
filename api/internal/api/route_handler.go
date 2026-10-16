package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"komorebi/internal/app"
	"komorebi/internal/domain/route"
	"github.com/go-chi/chi/v5"
)

// RouteHandler handles HTTP requests for Route resources.
type RouteHandler struct {
	svc *app.RouteService
}

// --- Request / Response types ---

type waypointJSON struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	SortOrder int     `json:"sort_order"`
}

type segmentJSON struct {
	Geometry     [][3]float64 `json:"geometry"`
	SurfaceType  string       `json:"surface_type"`
	GradePercent float64      `json:"grade_percent"`
	SegmentOrder int          `json:"segment_order"`
}

type routeResponse struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Geometry       [][3]float64   `json:"geometry"`
	DistanceM      float64        `json:"distance_m"`
	ElevationGainM float64        `json:"elevation_gain_m"`
	ElevationLossM float64        `json:"elevation_loss_m"`
	Difficulty     string         `json:"difficulty"`
	Status         string         `json:"status"`
	Featured       bool           `json:"featured"`
	CreatorID      string         `json:"creator_id"`
	Tags           []string       `json:"tags"`
	Waypoints      []waypointJSON `json:"waypoints"`
	Segments       []segmentJSON  `json:"segments"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

type listRoutesResponse struct {
	Routes     []routeResponse `json:"routes"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// --- Handler methods ---

// Get handles GET /api/v1/routes/{id}.
func (h *RouteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rt, err := h.svc.GetRoute(id)
	if err != nil {
		if errors.Is(err, app.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get route")
		return
	}
	writeJSON(w, http.StatusOK, toRouteResponse(rt))
}

// List handles GET /api/v1/routes.
func (h *RouteHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := route.ListParams{}

	// bbox=minLon,minLat,maxLon,maxLat
	if bbox := q.Get("bbox"); bbox != "" {
		parts := strings.Split(bbox, ",")
		if len(parts) == 4 {
			var vals [4]float64
			ok := true
			for i, p := range parts {
				v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
				if err != nil {
					ok = false
					break
				}
				vals[i] = v
			}
			if ok {
				params.BBox = vals
			}
		}
	}

	if d := q.Get("difficulty"); d != "" {
		params.Difficulty = route.Difficulty(d)
	}
	if s := q.Get("surface"); s != "" {
		params.Surface = route.SurfaceType(s)
	}
	if tags := q.Get("tags"); tags != "" {
		params.Tags = strings.Split(tags, ",")
	}
	if v := q.Get("min_distance"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			params.MinDistM = f
		}
	}
	if v := q.Get("max_distance"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			params.MaxDistM = f
		}
	}
	if c := q.Get("cursor"); c != "" {
		params.Cursor = c
	}
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			params.Limit = n
		}
	}

	result, err := h.svc.ListRoutes(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list routes")
		return
	}

	resp := listRoutesResponse{
		Routes:     make([]routeResponse, len(result.Routes)),
		NextCursor: result.NextCursor,
	}
	for i, rt := range result.Routes {
		resp.Routes[i] = toRouteResponse(rt)
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- helper utilities ---

func toRouteResponse(rt *route.Route) routeResponse {
	wps := make([]waypointJSON, len(rt.Waypoints))
	for i, wp := range rt.Waypoints {
		wps[i] = waypointJSON{
			Name:      wp.Name,
			Type:      string(wp.Type),
			Lat:       wp.Lat,
			Lon:       wp.Lon,
			SortOrder: wp.SortOrder,
		}
	}
	segs := make([]segmentJSON, len(rt.Segments))
	for i, seg := range rt.Segments {
		segs[i] = segmentJSON{
			Geometry:     seg.Geometry,
			SurfaceType:  string(seg.SurfaceType),
			GradePercent: seg.GradePercent,
			SegmentOrder: seg.SegmentOrder,
		}
	}
	return routeResponse{
		ID:             rt.ID,
		Name:           rt.Name,
		Description:    rt.Description,
		Geometry:       rt.Geometry,
		DistanceM:      rt.DistanceM,
		ElevationGainM: rt.ElevationGainM,
		ElevationLossM: rt.ElevationLossM,
		Difficulty:     string(rt.Difficulty),
		Status:         string(rt.Status),
		Featured:       rt.Featured,
		CreatorID:      rt.CreatorID,
		Tags:           rt.Tags,
		Waypoints:      wps,
		Segments:       segs,
		CreatedAt:      rt.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      rt.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorResponse{Error: msg})
}
