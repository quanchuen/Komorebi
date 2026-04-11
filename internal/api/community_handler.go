package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/go-chi/chi/v5"
)

// CommunityService is the interface the handler calls. Matches app.CommunityService.
type CommunityService interface {
	SubmitContribution(userID string, geometry [][3]float64, metadata map[string]any) (*community.Contribution, error)
	AddReview(userID, routeID string, rating int, body string) (*community.Review, error)
	ListReviews(routeID string) ([]*community.Review, error)
	LogRide(userID, routeID string, riddenAt, durationS int, gpxTrack [][3]float64) (*community.RideLog, error)
	ListUserRideLogs(userID string) ([]*community.RideLog, error)
}

// CommunityHandler handles community endpoints.
type CommunityHandler struct {
	svc CommunityService
}

// NewCommunityHandler creates a CommunityHandler.
func NewCommunityHandler(svc CommunityService) *CommunityHandler {
	return &CommunityHandler{svc: svc}
}

// --- request / response types ---

type submitContributionRequest struct {
	Geometry [][3]float64   `json:"geometry"`
	Metadata map[string]any `json:"metadata"`
}

type contributionResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submitted_at"`
}

type addReviewRequest struct {
	Rating int    `json:"rating"`
	Body   string `json:"body"`
}

type reviewResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	RouteID   string `json:"route_id"`
	Rating    int    `json:"rating"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type listReviewsResponse struct {
	Reviews []reviewResponse `json:"reviews"`
}

type logRideRequest struct {
	RiddenAt  int64        `json:"ridden_at"`  // Unix timestamp
	DurationS int          `json:"duration_s"` // seconds
	GPXTrack  [][3]float64 `json:"gpx_track"`  // optional
}

type rideLogResponse struct {
	ID        string       `json:"id"`
	UserID    string       `json:"user_id"`
	RouteID   string       `json:"route_id"`
	RiddenAt  int64        `json:"ridden_at"`
	DurationS int          `json:"duration_s"`
	GPXTrack  [][3]float64 `json:"gpx_track,omitempty"`
	CreatedAt string       `json:"created_at"`
}

type listRideLogsResponse struct {
	RideLogs []rideLogResponse `json:"ride_logs"`
}

// --- handlers ---

// SubmitContribution handles POST /api/v1/contributions (authenticated).
func (h *CommunityHandler) SubmitContribution(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req submitContributionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := h.svc.SubmitContribution(userID, req.Geometry, req.Metadata)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to submit contribution")
		return
	}
	writeJSON(w, http.StatusCreated, contributionResponse{
		ID:          c.ID,
		UserID:      c.UserID,
		Status:      string(c.Status),
		SubmittedAt: c.SubmittedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ListReviews handles GET /api/v1/routes/:id/reviews (public).
func (h *CommunityHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	routeID := chi.URLParam(r, "id")
	reviews, err := h.svc.ListReviews(routeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reviews")
		return
	}
	resp := listReviewsResponse{Reviews: make([]reviewResponse, len(reviews))}
	for i, rev := range reviews {
		resp.Reviews[i] = toReviewResponse(rev)
	}
	writeJSON(w, http.StatusOK, resp)
}

// AddReview handles POST /api/v1/routes/:id/reviews (authenticated).
func (h *CommunityHandler) AddReview(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	routeID := chi.URLParam(r, "id")
	var req addReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rev, err := h.svc.AddReview(userID, routeID, req.Rating, req.Body)
	if err != nil {
		if errors.Is(err, community.ErrInvalidRating) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add review")
		return
	}
	writeJSON(w, http.StatusCreated, toReviewResponse(rev))
}

// LogRide handles POST /api/v1/routes/:id/ride-logs (authenticated).
func (h *CommunityHandler) LogRide(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	routeID := chi.URLParam(r, "id")
	var req logRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rl, err := h.svc.LogRide(userID, routeID, int(req.RiddenAt), req.DurationS, req.GPXTrack)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to log ride")
		return
	}
	writeJSON(w, http.StatusCreated, toRideLogResponse(rl))
}

// ListUserRideLogs handles GET /api/v1/users/:id/ride-logs (public).
func (h *CommunityHandler) ListUserRideLogs(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	logs, err := h.svc.ListUserRideLogs(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list ride logs")
		return
	}
	resp := listRideLogsResponse{RideLogs: make([]rideLogResponse, len(logs))}
	for i, rl := range logs {
		resp.RideLogs[i] = toRideLogResponse(rl)
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- helpers ---

func toReviewResponse(rev *community.Review) reviewResponse {
	return reviewResponse{
		ID:        rev.ID,
		UserID:    rev.UserID,
		RouteID:   rev.RouteID,
		Rating:    rev.Rating,
		Body:      rev.Body,
		CreatedAt: rev.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toRideLogResponse(rl *community.RideLog) rideLogResponse {
	return rideLogResponse{
		ID:        rl.ID,
		UserID:    rl.UserID,
		RouteID:   rl.RouteID,
		RiddenAt:  int64(rl.RiddenAt),
		DurationS: rl.DurationS,
		GPXTrack:  rl.GPXTrack,
		CreatedAt: rl.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
