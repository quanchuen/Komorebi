# ADR 0005: Model explored-road knowledge as an Exploration bounded context

- Status: Accepted
- Date: 2026-08-04

## Context

Explorer mode imports KML, GPX, GeoJSON, or supported Fog of World data and
favors roads a user has not ridden. Raw files, GPS traces, OSM ways, and Valhalla
edges have different identities and lifecycles. Valhalla graph edge IDs can
change when tiles are rebuilt.

Community owns ride logs, but explored-network coverage has its own vocabulary,
matching process, quality rules, privacy concerns, and projections. Environment
describes shared external conditions rather than user-specific history.

## Decision

Create an Exploration supporting bounded context. It owns imports, normalized
traces, map-matching status, durable explored-edge evidence, and coverage
projections.

Import parsers and Valhalla map matching are anti-corruption adapters. Durable
edge identity records routing-dataset identity, source OSM identity when
available, direction, canonical geometry, observation time, and confidence.
Valhalla graph IDs are rebuildable adapter references, never the sole key.

Route Planning queries explored edges only inside its search corridor and sends
them as soft positive cost factors. It permits configurable explored connectors
near route endpoints, validates the result, and reports new-road distance,
explored-road distance, percentage new, and unavoidable reuse. Raw trace
polygons are not hard route exclusions.

Ride logs may publish an event requesting an Exploration import, but Community
does not update Exploration tables directly.

## Consequences

- Explorer mode remains useful even when total avoidance would disconnect the
  network.
- Tile rebuilds require projection rematching, not loss of exploration history.
- Per-user spatial data requires authorization, retention, deletion, export, and
  deduplication policies.
- Fog-style raster imports require a format-specific adapter that intersects
  coverage with the road network; unsupported exports fail with diagnostics.
- Large histories are spatially clipped and simplified before routing requests.

