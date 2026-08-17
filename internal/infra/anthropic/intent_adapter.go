// Package anthropic implements the natural-language route-intent adapter
// (ADR 0003) on the Claude API. The model acts only as an inbound
// anti-corruption adapter: free text in, schema-validated RouteIntent out.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"komorebi/internal/domain/plan"
)

const (
	// DefaultModel is used when INTENT_MODEL is not set.
	DefaultModel = "claude-opus-5"
	// promptVersion is recorded as provenance on every interpreted intent.
	promptVersion = "2026-08-05.1"
	intentToolName = "record_route_intent"
)

const systemPrompt = `You convert a cyclist's free-text ride request into a structured route intent.

You are strictly an interpreter. You must not invent locations or coordinates, plan routes, or make routing decisions. You only record what the rider expressed, using the ` + intentToolName + ` tool.

Preference weights are 0..1 where 0.5 is neutral: "some shade" ≈ 0.7, "as shady as possible" ≈ 1.0, "don't care about shade" ≈ 0.5, "avoid tree cover" ≈ 0.1. "wind" is the importance of wind shelter; "greenery" is preference for green surroundings. Leave a preference null when the text does not express it.

Constraints: record an explicit detour budget ("no more than 2 km extra") as max_detour_m and an explicit grade limit ("nothing steeper than 8%") as max_grade_percent. Leave them null otherwise; never guess numbers.

Anything you cannot map onto these fields — place names, stops, surface types, time constraints, ambiguous phrases — goes verbatim into unresolved_terms so downstream systems can resolve or surface it. Never stretch an unsupported request onto a supported field.`

var intentSchemaProperties = map[string]any{
	"summary": map[string]any{
		"type":        "string",
		"description": "One short sentence restating what was understood from the rider's request.",
	},
	"preferences": map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"shade", "greenery", "wind"},
		"properties": map[string]any{
			"shade":    map[string]any{"type": []string{"number", "null"}, "description": "Shade preference weight 0..1, null when not expressed."},
			"greenery": map[string]any{"type": []string{"number", "null"}, "description": "Greenery preference weight 0..1, null when not expressed."},
			"wind":     map[string]any{"type": []string{"number", "null"}, "description": "Wind-shelter importance 0..1, null when not expressed."},
		},
	},
	"constraints": map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"max_detour_m", "max_grade_percent"},
		"properties": map[string]any{
			"max_detour_m":      map[string]any{"type": []string{"number", "null"}, "description": "Explicit detour budget in meters, null unless stated."},
			"max_grade_percent": map[string]any{"type": []string{"number", "null"}, "description": "Explicit maximum grade in percent, null unless stated."},
		},
	},
	"unresolved_terms": map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Verbatim phrases that do not map onto the supported fields.",
	},
}

// intentPayload mirrors the tool input schema on the wire.
type intentPayload struct {
	Summary     string `json:"summary"`
	Preferences struct {
		Shade    *float64 `json:"shade"`
		Greenery *float64 `json:"greenery"`
		Wind     *float64 `json:"wind"`
	} `json:"preferences"`
	Constraints struct {
		MaxDetourM      *float64 `json:"max_detour_m"`
		MaxGradePercent *float64 `json:"max_grade_percent"`
	} `json:"constraints"`
	UnresolvedTerms []string `json:"unresolved_terms"`
}

// IntentAdapter calls Claude to interpret ride requests. It satisfies
// app.IntentAdapter.
type IntentAdapter struct {
	client sdk.Client
	model  string
}

// NewIntentAdapter creates an adapter with the given API key. An empty model
// selects DefaultModel.
func NewIntentAdapter(apiKey, model string) *IntentAdapter {
	if model == "" {
		model = DefaultModel
	}
	return &IntentAdapter{
		client: sdk.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

// Interpret converts text into a RouteIntent. The tool call is forced and the
// schema is strict, so the model cannot answer outside the contract.
func (a *IntentAdapter) Interpret(ctx context.Context, text string) (*plan.RouteIntent, error) {
	tool := sdk.BetaToolParam{
		Name:        intentToolName,
		Description: sdk.String("Record the structured route intent extracted from the rider's request."),
		Strict:      sdk.Bool(true),
		InputSchema: sdk.BetaToolInputSchemaParam{
			Properties:  intentSchemaProperties,
			Required:    []string{"summary", "preferences", "constraints", "unresolved_terms"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	resp, err := a.client.Beta.Messages.New(ctx, sdk.BetaMessageNewParams{
		Model:     sdk.Model(a.model),
		MaxTokens: 2048,
		System:    []sdk.BetaTextBlockParam{{Text: systemPrompt}},
		Messages: []sdk.BetaMessageParam{
			sdk.NewBetaUserMessage(sdk.NewBetaTextBlock(text)),
		},
		Tools:      []sdk.BetaToolUnionParam{{OfTool: &tool}},
		ToolChoice: sdk.BetaToolChoiceParamOfTool(intentToolName),
		// Safety classifiers on this model tier can decline a request; the
		// server-side fallback re-serves it on Opus 4.8 in the same call.
		Betas:     []sdk.AnthropicBeta{sdk.AnthropicBetaServerSideFallback2026_06_01},
		Fallbacks: sdk.BetaFallbacksParamUnion{OfBetaFallbackArray: []sdk.BetaFallbackParam{{Model: "claude-opus-4-8"}}},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}
	if resp.StopReason == sdk.BetaStopReasonRefusal {
		return nil, errors.New("anthropic: request declined by safety classifiers")
	}

	for _, block := range resp.Content {
		variant, ok := block.AsAny().(sdk.BetaToolUseBlock)
		if !ok || variant.Name != intentToolName {
			continue
		}
		var payload intentPayload
		if err := json.Unmarshal([]byte(variant.JSON.Input.Raw()), &payload); err != nil {
			return nil, fmt.Errorf("anthropic: decode intent payload: %w", err)
		}
		return &plan.RouteIntent{
			SchemaVersion: plan.RouteIntentSchemaVersion,
			Summary:       payload.Summary,
			Preferences: plan.RouteIntentPreferences{
				Shade:    payload.Preferences.Shade,
				Greenery: payload.Preferences.Greenery,
				Wind:     payload.Preferences.Wind,
			},
			Constraints: plan.RouteIntentConstraints{
				MaxDetourM:      payload.Constraints.MaxDetourM,
				MaxGradePercent: payload.Constraints.MaxGradePercent,
			},
			UnresolvedTerms: payload.UnresolvedTerms,
			Provenance: plan.RouteIntentProvenance{
				Model:         string(resp.Model),
				PromptVersion: promptVersion,
			},
		}, nil
	}
	return nil, errors.New("anthropic: model returned no intent tool call")
}
