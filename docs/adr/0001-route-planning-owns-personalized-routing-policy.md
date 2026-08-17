# ADR 0001: Route Planning owns personalized routing policy

- Status: Accepted
- Date: 2026-08-04

## Context

The system currently accepts shade, greenery, and wind preferences, but the
routing application service selects three fixed Valhalla profiles. New requests
include river preference, requested venue stops, detour budgets, grade limits,
and avoiding explored roads. These inputs cross several data-owning contexts.

Putting policy in Valhalla would couple the product domain to an external graph
engine. Putting it in Environment, Community, or an LLM would give a supporting
capability authority over the core user promise.

## Decision

Route Planning is the core bounded context for personalized routing. It owns:

- the versioned `RouteIntent` and `RoutingConstraints` language;
- the distinction between soft preferences and hard constraints;
- baseline calculation and detour budgets;
- candidate generation orchestration;
- deterministic validation, scoring, selection, and explanations.

Environment, Place Catalog, and Exploration provide facts through ports.
Valhalla implements a `RoutingEngine` port and returns candidates; it does not
decide whether a candidate fulfills the product request. Natural-language and
HTTP interfaces are inbound adapters.

The existing package layout may evolve incrementally. Place Catalog can remain
physically under `internal/domain/environment` until its vocabulary or release
cadence warrants extraction. Logical ownership must nevertheless follow this
decision.

## Consequences

- Preference fields can no longer be silently ignored.
- Every hard constraint needs a validator or an explicit unsupported result.
- Candidate score components must be observable and testable.
- Supporting contexts remain reusable and cannot depend on Route Planning
  implementation types.
- Routing orchestration grows in complexity and needs bounded candidate counts,
  timeouts, and partial-failure behavior.

