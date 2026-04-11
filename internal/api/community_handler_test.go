package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
	"github.com/go-chi/chi/v5"
)

// --- stub community service ---

type stubCommunityService struct {
	contributions []*community.Contribution
	reviews       []*community.Review
	rideLogs      []*community.RideLog
}

func (s *stubCommunityService) SubmitContribution(userID string, geometry [][3]float64, metadata map[string]any) (*community.Contribution, error) {
	c := community.NewContribution(userID, geometry, metadata)
	s.contributions = append(s.contributions, c)
	return c, nil
}

func (s *stubCommunityService) AddReview(userID, routeID string, rating int, body string) (*community.Review, error) {
	r, err := community.NewReview(userID, routeID, rating, body)
	if err != nil {
		return nil, err
	}
	s.reviews = append(s.reviews, r)
	return r, nil
}

func (s *stubCommunityService) ListReviews(routeID string) ([]*community.Review, error) {
	var out []*community.Review
	for _, r := range s.reviews {
		if r.RouteID == routeID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *stubCommunityService) LogRide(userID, routeID string, riddenAt, durationS int, gpxTrack [][3]float64) (*community.RideLog, error) {
	rl, err := community.NewRideLog(userID, routeID, riddenAt, durationS)
	if err != nil {
		return nil, err
	}
	if len(gpxTrack) > 0 {
		rl.SetGPXTrack(gpxTrack)
	}
	s.rideLogs = append(s.rideLogs, rl)
	return rl, nil
}

func (s *stubCommunityService) ListUserRideLogs(userID string) ([]*community.RideLog, error) {
	var out []*community.RideLog
	for _, rl := range s.rideLogs {
		if rl.UserID == userID {
			out = append(out, rl)
		}
	}
	return out, nil
}

// --- test helpers ---

func ctxWithUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), api.ExportedCtxUserID, userID)
	return r.WithContext(ctx)
}

func chiCtxWithParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- tests ---

func TestCommunityHandler_SubmitContribution(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"geometry":[[139.69,35.69,10],[139.70,35.70,15]],"metadata":{"surface":"gravel"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contributions", bytes.NewBufferString(body))
	req = ctxWithUser(req, "user-123")
	rr := httptest.NewRecorder()
	h.SubmitContribution(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_SubmitContribution_Unauthenticated(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"geometry":[[139.69,35.69,10],[139.70,35.70,15]]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contributions", bytes.NewBufferString(body))
	// No user in context
	rr := httptest.NewRecorder()
	h.SubmitContribution(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCommunityHandler_AddReview(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"rating":4,"body":"Great ride!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-1/reviews", bytes.NewBufferString(body))
	req = ctxWithUser(req, "user-123")
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.AddReview(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_AddReview_InvalidRating(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	body := `{"rating":6,"body":"too high"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-1/reviews", bytes.NewBufferString(body))
	req = ctxWithUser(req, "user-123")
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.AddReview(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCommunityHandler_ListReviews(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)
	// Pre-seed a review.
	rev, _ := community.NewReview("user-1", "route-1", 5, "Perfect!")
	stub.reviews = append(stub.reviews, rev)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/route-1/reviews", nil)
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.ListReviews(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	reviews, ok := resp["reviews"].([]any)
	if !ok || len(reviews) != 1 {
		t.Errorf("expected 1 review, got %v", resp)
	}
}

func TestCommunityHandler_LogRide(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)

	now := time.Now().Unix()
	body, _ := json.Marshal(map[string]any{
		"ridden_at":  now,
		"duration_s": 3600,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/route-1/ride-logs", bytes.NewReader(body))
	req = ctxWithUser(req, "user-123")
	req = chiCtxWithParam(req, "id", "route-1")
	rr := httptest.NewRecorder()
	h.LogRide(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_ListUserRideLogs(t *testing.T) {
	stub := &stubCommunityService{}
	h := api.NewCommunityHandler(stub)
	rl, _ := community.NewRideLog("user-1", "route-1", 1744300000, 7200)
	stub.rideLogs = append(stub.rideLogs, rl)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1/ride-logs", nil)
	req = chiCtxWithParam(req, "id", "user-1")
	rr := httptest.NewRecorder()
	h.ListUserRideLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	logs, ok := resp["ride_logs"].([]any)
	if !ok || len(logs) != 1 {
		t.Errorf("expected 1 ride log, got %v", resp)
	}
}
