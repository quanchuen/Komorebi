package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
)

func makeAuthService(t *testing.T) *app.AuthService {
	t.Helper()
	repo := newStubAuthRepo()
	svc, err := app.NewAuthService(repo, "test-secret-32-chars-padding-here", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	return svc
}

func TestAuthMiddleware_NoBearerToken(t *testing.T) {
	svc := makeAuthService(t)
	mw := api.AuthMiddleware(svc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	svc := makeAuthService(t)
	mw := api.AuthMiddleware(svc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidToken_InjectsUserID(t *testing.T) {
	repo := newStubAuthRepo()
	svc2, _ := app.NewAuthService(repo, "test-secret-32-chars-padding-here", 15*time.Minute, 7*24*time.Hour)

	// Register and login to get a real token.
	u, _ := svc2.Register("Test User", "test@example.com", "password123")
	tokens, _ := svc2.Login("test@example.com", "password123")

	mw := api.AuthMiddleware(svc2)
	var gotUserID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = api.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if gotUserID != u.ID {
		t.Errorf("userID in context: want %q, got %q", u.ID, gotUserID)
	}
}
