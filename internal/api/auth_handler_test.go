package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyclist-map/cyclist-map/internal/api"
	"github.com/cyclist-map/cyclist-map/internal/app"
)

func makeAuthHandler(t *testing.T) (*api.AuthHandler, *app.AuthService) {
	t.Helper()
	repo := newStubAuthRepo()
	svc, _ := app.NewAuthService(repo, "test-secret-32-chars-padding-here", 15*time.Minute, 7*24*time.Hour)
	return api.NewAuthHandler(svc), svc
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h, _ := makeAuthHandler(t)

	body := `{"display_name":"Yuki","email":"yuki@test.com","password":"goodpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected id in response")
	}
}

func TestAuthHandler_Register_EmptyPassword(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"display_name":"Yuki","email":"yuki2@test.com","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"display_name":"Yuki","email":"dup@test.com","password":"password1"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		h.Register(rr, req)
		if i == 1 && rr.Code != http.StatusConflict {
			t.Errorf("expected 409 on duplicate, got %d", rr.Code)
		}
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h, svc := makeAuthHandler(t)
	_, _ = svc.Register("Yuki", "yuki3@test.com", "pass123")

	body := `{"email":"yuki3@test.com","password":"pass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["access_token"] == "" {
		t.Error("expected access_token in response")
	}
}

func TestAuthHandler_Login_BadCredentials(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"email":"nobody@test.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	h, svc := makeAuthHandler(t)
	_, _ = svc.Register("Yuki", "yuki4@test.com", "pass123")
	tokens, _ := svc.Login("yuki4@test.com", "pass123")

	body, _ := json.Marshal(map[string]string{"refresh_token": tokens.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAuthHandler_Refresh_Invalid(t *testing.T) {
	h, _ := makeAuthHandler(t)
	body := `{"refresh_token":"garbage.token.here"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
