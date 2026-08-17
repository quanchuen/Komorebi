# Web application instructions

These instructions apply to everything under `web/` except where a deeper
`AGENTS.md` adds stricter rules.

## Stack and boundaries

- Use SvelteKit 2, Svelte 5 runes (`$state`, `$derived`, `$effect`), TypeScript,
  Tailwind CSS 4, and MapLibre GL JS.
- Preserve SSR for discovery and route-detail pages. Browser-only APIs must be
  guarded with `$app/environment` or invoked after mount.
- Vite proxies `/api` to the Go API, `/tiles` to Martin, and `/nominatim` to the
  geocoder. Do not bake development origins into new components.
- Treat the Go API as the source of routing truth. The browser may interpret and
  present routes but must not invent safe route geometry.

## State and persistence

- Put cross-component state in `src/lib/stores/`; do not create duplicate local
  defaults in separate components.
- Low-risk rider preferences use the versioned browser storage adapter in
  `stores/planner.ts`.
- Never persist live geolocation fixes or raw exploration tracks in
  `localStorage`. Larger structured offline data belongs in IndexedDB with an
  explicit retention design.
- Browser storage must tolerate malformed values, disabled storage, quota
  failures, and SSR.

## Maps and navigation

- Keep map rendering in the shared Map component and stores rather than directly
  manipulating MapLibre from unrelated UI components.
- Foreground web navigation must request location only after a user action and
  stop watchers on teardown.
- Do not claim reliable background or locked-screen guidance on iOS. Visibility
  changes must be represented honestly in navigation state.
- Service workers may cache the app shell and explicit offline data, but must not
  cache authenticated API responses indiscriminately.

## Verification

Run focused checks while editing and finish with:

```bash
npm run check
npm run lint
npm run format:check
npm run check:tokens
npm run build
```

Report pre-existing failures separately from regressions. UI and layout tests
must live under `web/tests/` and obey `web/tests/AGENTS.md`.
