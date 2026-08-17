package plan

// RouteIntentSchemaVersion identifies the intent schema an interpreted intent
// conforms to. Bump when fields are added or their meaning changes.
const RouteIntentSchemaVersion = "route-intent/v1"

// MaxIntentTextLen bounds the free-text input accepted for interpretation.
const MaxIntentTextLen = 500

// RouteIntentPreferences carries optional 0..1 routing preference weights.
// A nil field means the rider's text did not express that preference.
type RouteIntentPreferences struct {
	Shade    *float64
	Greenery *float64
	Wind     *float64
}

// RouteIntentConstraints carries hard constraints the schema can represent.
// Representation does not imply execution support: constraints the routing
// pipeline cannot enforce must be reported as unsupported, never silently
// dropped or presented as satisfied (ADR 0003, ADR 0004).
type RouteIntentConstraints struct {
	MaxDetourM      *float64
	MaxGradePercent *float64
}

// RouteIntentProvenance records which model and prompt produced the intent,
// for debugging and evaluation of natural-language behavior.
type RouteIntentProvenance struct {
	Model         string
	PromptVersion string
}

// RouteIntent is the schema-validated output of the natural-language intent
// adapter (ADR 0003). It contains only supported operations, preferences,
// constraints, and unresolved terms — never geometry or coordinates.
type RouteIntent struct {
	SchemaVersion   string
	OriginalText    string
	Summary         string
	Preferences     RouteIntentPreferences
	Constraints     RouteIntentConstraints
	UnresolvedTerms []string
	Provenance      RouteIntentProvenance
}

// Normalize clamps every expressed weight into [0,1] and drops constraint
// values that are not positive. It is the deterministic validation gate
// between model output and the routing pipeline.
func (ri *RouteIntent) Normalize() {
	ri.Preferences.Shade = clampUnitWeight(ri.Preferences.Shade)
	ri.Preferences.Greenery = clampUnitWeight(ri.Preferences.Greenery)
	ri.Preferences.Wind = clampUnitWeight(ri.Preferences.Wind)
	ri.Constraints.MaxDetourM = dropNonPositive(ri.Constraints.MaxDetourM)
	ri.Constraints.MaxGradePercent = dropNonPositive(ri.Constraints.MaxGradePercent)
	if ri.SchemaVersion == "" {
		ri.SchemaVersion = RouteIntentSchemaVersion
	}
}

func clampUnitWeight(v *float64) *float64 {
	if v == nil {
		return nil
	}
	w := *v
	if w != w { // NaN
		return nil
	}
	if w < 0 {
		w = 0
	}
	if w > 1 {
		w = 1
	}
	return &w
}

func dropNonPositive(v *float64) *float64 {
	if v == nil {
		return nil
	}
	w := *v
	if w != w || w <= 0 {
		return nil
	}
	return &w
}
