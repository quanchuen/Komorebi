package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"komorebi/internal/app"
	"komorebi/internal/domain/plan"
)

type stubIntentService struct {
	result  *app.RouteIntentResult
	err     error
	gotText string
	gotBase plan.Preferences
}

func (s *stubIntentService) InterpretIntent(_ context.Context, text string, base plan.Preferences) (*app.RouteIntentResult, error) {
	s.gotText = text
	s.gotBase = base
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func postIntent(t *testing.T, h *RoutingIntentHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routing/intent", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Interpret(rec, req)
	return rec
}

func TestIntentHandlerHappyPath(t *testing.T) {
	shade := 0.9
	svc := &stubIntentService{result: &app.RouteIntentResult{
		Intent: plan.RouteIntent{
			SchemaVersion: plan.RouteIntentSchemaVersion,
			Summary:       "Shady ride",
			OriginalText:  "shady please",
			Preferences:   plan.RouteIntentPreferences{Shade: &shade},
			Provenance:    plan.RouteIntentProvenance{Model: "claude-opus-5", PromptVersion: "v"},
		},
		Preferences: plan.Preferences{ShadeWeight: 0.9, GreeneryWeight: 0.5, WindWeight: 0.5},
		Applied:     []string{"shade"},
		Unsupported: []string{"max_grade_percent"},
	}}
	h := NewRoutingIntentHandler(svc)

	rec := postIntent(t, h, `{"text":"shady please","preferences":{"shade":0.5,"greenery":0.5,"wind":0.5}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if svc.gotText != "shady please" || svc.gotBase.ShadeWeight != 0.5 {
		t.Fatalf("service received text %q base %+v", svc.gotText, svc.gotBase)
	}

	var resp struct {
		Intent struct {
			SchemaVersion string   `json:"schema_version"`
			Summary       string   `json:"summary"`
			Model         string   `json:"model"`
			Preferences   struct{ Shade *float64 } `json:"preferences"`
		} `json:"intent"`
		Preferences struct{ Shade float64 } `json:"preferences"`
		Applied     []string                `json:"applied"`
		Unsupported []string                `json:"unsupported"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Intent.SchemaVersion != plan.RouteIntentSchemaVersion || resp.Intent.Summary != "Shady ride" {
		t.Fatalf("unexpected intent: %+v", resp.Intent)
	}
	if resp.Intent.Preferences.Shade == nil || *resp.Intent.Preferences.Shade != 0.9 {
		t.Fatalf("intent shade missing: %+v", resp.Intent.Preferences)
	}
	if resp.Preferences.Shade != 0.9 {
		t.Fatalf("merged preferences wrong: %+v", resp.Preferences)
	}
	if len(resp.Applied) != 1 || len(resp.Unsupported) != 1 {
		t.Fatalf("applied/unsupported wrong: %v %v", resp.Applied, resp.Unsupported)
	}
}

func TestIntentHandlerBadJSON(t *testing.T) {
	h := NewRoutingIntentHandler(&stubIntentService{})
	if rec := postIntent(t, h, `{`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestIntentHandlerInvalidText(t *testing.T) {
	h := NewRoutingIntentHandler(&stubIntentService{err: app.ErrIntentTextInvalid})
	if rec := postIntent(t, h, `{"text":""}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestIntentHandlerUnavailable(t *testing.T) {
	h := NewRoutingIntentHandler(&stubIntentService{err: app.ErrIntentUnavailable})
	if rec := postIntent(t, h, `{"text":"shady"}`); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestIntentHandlerAdapterFailure(t *testing.T) {
	h := NewRoutingIntentHandler(&stubIntentService{err: context.DeadlineExceeded})
	if rec := postIntent(t, h, `{"text":"shady"}`); rec.Code != http.StatusBadGateway {
		t.Fatalf("status %d", rec.Code)
	}
}
