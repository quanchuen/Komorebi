# ADR 0004: Generate and validate flex routes

- Status: Accepted
- Date: 2026-08-04

## Context

Users may accept a 1 km, 2 km, or 5 km detour in exchange for flatter terrain.
Valhalla's bicycle `use_hills` option attempts to avoid steep grades but does not
provide a maximum detour or maximum-grade guarantee. “Avoid grades over 12%” and
“reduce climbing by 30%” are distinct requirements.

## Decision

Route Planning uses a generate, validate, rank process:

1. Calculate a baseline route and distance.
2. Generate a bounded set of flatter candidates using `use_hills`, route
   alternatives, and steep-edge linear penalties.
3. Reject candidates whose distance exceeds baseline plus `max_detour_m`.
4. Calculate directional grade and climbing metrics from routing elevation data.
5. Reject candidates that violate a requested hard grade limit when the system
   can validate all edges; otherwise report the hard constraint as unsupported.
6. Rank valid candidates by constraint satisfaction, total climbing, distance,
   duration, and the remaining declared preferences.

The intent schema represents `max_grade_percent`,
`target_climbing_reduction_percent`, and `max_detour_m` separately. The UI must
not label a soft `use_hills` route as guaranteed to avoid a grade.

## Consequences

- Detour budgets are exact application invariants rather than routing hints.
- Multiple Valhalla calls may be required, so requests need budgets, concurrency
  limits, cancellation, and cache keys.
- Route results expose baseline distance, detour distance, total ascent, maximum
  directional grade, and validation coverage.
- A strict grade guarantee may eventually trigger the custom-costing exception
  defined by ADR 0002.
- “No valid route” becomes a normal domain outcome with explicit possible
  relaxations.

