# ADR 0002: Use Valhalla request-scoped costing before forking

- Status: Accepted
- Date: 2026-08-04

## Context

Personalized routes need to favor or discourage arbitrary graph-aligned roads:
riverbanks, greenery, shade, steep grades, and roads the user has explored.
Valhalla bicycle options cover general preferences such as hills and road class,
but not every product-specific signal.

Current Valhalla supports `linear_cost_factors`, whose graph-aligned lines apply
request-scoped cost multipliers. It also supports intermediate locations,
alternative routes, matrices, map matching, and route exclusion inputs. See the
[official OpenAPI schema](https://raw.githubusercontent.com/valhalla/valhalla/master/docs/docs/api/openapi.yaml).

## Decision

Use stock, version-pinned Valhalla and request-scoped costing as the default:

- standard bicycle costing options for general cycling preferences;
- ordered waypoints for bridges and selected venue stops;
- matrices to shortlist stop insertion candidates;
- `linear_cost_factors` below `1` to favor and above `1` to discourage
  graph-aligned edges;
- map matching to align imported or derived lines with the graph;
- application-level validation of returned candidates.

All graph-aligned data must derive from the same pinned OSM snapshot as the
Valhalla tiles. Linear factors are limited to the request corridor, normalized,
clamped, and capped below configured service limits.

Maintain a custom Valhalla build only when a documented invariant cannot be
enforced or proven by request costing plus post-route validation. The first
possible case is strict directional maximum-grade exclusion during graph search.

## Consequences

- Most new preferences do not create a long-lived C++ fork.
- The Valhalla image may not use the mutable `latest` tag in reproducible
  environments.
- OSM snapshot identity becomes part of routing dataset metadata.
- PostGIS-to-Valhalla graph alignment needs explicit tooling and verification.
- Linear costs are soft; they cannot be presented as hard exclusion guarantees.
- A future fork requires a separate ADR describing the invariant, upstreaming
  strategy, compatibility tests, and tile migration cost.

