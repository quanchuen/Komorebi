package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/cyclist-map/cyclist-map/internal/domain/community"
)

// AuthHandler handles registration, login, and token refresh.
type AuthHandler struct {
	svc *app.AuthService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc *app.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// --- request / response types ---

type registerRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type registerResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	CreatedAt   string `json:"created_at"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// --- handlers ---

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u, err := h.svc.Register(req.DisplayName, req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, community.ErrEmptyDisplayName):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrEmailTaken):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}
	writeJSON(w, http.StatusCreated, registerResponse{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tokens, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, app.ErrBadCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tokens, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		if errors.Is(err, app.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		writeError(w, http.StatusInternalServerError, "refresh failed")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}
