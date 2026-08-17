# ADR 0006: Store the anonymous rider profile in the browser

- Status: Accepted
- Date: 2026-08-04

## Context

Personalized routing needs durable rider preferences, but requiring an account
would add identity, consent, recovery, and synchronization work before the route
policy is proven. The current planner holds shade, greenery, and wind weights in
an in-memory Svelte store, and another navigation surface previously submitted
hard-coded defaults.

Preferences are low-risk configuration. Exploration imports, precise location
history, ride logs, and generated routes are more sensitive and have different
storage and lifecycle requirements.

## Decision

Store the anonymous `RiderProfile` locally in the browser for the first release.
Use a versioned key and a schema-owned adapter rather than accessing
`localStorage` from components. Validate and clamp values when loading, supply
defaults for missing fields, and tolerate malformed or unavailable storage.

The initial persisted profile contains routing preference weights. Future fields
may include default detour allowance, climbing tolerance, preferred surfaces,
units, and navigation cue settings. Schema migrations must preserve valid known
fields and reset invalid values safely.

Do not place the following in `localStorage`:

- authentication tokens that can be kept in secure HTTP-only cookies;
- raw KML, GPX, Fog of World, or continuous location history;
- large offline maps or route datasets;
- secrets or payment data.

Large structured local data, if introduced, uses IndexedDB with explicit quotas,
retention, export, and deletion behavior. A future account service can synchronize
the local profile only after an explicit user action and a separate decision.

## Consequences

- Preferences survive reloads without registration and remain device/browser
  specific.
- Clearing site data, private browsing, or storage eviction can remove them.
- There is no automatic synchronization, backup, or conflict resolution.
- The UI needs reset/export controls as the profile grows.
- Every routing surface must read from the same profile store; hard-coded copies
  are not allowed.

