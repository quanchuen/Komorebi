<!-- web/src/lib/components/NavigationPanel.svelte -->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { SvelteMap } from 'svelte/reactivity';
  import { replaceState } from '$app/navigation';
  import { resolve } from '$app/paths';
  import { browser } from '$app/environment';
  import { routing, discovery, routes as routesApi } from '$lib/api/client';
  import { buildLineGradient } from '$lib/utils/conditionColors';
  import {
    departureAt,
    highlightedRouteId,
    bboxString,
    mapInstance,
    routeDisplays,
    selectedRouteGeometry,
    selectedRouteDistanceM,
    activeOverlay,
    liveNavigationPosition
  } from '$lib/stores/map';
  import { discoveryRoutes, discoveryLoading, discoveryError } from '$lib/stores/discovery';
  import { plannerPreferences } from '$lib/stores/planner';
  import { navCardCollapsed, resultsPanelCollapsed } from '$lib/stores/ui';
  import {
    foregroundNavigation,
    setNavigationRoute,
    startForegroundNavigation,
    stopForegroundNavigation
  } from '$lib/stores/navigation';
  import type { Route, RouteConditionSegment, RouteIntentResponse } from '$lib/api/types';
  import RouteCard from './RouteCard.svelte';
  import ConditionSparkline from './ConditionSparkline.svelte';
  import ElevationSparkline from './ElevationSparkline.svelte';
  import MapLayerControl from './MapLayerControl.svelte';
  import AsyncBoundary from './ui/AsyncBoundary.svelte';
  import Chevron from './ui/Chevron.svelte';

  interface Stop {
    id: string;
    label: string;
    query: string; // typed search text
    lat: number | null;
    lon: number | null;
  }

  let stops = $state<Stop[]>([
    { id: crypto.randomUUID(), label: '', query: '', lat: null, lon: null },
    { id: crypto.randomUUID(), label: '', query: '', lat: null, lon: null }
  ]);

  let conditionsCache = $state(new Map<string, RouteConditionSegment[]>());
  let routeDetailsCache = $state(new Map<string, Route>());

  // Address lookup
  let activeInputIndex = $state<number | null>(null);
  let suggestions = $state<{ display_name: string; lat: string; lon: string }[]>([]);
  let highlightedSuggIdx = $state(-1);
  let searchDebounce: ReturnType<typeof setTimeout>;
  let inputRefs: HTMLInputElement[] = [];

  function focusInput(index: number) {
    activeInputIndex = index;
    suggestions = [];
    highlightedSuggIdx = -1;
  }

  function handleKeydown(index: number, e: KeyboardEvent) {
    if (suggestions.length === 0) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      highlightedSuggIdx = Math.min(highlightedSuggIdx + 1, suggestions.length - 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      highlightedSuggIdx = Math.max(highlightedSuggIdx - 1, 0);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (highlightedSuggIdx >= 0 && highlightedSuggIdx < suggestions.length) {
        selectSuggestion(index, suggestions[highlightedSuggIdx]);
      }
    } else if (e.key === 'Escape') {
      suggestions = [];
      highlightedSuggIdx = -1;
    }
  }

  async function searchAddress(query: string) {
    if (query.length < 3) {
      suggestions = [];
      highlightedSuggIdx = -1;
      return;
    }
    try {
      const res = await fetch(
        `/nominatim/search?format=json&q=${encodeURIComponent(query)}&limit=5&countrycodes=jp&accept-language=en`
      );
      if (res.ok) {
        suggestions = await res.json();
        highlightedSuggIdx = -1;
      }
    } catch {
      suggestions = [];
      highlightedSuggIdx = -1;
    }
  }

  function handleInput(index: number, e: Event) {
    const val = (e.target as HTMLInputElement).value;
    // Only update query (what's typed). Clear coordinates since user is changing the location.
    // Don't overwrite label — it gets set properly by selectSuggestion or handleMapClick.
    stops = stops.map((s, i) => (i === index ? { ...s, query: val, lat: null, lon: null } : s));
    clearTimeout(searchDebounce);
    searchDebounce = setTimeout(() => searchAddress(val), 300);
  }

  function selectSuggestion(
    index: number,
    suggestion: { display_name: string; lat: string; lon: string }
  ) {
    const lat = parseFloat(suggestion.lat);
    const lon = parseFloat(suggestion.lon);
    const shortName = suggestion.display_name.split(',').slice(0, 2).join(',').trim();
    stops = stops.map((s, i) =>
      i === index ? { ...s, lat, lon, label: shortName, query: shortName } : s
    );
    suggestions = [];
    activeInputIndex = null;

    // Auto-focus next empty input
    const nextEmpty = stops.findIndex((s, i) => i > index && s.lat === null);
    if (nextEmpty !== -1) {
      setTimeout(() => {
        inputRefs[nextEmpty]?.focus();
        activeInputIndex = nextEmpty;
      }, 100);
    }

    // Fly map to selected location
    const mapInst = $mapInstance;
    if (mapInst) {
      mapInst.flyTo({ center: [lon, lat], zoom: 14, duration: 800 });
    }
  }

  function addStopAfter(index: number) {
    const insertAt = index + 1;
    const newStop: Stop = { id: crypto.randomUUID(), label: '', query: '', lat: null, lon: null };
    stops = [...stops.slice(0, insertAt), newStop, ...stops.slice(insertAt)];
    setTimeout(() => {
      inputRefs[insertAt]?.focus();
      activeInputIndex = insertAt;
    }, 100);
  }

  function removeStop(index: number) {
    if (stops.length <= 2) return;
    stops = stops.filter((_, i) => i !== index);
  }

  // Called by parent when map is clicked
  export function handleMapClick(lat: number, lon: number) {
    if (activeInputIndex !== null) {
      const idx = activeInputIndex;
      const coordLabel = `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
      stops = stops.map((s, i) => {
        if (i !== idx) return s;
        // Keep existing address name if user already searched. Only use coords if empty.
        const displayName = s.query.trim() && s.lat === null ? s.query : coordLabel;
        return { ...s, lat, lon, label: displayName, query: displayName };
      });
      suggestions = [];

      // Auto-focus next empty
      const nextEmpty = stops.findIndex((s, i) => i > idx && s.lat === null);
      if (nextEmpty !== -1) {
        setTimeout(() => {
          inputRefs[nextEmpty]?.focus();
          activeInputIndex = nextEmpty;
        }, 100);
      } else {
        activeInputIndex = null;
      }
    }
  }

  // --- Shareable path URL (?from=lat,lon,label&via=…&to=…) ---
  // Only the path (stops) is encoded, not the computed route geometry, so a
  // shared link re-routes with the recipient's own preferences.

  const MAX_URL_VIAS = 8;

  function stopToParam(s: Stop): string {
    const base = `${s.lat!.toFixed(5)},${s.lon!.toFixed(5)}`;
    const label = s.label.trim();
    // Coordinate-style labels (from map clicks) carry no extra information.
    return label && !/^-?\d+\.\d+,\s*-?\d+\.\d+$/.test(label) ? `${base},${label}` : base;
  }

  function parseStopParam(value: string): Stop | null {
    const parts = value.split(',');
    if (parts.length < 2) return null;
    const lat = Number(parts[0]);
    const lon = Number(parts[1]);
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) return null;
    if (lat < -90 || lat > 90 || lon < -180 || lon > 180) return null;
    const label = parts.slice(2).join(',').trim() || `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
    return { id: crypto.randomUUID(), lat, lon, label, query: label };
  }

  function syncStopsToUrl() {
    const url = new URL(window.location.href);
    url.searchParams.delete('from');
    url.searchParams.delete('via');
    url.searchParams.delete('to');

    const first = stops[0];
    const last = stops[stops.length - 1];
    if (first?.lat !== null && last?.lat !== null && stops.length >= 2) {
      url.searchParams.set('from', stopToParam(first));
      for (const via of stops.slice(1, -1)) {
        if (via.lat !== null) url.searchParams.append('via', stopToParam(via));
      }
      url.searchParams.set('to', stopToParam(last));
    }
    if (url.href !== window.location.href) {
      const query = url.searchParams.toString();
      if (query) {
        // The rule only recognizes a bare resolve() argument; it cannot
        // express a resolved path plus query string, which is what a
        // shareable stop URL needs. The path itself still comes from resolve.
        // eslint-disable-next-line svelte/no-navigation-without-resolve
        replaceState(resolve('/') + '?' + query, {});
      } else {
        replaceState(resolve('/'), {});
      }
    }
  }

  let urlSyncTimeout: ReturnType<typeof setTimeout>;
  $effect(() => {
    // Register a dependency on every stop's coordinates and label.
    stops.forEach((s) => [s.lat, s.lon, s.label]);
    if (!browser) return;
    clearTimeout(urlSyncTimeout);
    urlSyncTimeout = setTimeout(syncStopsToUrl, 300);
  });

  function restoreStopsFromUrl(): boolean {
    const params = new URLSearchParams(window.location.search);
    const from = params.get('from');
    const to = params.get('to');
    if (!from || !to) return false;
    const fromStop = parseStopParam(from);
    const toStop = parseStopParam(to);
    if (!fromStop || !toStop) return false;
    const vias = params
      .getAll('via')
      .slice(0, MAX_URL_VIAS)
      .map(parseStopParam)
      .filter((s): s is Stop => s !== null);
    stops = [fromStop, ...vias, toStop];
    return true;
  }

  onMount(() => {
    if (restoreStopsFromUrl()) doRoute();
  });

  let canRoute = $derived(stops.filter((s) => s.lat !== null).length >= 2);
  let hasAllStops = $derived(stops.every((s) => s.lat !== null));
  // With only start + end the wide layout is a single horizontal row; via
  // stops switch the card back to the vertical list so each gets a full row.
  let hasVias = $derived(stops.length > 2);

  // Routing state
  import type { RouteAlternative } from '$lib/api/types';

  let isRouting = $state(false);
  let alternatives = $state<RouteAlternative[]>([]);
  let selectedProfile = $state<string | null>(null);
  let routeError = $state<string | null>(null);

  let selectedAlt = $derived(alternatives.find((a) => a.profile === selectedProfile) ?? null);

  const profileIcons: Record<string, string> = {
    suggested: '⚖',
    fast: '⚡',
    avoid_main_roads: '🛡'
  };

  const profileColors: Record<string, string> = {
    suggested: '#38bdf8', // sky-400
    fast: '#f59e0b', // amber-500
    avoid_main_roads: '#34d399' // emerald-400
  };

  function updateRouteDisplays() {
    routeDisplays.set(
      alternatives.map((alt) => {
        const coords: [number, number][] = (alt.geometry?.coordinates ?? []).map(
          (c: number[]) => [c[0], c[1]] as [number, number]
        );
        return {
          coords,
          selected: alt.profile === selectedProfile,
          profile: alt.profile,
          color: profileColors[alt.profile] ?? '#64748b',
          distanceM: alt.total_distance_km * 1000
        };
      })
    );
  }

  async function doRoute() {
    const validStops = stops.filter((s) => s.lat !== null && s.lon !== null);
    if (validStops.length < 2) return;

    isRouting = true;
    routeError = null;
    alternatives = [];
    selectedProfile = null;
    conditionsRequestSeq += 1;

    try {
      const res = await routing.directions({
        stops: validStops.map((s) => ({ type: 'manual' as const, lat: s.lat!, lon: s.lon! })),
        departure_at: $departureAt,
        speed_model: 'elevation',
        preferences: $plannerPreferences
      });

      alternatives = res.alternatives ?? [];
      altConditions.clear();
      if (alternatives.length > 0) {
        selectAlternative(alternatives[0].profile);
        void fetchAlternativeConditions(alternatives);
      }
      updateRouteDisplays();
    } catch (e) {
      // Release the map state a previous run may have set, or the curated
      // routes stay dimmed behind alternatives that no longer exist.
      routeDisplays.set([]);
      selectedRouteGeometry.set(null);
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('Failed to fetch')) {
        routeError = 'Cannot connect to API';
      } else if (msg.includes('502')) {
        routeError = 'Routing engine not running (Valhalla)';
      } else {
        routeError = msg;
      }
    } finally {
      isRouting = false;
    }
  }

  // Natural-language route intent (ADR 0003). The LLM only interprets text;
  // interpreted constraints are shown and applied only on explicit confirm.
  let intentText = $state('');
  let intentLoading = $state(false);
  let intentResult = $state<RouteIntentResponse | null>(null);
  let intentError = $state<string | null>(null);
  let intentApplied = $state(false);

  const unsupportedLabels: Record<string, string> = {
    max_detour_m: 'detour budget',
    max_grade_percent: 'max grade limit'
  };

  async function interpretIntent(e: Event) {
    e.preventDefault();
    const text = intentText.trim();
    if (!text || intentLoading) return;
    intentLoading = true;
    intentError = null;
    intentResult = null;
    intentApplied = false;
    try {
      intentResult = await routing.interpretIntent(text, $plannerPreferences);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('503')) {
        intentError = 'Natural-language routing is not configured on this server';
      } else if (msg.includes('Failed to fetch')) {
        intentError = 'Cannot connect to API';
      } else {
        intentError = 'Could not interpret that request';
      }
    } finally {
      intentLoading = false;
    }
  }

  function applyIntent() {
    if (!intentResult) return;
    plannerPreferences.set(intentResult.preferences);
    intentApplied = true;
    if (canRoute) doRoute();
  }

  // Real environment conditions per alternative, computed by the backend from
  // the generated geometry (same data the curated route cards show).
  const altConditions = new SvelteMap<string, RouteConditionSegment[]>();
  // Bumped whenever altConditions is reset for a new routing run so a slow
  // response from an earlier run can never overwrite the current route's data.
  let conditionsRequestSeq = 0;
  let selectedConditions = $derived(
    (selectedProfile ? altConditions.get(selectedProfile) : undefined) ?? []
  );

  async function fetchAlternativeConditions(alts: RouteAlternative[]) {
    const departure = $departureAt;
    const seq = conditionsRequestSeq;
    await Promise.allSettled(
      alts.map(async (alt) => {
        try {
          const res = await routing.conditions(alt.geometry, alt.elevation_profile, departure);
          if (seq !== conditionsRequestSeq) return; // stale: a newer route replaced this one
          altConditions.set(alt.profile, res.segments ?? []);
        } catch {
          /* conditions are enrichment; the alternative stays usable without them */
        }
      })
    );
  }

  function conditionsSummary(segs: RouteConditionSegment[]) {
    if (segs.length === 0) return null;
    return {
      avgShade: segs.reduce((s, c) => s + c.shade, 0) / segs.length,
      avgWind: segs.reduce((s, c) => s + c.wind_benefit, 0) / segs.length,
      maxPrecip: Math.max(...segs.map((c) => c.precip)),
      signals: segs.reduce((s, c) => s + c.signals, 0)
    };
  }

  function windLabel(v: number): string {
    if (v > 0.3) return 'Tailwind';
    if (v < -0.3) return 'Headwind';
    return 'Crosswind';
  }

  function precipLabel(v: number): string {
    if (v <= 0) return 'Dry';
    if (v < 0.3) return 'Light rain';
    if (v < 0.6) return 'Moderate rain';
    return 'Heavy rain';
  }

  function selectAlternative(profile: string) {
    selectedProfile = profile;
    updateRouteDisplays();

    const alt = alternatives.find((a) => a.profile === profile);
    if (!alt) return;

    const coords: [number, number][] = (alt.geometry?.coordinates ?? []).map(
      (c: number[]) => [c[0], c[1]] as [number, number]
    );

    // Set stores for reactive map update
    highlightedRouteId.set(null);
    selectedRouteGeometry.set(coords);
    selectedRouteDistanceM.set(alt.total_distance_km * 1000);
    setNavigationRoute(coords);

    const mapInst = $mapInstance;
    if (mapInst && coords.length > 0) {
      // Geometry is drawn by the Map component from selectedRouteGeometry;
      // only paint (profile color) is set here — overlay replaces via $effect.
      mapInst.setPaintProperty('highlight-route-line', 'line-gradient', null);
      mapInst.setPaintProperty(
        'highlight-route-line',
        'line-color',
        profileColors[profile] ?? '#38BDF8'
      );

      const lons = coords.map((c) => c[0]);
      const lats = coords.map((c) => c[1]);
      mapInst.fitBounds(
        [
          [Math.min(...lons), Math.min(...lats)],
          [Math.max(...lons), Math.max(...lats)]
        ],
        { padding: 80, duration: 800 }
      );
    }
  }

  // Apply condition gradient when overlay is toggled
  $effect(() => {
    const overlay = $activeOverlay;
    const mapInst = $mapInstance;
    const geom = $selectedRouteGeometry;
    const distM = $selectedRouteDistanceM;

    if (!mapInst || !geom || geom.length === 0) return;
    if (!overlay || selectedConditions.length === 0) {
      // No overlay active — use profile color
      mapInst.setPaintProperty('highlight-route-line', 'line-gradient', null);
      mapInst.setPaintProperty(
        'highlight-route-line',
        'line-color',
        profileColors[selectedProfile ?? 'suggested'] ?? '#38BDF8'
      );
      return;
    }

    const gradient = buildLineGradient(selectedConditions, overlay, distM);
    mapInst.setPaintProperty('highlight-route-line', 'line-gradient', gradient);
  });

  $effect(() => {
    liveNavigationPosition.set($foregroundNavigation.position);
  });

  onDestroy(() => {
    clearTimeout(loadDebounce);
    routeLoadSequence += 1;
    stopForegroundNavigation();
    liveNavigationPosition.set(null);
  });

  // Close suggestions when clicking outside
  function handleBlur() {
    setTimeout(() => {
      suggestions = [];
    }, 200);
  }

  // Load routes in viewport
  let routeLoadSequence = 0;

  async function loadRoutes(bbox: string | null, departure: string) {
    if (!bbox) return;
    const sequence = ++routeLoadSequence;
    // Keep the current cards visible while refreshing an already populated
    // viewport. Conditions are enrichment and must not block route rendering.
    if ($discoveryRoutes.length === 0) discoveryLoading.set(true);
    discoveryError.set(null);
    try {
      const res = await discovery.viewport({ bbox });
      if (sequence !== routeLoadSequence) return;
      discoveryRoutes.set(res.routes);
      // A highlight pointing at a route that left the viewport has no card
      // left to clear it — release it so the map doesn't stay dimmed forever.
      if ($highlightedRouteId && !res.routes.some((r) => r.id === $highlightedRouteId)) {
        highlightedRouteId.set(null);
      }
      // Route geometry carries the elevation samples used by every card.
      void Promise.allSettled(
        res.routes.map(async (r) => {
          if (!routeDetailsCache.has(r.id)) {
            try {
              const detail = await routesApi.get(r.id);
              routeDetailsCache = new Map(routeDetailsCache).set(r.id, detail);
            } catch {
              /* keep discovery summary */
            }
          }
        })
      );
      // Prefetch weather for the first 3 routes only (avoid flooding API).
      void Promise.allSettled(
        res.routes.slice(0, 3).map(async (r) => {
          if (!conditionsCache.has(r.id)) {
            try {
              const c = await routesApi.conditions(r.id, departure);
              conditionsCache = new Map(conditionsCache).set(r.id, c.segments ?? []);
            } catch {
              /* skip */
            }
          }
        })
      );
    } catch (e) {
      if (sequence !== routeLoadSequence) return;
      const msg = e instanceof Error ? e.message : String(e);
      discoveryError.set(
        msg.includes('Failed to fetch')
          ? 'Cannot connect to API server (localhost:8080)'
          : `Error: ${msg}`
      );
    } finally {
      if (sequence === routeLoadSequence) discoveryLoading.set(false);
    }
  }

  let loadDebounce: ReturnType<typeof setTimeout>;
  $effect(() => {
    const bbox = $bboxString;
    const dep = $departureAt;
    clearTimeout(loadDebounce);
    loadDebounce = setTimeout(() => loadRoutes(bbox, dep), 1000);
  });

  function retryLoad() {
    discoveryError.set(null);
    loadRoutes($bboxString, $departureAt);
  }

  let filteredRoutes = $derived($discoveryRoutes);

  // One-line summary shown when the address card is folded.
  let collapsedSummary = $derived.by(() => {
    const first = stops[0];
    const last = stops[stops.length - 1];
    const from = first?.label || first?.query.trim();
    const to = last?.label || last?.query.trim();
    if (!from && !to) return 'Where to?';
    const vias = stops.length - 2;
    const core = `${from || 'Start'} → ${to || 'Destination'}`;
    return vias > 0 ? `${core} · ${vias} via` : core;
  });

  // Fresh alternatives are the one thing a rider always wants to see: unfold
  // the results panel when a route search lands, even if it was folded away.
  $effect(() => {
    if (alternatives.length > 0) resultsPanelCollapsed.set(false);
  });
