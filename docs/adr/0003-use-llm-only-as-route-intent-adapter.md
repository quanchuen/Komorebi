# ADR 0003: Use an LLM only as a route-intent adapter

- Status: Accepted
- Date: 2026-08-04

## Context

Users should be able to ask for changes such as “stop by a konbini,” “stay by
the river until that bridge,” or “keep the detour under two kilometres.” An LLM
can interpret this language, but it cannot reliably perform graph search,
calculate detours, invent safe coordinates, or prove routing constraints.

## Decision

The LLM is an optional inbound anti-corruption adapter. It converts text into a
schema-validated, versioned `RouteIntent` containing only supported operations,
preferences, constraints, references, and unresolved terms.

It must not:

- produce or edit route geometry;
- invent POI or bridge coordinates;
- select the winning venue or route;
- waive access, safety, grade, or detour constraints;
- call Valhalla directly.

Route Planning resolves references through Place Catalog and map data, executes
deterministic routing tools, and returns structured results. Material ambiguity
or relaxation of a hard constraint requires user confirmation. Every interpreted
intent records schema version, original text, normalized values, unresolved
terms, and model/prompt provenance for debugging; sensitive text follows the
project retention policy.

The non-LLM API remains first-class and capable of expressing every supported
operation.

## Consequences

- Natural-language behavior can be evaluated separately from route correctness.
- Hallucinated coordinates cannot enter routing without deterministic resolution.
- A model outage does not remove structured route planning.
- New phrases require intent fixtures and contract tests, not routing-engine
  changes.
- The product must clearly display interpreted constraints before or alongside
  consequential route changes.

