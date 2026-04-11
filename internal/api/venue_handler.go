// internal/api/venue_handler.go
package api

import (
	"net/http"
	"strconv"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/environment"
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