</script>

<!-- Floating panel. On wide screens the address card detaches from the left
     column and centers at the top; the results list stays on the left edge so
     it never covers the map center where routes render. -->
<div
  class="absolute top-4 left-4 bottom-4 z-10 w-80
            flex flex-col gap-3 pointer-events-none
            xl:right-4 xl:w-auto"
>
  <!-- Navigation card. Wide screens: a horizontal Start → End bar centered at
       the top; it only expands into the vertical stop list when via stops
       exist. Narrow screens: always the vertical list. -->
  <div
    class="relative z-20 bg-surface/90 backdrop-blur-lg border border-border/50
              rounded-2xl shadow-2xl pointer-events-auto
              xl:absolute xl:top-0 xl:left-1/2 xl:-translate-x-1/2
              {$navCardCollapsed
      ? 'px-3 py-1 xl:w-auto xl:min-w-72 xl:max-w-2xl'
      : hasVias
        ? 'p-4 xl:w-96'
        : 'p-4 xl:w-2xl'}"
  >
    {#if $navCardCollapsed}
      <!-- Folded: a single bar with the trip summary. The Stop control stays
           reachable here so live guidance can always be ended. -->
      <div class="flex items-center gap-2">
        <button
          onclick={() => navCardCollapsed.set(false)}
          aria-expanded="false"
          aria-controls="nav-card-body"
          class="flex-1 min-w-0 min-h-11 flex items-center gap-2 text-left
                 text-text-muted hover:text-text transition-colors"
        >
          <Chevron direction="down" class="text-text-subtle" />
          <span class="text-xs truncate">{collapsedSummary}</span>
        </button>
        {#if $foregroundNavigation.status !== 'idle'}
          <span class="text-3xs text-text-subtle shrink-0 hidden sm:inline">
            {$foregroundNavigation.status === 'requesting'
              ? 'Waiting for GPS…'
              : $foregroundNavigation.offRoute
                ? 'Off route'
                : $foregroundNavigation.remainingDistanceM !== null
                  ? `${($foregroundNavigation.remainingDistanceM / 1000).toFixed(1)} km left`
                  : 'Guidance active'}
          </span>
          <button
            onclick={stopForegroundNavigation}
            class="shrink-0 px-2.5 py-1.5 rounded-lg text-3xs text-text-muted
                   border border-border hover:text-text hover:bg-surface-raised">Stop</button
          >
        {/if}
        <MapLayerControl />
      </div>
    {:else}
      <div id="nav-card-body">
        <!-- Stop inputs with icon rail -->
        <div class="flex flex-col gap-0 {hasVias ? '' : 'xl:flex-row xl:items-center xl:gap-2'}">
          {#each stops as stop, i (stop.id)}
            <!-- Stop row -->
            <div class="flex items-center gap-2 {hasVias ? '' : 'xl:flex-1 xl:min-w-0'}">
              <!-- Icon -->
              <div class="w-5 shrink-0 flex items-center justify-center text-sm">
                {#if i === 0}
                  <span title="Start">🏁</span>
                {:else if i === stops.length - 1}
                  <span title="End">🚩</span>
                {:else}
                  <div class="w-3 h-3 rounded-full bg-amber-400 border-2 border-amber-300"></div>
                {/if}
              </div>

              <!-- Input -->
              <div class="flex-1 relative">
                <div class="flex items-center gap-1">
                  <input
                    bind:this={inputRefs[i]}
                    type="text"
                    placeholder={i === 0
                      ? 'Start location'
                      : i === stops.length - 1
                        ? 'Destination'
                        : 'Via stop'}
                    value={stop.query}
                    onfocus={() => focusInput(i)}
                    onblur={handleBlur}
                    oninput={(e) => handleInput(i, e)}
                    onkeydown={(e) => handleKeydown(i, e)}
                    class="w-full bg-surface-raised/80 border text-text text-xs rounded-lg
                       px-3 py-2 transition-colors
                       {activeInputIndex === i
                      ? 'border-accent ring-1 ring-accent/30'
                      : 'border-border hover:border-border-strong'}
                       focus:outline-none placeholder:text-text-subtle"
                  />
                  {#if i > 0 && i < stops.length - 1}
                    <button
                      onclick={() => removeStop(i)}
                      class="text-text-subtle hover:text-danger text-sm w-5 h-5
                         flex items-center justify-center shrink-0"
                      aria-label="Remove stop">&times;</button
                    >
                  {/if}
                </div>

                <!-- Address suggestions dropdown -->
                {#if activeInputIndex === i && suggestions.length > 0}
                  <div
                    class="absolute top-full left-0 right-0 mt-1 z-50
                          bg-surface-raised border border-border rounded-lg shadow-xl
                          overflow-hidden"
                  >
                    {#each suggestions as s, si}
                      <button
                        onmousedown={() => selectSuggestion(i, s)}
                        onmouseenter={() => (highlightedSuggIdx = si)}
                        class="w-full text-left px-3 py-2 text-xs transition-colors border-b border-border/50
                           last:border-b-0
                           {si === highlightedSuggIdx
                          ? 'bg-accent/30 text-text'
                          : 'text-text-muted hover:bg-surface-overlay'}"
                      >
                        {s.display_name.split(',').slice(0, 3).join(',')}
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>

            <!-- Connector + add-stop button between each pair -->
            {#if i < stops.length - 1}
              {#if !hasVias}
                <!-- Compact horizontal connector for the wide Start → End bar -->
                <div class="hidden xl:flex items-center gap-1 shrink-0">
                  <div class="w-3 border-t border-dashed border-border"></div>
                  <button
                    onclick={() => addStopAfter(i)}
                    class="text-3xs text-text-subtle hover:text-amber-400
                       bg-surface-raised hover:bg-surface-overlay border border-border
                       hover:border-amber-500/50
                       rounded-full w-5 h-5 flex items-center justify-center
                       transition-colors"
                    aria-label="Add stop">+</button
                  >
                  <div class="w-3 border-t border-dashed border-border"></div>
                </div>
              {/if}
              <div class="flex items-center gap-2 my-2 {hasVias ? '' : 'xl:hidden'}">
                <!-- Vertical dash line under icon column -->
                <div class="w-5 shrink-0 flex justify-center">
                  <div class="w-px h-4 border-l border-dashed border-border-strong"></div>
                </div>
                <!-- Dashed line + plus button -->
                <div class="flex-1 flex items-center gap-2">
                  <div class="flex-1 border-t border-dashed border-border"></div>
                  <button
                    onclick={() => addStopAfter(i)}
                    class="text-3xs text-text-subtle hover:text-amber-400
                       bg-surface-raised hover:bg-surface-overlay border border-border
                       hover:border-amber-500/50
                       rounded-full w-5 h-5 flex items-center justify-center
                       transition-colors"
                    aria-label="Add stop">+</button
                  >
                  <div class="flex-1 border-t border-dashed border-border"></div>
                </div>
              </div>
            {/if}
          {/each}
        </div>

        <!-- Route button -->
        {#if canRoute}
          <button
            onclick={doRoute}
            disabled={isRouting}
            class="w-full mt-3 py-2 rounded-lg text-xs font-semibold transition-colors
               {isRouting
              ? 'bg-accent-strong text-accent cursor-wait'
              : 'bg-accent hover:bg-accent-strong text-white'}"
          >
            {isRouting ? 'Finding routes...' : 'Route'}
          </button>
        {/if}

        <!-- Route error -->
        {#if routeError}
          <div class="mt-2 text-xs text-danger bg-danger-surface/50 rounded-lg px-3 py-2">
            {routeError}
          </div>
        {/if}

        <!-- Natural-language routing -->
        <div class="mt-3 pt-3 border-t border-border/50">
          <form class="flex items-center gap-2" onsubmit={interpretIntent}>
            <input
              type="text"
              bind:value={intentText}
              maxlength="500"
              placeholder="Describe your ride — e.g. max shade, out of the wind"
              aria-label="Describe your ride"
              class="flex-1 min-w-0 bg-surface-raised/80 border border-border text-text text-xs
                 rounded-lg px-3 py-2 transition-colors hover:border-border-strong
                 focus:outline-none focus:border-accent placeholder:text-text-subtle"
            />
            <button
              type="submit"
              disabled={intentLoading || !intentText.trim()}
              aria-label="Interpret ride description"
              class="shrink-0 px-2.5 py-2 rounded-lg text-xs transition-colors border
                 {intentLoading
                ? 'bg-surface-raised text-text-subtle border-border cursor-wait'
                : 'bg-surface-raised hover:bg-surface-overlay text-text-muted hover:text-text border-border hover:border-border-strong'}"
            >
              {intentLoading ? '…' : '✨'}
            </button>
          </form>

          {#if intentError}
            <div class="mt-2 text-3xs text-danger bg-danger-surface/50 rounded-lg px-3 py-1.5">
              {intentError}
            </div>
          {/if}

          {#if intentResult}
            <div
              class="mt-2 bg-surface-raised/60 border border-border/50 rounded-lg px-3 py-2 space-y-1.5"
            >
              <div class="text-2xs text-text">{intentResult.intent.summary}</div>

              {#if intentResult.applied.length > 0}
                <div class="flex flex-wrap gap-1">
                  {#each intentResult.applied as key (key)}
                    <span
                      class="text-3xs px-1.5 py-0.5 rounded-full bg-accent/15 text-accent border border-accent/30"
                    >
                      {key}
                      {intentResult.preferences[key as 'shade' | 'greenery' | 'wind'].toFixed(1)}
                    </span>
                  {/each}
                </div>
              {/if}

              {#if intentResult.unsupported.length > 0}
                <div class="text-3xs text-amber-300">
                  Not supported yet: {intentResult.unsupported
                    .map((k) => unsupportedLabels[k] ?? k)
                    .join(', ')}
                </div>
              {/if}

              {#if intentResult.intent.unresolved_terms.length > 0}
                <div class="text-3xs text-text-subtle">
                  Couldn't interpret: {intentResult.intent.unresolved_terms.join(' · ')}
                </div>
              {/if}

              {#if intentResult.applied.length > 0}
                <button
                  onclick={applyIntent}
                  disabled={intentApplied}
                  class="w-full mt-1 py-1.5 rounded-lg text-3xs font-semibold transition-colors
                     {intentApplied
                    ? 'bg-surface-overlay text-text-subtle cursor-default'
                    : 'bg-accent hover:bg-accent-strong text-white'}"
                >
                  {intentApplied
                    ? 'Applied to preferences ✓'
                    : canRoute
                      ? 'Apply & route'
                      : 'Apply to preferences'}
                </button>
              {:else}
                <div class="text-3xs text-text-subtle">No routing preferences to apply.</div>
              {/if}
            </div>
          {/if}
        </div>

        <!-- Keep the status/Stop controls visible while navigation is running even
         if a re-route cleared the alternatives, so the GPS watch and wake lock
         can always be stopped from the UI. -->
        {#if selectedAlt || $foregroundNavigation.status !== 'idle'}
          <div class="mt-3 pt-3 border-t border-border/50">
            {#if $foregroundNavigation.status === 'idle'}
              <button
                onclick={startForegroundNavigation}
                class="w-full py-2 rounded-lg text-xs font-semibold bg-emerald-600
                   hover:bg-emerald-500 text-white transition-colors"
              >
                Start foreground navigation
              </button>
            {:else}
              <div class="flex items-center justify-between gap-3">
                <div class="min-w-0">
                  <div class="text-xs font-medium text-text">
                    {$foregroundNavigation.status === 'requesting'
                      ? 'Waiting for GPS…'
                      : $foregroundNavigation.status === 'paused'
                        ? 'Guidance paused in background'
                        : $foregroundNavigation.status === 'error'
                          ? 'Navigation unavailable'
                          : $foregroundNavigation.offRoute
                            ? 'Off route'
                            : 'Foreground guidance active'}
                  </div>
                  <div class="text-3xs text-text-subtle mt-0.5">
                    {#if $foregroundNavigation.error}
                      {$foregroundNavigation.error}
                    {:else if $foregroundNavigation.remainingDistanceM !== null}
                      {($foregroundNavigation.remainingDistanceM / 1000).toFixed(1)} km remaining · GPS
                      ±{Math.round($foregroundNavigation.position?.accuracy ?? 0)} m
                    {:else}
                      Keep Komorebi visible for continuous guidance
                    {/if}
                  </div>
                </div>
                <button
                  onclick={stopForegroundNavigation}
                  class="shrink-0 px-2.5 py-1.5 rounded-lg text-3xs text-text-muted
                     border border-border hover:text-text hover:bg-surface-raised">Stop</button
                >
              </div>
              {#if $foregroundNavigation.offRoute}
                <div class="mt-2 text-3xs text-amber-300 bg-amber-950/50 rounded-lg px-2 py-1.5">
                  About {Math.round($foregroundNavigation.distanceFromRouteM ?? 0)} m from this route.
                  Recalculate when it is safe to stop.
                </div>
              {/if}
            {/if}
          </div>
        {/if}

        <!-- Fold handle: a wide grabber spanning the card's last row (the top
             bar folds upward), with the layer control kept at its right. -->
        <div class="mt-3 pt-2 border-t border-border/50 flex items-center gap-2">
          <button
            onclick={() => navCardCollapsed.set(true)}
            aria-expanded="true"
            aria-controls="nav-card-body"
            aria-label="Hide trip planner"
            class="flex-1 h-9 flex items-center justify-center gap-3 rounded-lg
                   text-text-subtle hover:text-text-muted hover:bg-surface-raised/60 transition-colors"
          >
            <span class="w-12 h-1 rounded-full bg-border-strong" aria-hidden="true"></span>
            <Chevron direction="up" />
            <span class="w-12 h-1 rounded-full bg-border-strong" aria-hidden="true"></span>
          </button>
          <MapLayerControl />
        </div>
      </div>
    {/if}
  </div>

  <!-- Results panel: route alternatives OR suggested routes. It is a vertical
       sidebar, so it folds sideways: expanded, a full-height handle runs down
       its right edge; folded, only a vertical tab remains on the map's left
       edge. Sized by content, scrolling internally once it overflows. -->
  {#if $resultsPanelCollapsed}
    <button
      onclick={() => resultsPanelCollapsed.set(false)}
      aria-expanded="false"
      aria-controls="results-panel-body"
      class="self-start min-w-11 py-3 px-2 pointer-events-auto flex flex-col items-center gap-3
             bg-surface/80 backdrop-blur-lg border border-border/50 rounded-2xl shadow-2xl
             text-3xs text-text-subtle uppercase tracking-wider
             hover:text-text-muted hover:bg-surface/95 transition-colors
             xl:absolute xl:top-0 xl:left-0"
    >
      <Chevron direction="right" />
      <span class="vertical-label">
        {alternatives.length > 0 ? 'Routes found' : 'Suggested routes'}
        ({alternatives.length > 0 ? alternatives.length : filteredRoutes.length})
      </span>
      <span class="w-1 h-10 rounded-full bg-border-strong" aria-hidden="true"></span>
    </button>
  {:else}
    <div
      class="min-h-0 flex pointer-events-auto overflow-hidden
                bg-surface/80 backdrop-blur-lg border border-border/50
                rounded-2xl shadow-2xl
                xl:absolute xl:top-0 xl:left-0 xl:w-80 xl:max-h-full"
    >
      <div class="flex-1 min-w-0 min-h-0 flex flex-col">
        <div class="px-4 pt-3 pb-1.5 text-3xs text-text-subtle uppercase tracking-wider">
          {alternatives.length > 0 ? 'Routes found' : 'Suggested routes'}
          <span class="normal-case tracking-normal">
            ({alternatives.length > 0 ? alternatives.length : filteredRoutes.length})
          </span>
        </div>

        <div id="results-panel-body" class="flex-1 min-h-0 overflow-y-auto px-3 pb-3 space-y-2">
          {#if alternatives.length > 0}
            <!-- Route alternatives -->
            <div class="flex flex-col gap-1.5">
              {#each alternatives as alt (alt.profile)}
                {@const segs = altConditions.get(alt.profile) ?? []}
                {@const summary = conditionsSummary(segs)}
                <button
                  onclick={() => selectAlternative(alt.profile)}
                  class="w-full px-3 py-2.5 rounded-lg text-left transition-colors border
                   {selectedProfile === alt.profile
                    ? 'border-accent/40 text-text'
                    : 'bg-surface-raised/50 border-border/50 text-text-muted hover:bg-surface-raised hover:text-text'}"
                  style={selectedProfile === alt.profile
                    ? `background: ${profileColors[alt.profile]}15; border-color: ${profileColors[alt.profile]}66`
                    : ''}
                >
                  <div class="flex items-center gap-2.5">
                    <!-- Color dot matching map line -->
                    <div
                      class="w-3 h-3 rounded-full shrink-0"
                      style="background: {profileColors[alt.profile] ??
                        '#64748b'}; opacity: {selectedProfile === alt.profile ? 1 : 0.4}"
                    ></div>
                    <span class="text-sm shrink-0">{profileIcons[alt.profile] ?? '🚲'}</span>
                    <div class="flex-1 min-w-0">
                      <div class="text-2xs font-medium">{alt.label}</div>
                      <div class="text-3xs text-text-subtle">
                        {alt.total_distance_km.toFixed(1)} km · {Math.round(
                          alt.total_duration_s / 60
                        )} min · ↗ {Math.round(alt.elevation_gain_m ?? 0)} m · ↘ {Math.round(
                          alt.elevation_loss_m ?? 0
                        )} m
                      </div>
                    </div>
                  </div>

                  <!-- Conditions summary, matching the curated route cards -->
                  {#if summary}
                    <div class="flex gap-3 text-3xs mt-1.5">
                      <span class="text-blue-400" title="Shade coverage">
                        ☀ {Math.round(summary.avgShade * 100)}% shade
                      </span>
                      <span
                        class={summary.avgWind > 0.1
                          ? 'text-green-400'
                          : summary.avgWind < -0.1
                            ? 'text-red-400'
                            : 'text-text-muted'}
                      >
                        💨 {windLabel(summary.avgWind)}
                      </span>
                      <span class={summary.maxPrecip > 0 ? 'text-purple-400' : 'text-text-subtle'}>
                        🌧 {precipLabel(summary.maxPrecip)}
                      </span>
                    </div>
                  {/if}

                  <!-- Expanded detail for the selected alternative -->
                  {#if selectedProfile === alt.profile}
                    {#if alt.elevation_profile?.length > 1}
                      <div class="mt-2">
                        <div class="text-3xs text-text-subtle mb-0.5">Elevation</div>
                        <ElevationSparkline
                          samples={alt.elevation_profile.map((point) => ({
                            distanceM: point.distance_m,
                            elevationM: point.elevation_m
                          }))}
                        />
                      </div>
                    {/if}
                    {#if segs.length > 0}
                      <div class="flex gap-3 mt-2">
                        <div class="flex-1">
                          <div class="text-3xs text-text-subtle mb-0.5">Shade</div>
                          <ConditionSparkline segments={segs} overlay="shade" />
                        </div>
                        <div class="flex-1">
                          <div class="text-3xs text-text-subtle mb-0.5">Wind</div>
                          <ConditionSparkline segments={segs} overlay="wind" />
                        </div>
                        <div class="flex-1">
                          <div class="text-3xs text-text-subtle mb-0.5">Rain</div>
                          <ConditionSparkline segments={segs} overlay="rain" />
                        </div>
                      </div>
                      {#if summary && summary.signals > 0}
                        <div class="text-3xs text-text-subtle mt-1.5">
                          🚦 {summary.signals} signals along route
                        </div>
                      {/if}
                    {:else}
                      <div class="text-3xs text-text-subtle italic mt-1.5">Loading conditions…</div>
                    {/if}
                  {/if}
                </button>
              {/each}
            </div>
          {:else}
            <!-- Suggested routes -->
            <AsyncBoundary
              loading={$discoveryLoading}
              error={$discoveryError}
              empty={filteredRoutes.length === 0}
              loadingMessage="Loading..."
              emptyMessage="No routes in view"
              onRetry={retryLoad}
            >
              {#each filteredRoutes as route (route.id)}
                <RouteCard
                  route={routeDetailsCache.get(route.id) ?? route}
                  conditions={conditionsCache.get(route.id) ?? []}
                />
              {/each}
            </AsyncBoundary>
          {/if}
        </div>
      </div>

      <button
        onclick={() => resultsPanelCollapsed.set(true)}
        aria-expanded="true"
        aria-controls="results-panel-body"
        aria-label="Hide route list"
        class="w-8 shrink-0 flex flex-col items-center justify-center gap-3
               border-l border-border/50 text-text-subtle
               hover:text-text-muted hover:bg-surface-raised/60 transition-colors"
      >
        <Chevron direction="left" />
        <span class="w-1 h-10 rounded-full bg-border-strong" aria-hidden="true"></span>
      </button>
    </div>
  {/if}
</div>

<style>
  /* Folded sidebar tab: the label reads top-to-bottom along the tab. */
  .vertical-label {
    writing-mode: vertical-rl;
  }
</style>
