package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"komorebi/internal/app"
	"komorebi/internal/domain/plan"
)

// IntentInterpreter is the interface the handler uses to interpret ride text.
type IntentInterpreter interface {
	InterpretIntent(ctx context.Context, text string, base plan.Preferences) (*app.RouteIntentResult, error)
}

// RoutingIntentHandler handles POST /api/v1/routing/intent.
type RoutingIntentHandler struct {
	svc IntentInterpreter
}

func NewRoutingIntentHandler(svc IntentInterpreter) *RoutingIntentHandler {
	return &RoutingIntentHandler{svc: svc}
}

type intentRequest struct {
	Text        string                    `json:"text"`
	Preferences directionsPreferencesJSON `json:"preferences"`
}

type intentPreferencesJSON struct {
	Shade    *float64 `json:"shade"`
	Greenery *float64 `json:"greenery"`
	Wind     *float64 `json:"wind"`
}

type intentConstraintsJSON struct {
	MaxDetourM      *float64 `json:"max_detour_m"`
	MaxGradePercent *float64 `json:"max_grade_percent"`
}

type intentJSON struct {
	SchemaVersion   string                `json:"schema_version"`
	Summary         string                `json:"summary"`
	OriginalText    string                `json:"original_text"`
	Preferences     intentPreferencesJSON `json:"preferences"`
	Constraints     intentConstraintsJSON `json:"constraints"`
	UnresolvedTerms []string              `json:"unresolved_terms"`
	Model           string                `json:"model"`
	PromptVersion   string                `json:"prompt_version"`
}

type intentResponse struct {
	Intent      intentJSON                `json:"intent"`
	Preferences directionsPreferencesJSON `json:"preferences"`
	Applied     []string                  `json:"applied"`
	Unsupported []string                  `json:"unsupported"`
}

// Interpret handles POST /api/v1/routing/intent. It never applies routing
// changes itself: it returns the interpreted intent so the client can display
// it and then issue a normal structured directions request.
func (h *RoutingIntentHandler) Interpret(w http.ResponseWriter, r *http.Request) {
	var req intentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.svc.InterpretIntent(r.Context(), req.Text, plan.Preferences{
		ShadeWeight:    req.Preferences.Shade,
		GreeneryWeight: req.Preferences.Greenery,
		WindWeight:     req.Preferences.Wind,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrIntentUnavailable):
			writeError(w, http.StatusServiceUnavailable, "natural-language routing is not configured")
		case errors.Is(err, app.ErrIntentTextInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusBadGateway, "intent interpretation failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, intentResponse{
		Intent: intentJSON{
			SchemaVersion:   result.Intent.SchemaVersion,
			Summary:         result.Intent.Summary,
			OriginalText:    result.Intent.OriginalText,
			Preferences: intentPreferencesJSON{
				Shade:    result.Intent.Preferences.Shade,
				Greenery: result.Intent.Preferences.Greenery,
				Wind:     result.Intent.Preferences.Wind,
			},
			Constraints: intentConstraintsJSON{
				MaxDetourM:      result.Intent.Constraints.MaxDetourM,
				MaxGradePercent: result.Intent.Constraints.MaxGradePercent,
			},
			UnresolvedTerms: result.Intent.UnresolvedTerms,
			Model:           result.Intent.Provenance.Model,
			PromptVersion:   result.Intent.Provenance.PromptVersion,
		},
		Preferences: directionsPreferencesJSON{
			Shade:    result.Preferences.ShadeWeight,
			Greenery: result.Preferences.GreeneryWeight,
			Wind:     result.Preferences.WindWeight,
		},
		Applied:     result.Applied,
		Unsupported: result.Unsupported,
	})
}
