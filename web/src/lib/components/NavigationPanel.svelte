<!-- web/src/lib/components/NavigationPanel.svelte -->
<script lang="ts">
  import { routing, discovery, routes as routesApi } from '$lib/api/client';
  import { departureAt, highlightedRouteId, bboxString, mapInstance } from '$lib/stores/map';
  import { discoveryRoutes, discoveryLoading, discoveryError } from '$lib/stores/discovery';
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import RouteCard from './RouteCard.svelte';
  import MapLayerControl from './MapLayerControl.svelte';

  interface Stop {
    id: string;
    label: string;
    query: string;        // typed search text
    lat: number | null;
    lon: number | null;
  }

  let stops = $state<Stop[]>([
    { id: crypto.randomUUID(), label: '', query: '', lat: null, lon: null },
    { id: crypto.randomUUID(), label: '', query: '', lat: null, lon: null }
  ]);

  let conditionsCache = $state(new Map<string, RouteConditionSegment[]>());
  let routeGeometryCache = $state(new Map<string, number[][]>());

  // Address lookup
  let activeInputIndex = $state<number | null>(null);
  let suggestions = $state<{ display_name: string; lat: string; lon: string }[]>([]);
  let searchDebounce: ReturnType<typeof setTimeout>;
  let inputRefs: HTMLInputElement[] = [];

  function focusInput(index: number) {
    activeInputIndex = index;
    suggestions = [];
  }

  async function searchAddress(query: string) {
    if (query.length < 3) { suggestions = []; return; }
    try {
      const res = await fetch(
        `/nominatim/search?format=json&q=${encodeURIComponent(query)}&limit=5&countrycodes=jp&accept-language=en`
      );
      if (res.ok) {
        suggestions = await res.json();
      }
    } catch {
      suggestions = [];
    }
  }

  function handleInput(index: number, e: Event) {
    const val = (e.target as HTMLInputElement).value;
    stops = stops.map((s, i) => i === index ? { ...s, query: val, label: val, lat: null, lon: null } : s);
    clearTimeout(searchDebounce);
    searchDebounce = setTimeout(() => searchAddress(val), 300);
  }

  function selectSuggestion(index: number, suggestion: { display_name: string; lat: string; lon: string }) {
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
    // If an input is focused/active, set it from map click
    if (activeInputIndex !== null) {
      const idx = activeInputIndex;
      const label = `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
      stops = stops.map((s, i) =>
        i === idx ? { ...s, lat, lon, label, query: label } : s
      );
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

  let canRoute = $derived(stops.filter((s) => s.lat !== null).length >= 2);
  let hasAllStops = $derived(stops.every((s) => s.lat !== null));

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

  async function doRoute() {
    const validStops = stops.filter((s) => s.lat !== null && s.lon !== null);
    if (validStops.length < 2) return;

    isRouting = true;
    routeError = null;
    alternatives = [];
    selectedProfile = null;

    try {
      const res = await routing.directions({
        stops: validStops.map((s) => ({ type: 'manual' as const, lat: s.lat!, lon: s.lon! })),
        departure_at: $departureAt,
        speed_model: 'elevation',
        preferences: { shade: 0.5, greenery: 0.5, wind: 0.5 }
      });

      alternatives = res.alternatives ?? [];
      if (alternatives.length > 0) {
        selectAlternative(alternatives[0].profile); // auto-select suggested
      }
    } catch (e) {
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

  function selectAlternative(profile: string) {
    selectedProfile = profile;
    const alt = alternatives.find((a) => a.profile === profile);
    if (!alt) return;

    const coords: [number, number][] = (alt.geometry?.coordinates ?? []).map(
      (c: number[]) => [c[0], c[1]] as [number, number]
    );

    const mapInst = $mapInstance;
    if (mapInst && coords.length > 0) {
      highlightedRouteId.set(null);
      const src = mapInst.getSource('highlight-route') as any;
      if (src) {
        src.setData({
          type: 'Feature',
          geometry: { type: 'LineString', coordinates: coords },
          properties: {}
        });
      }
      const lons = coords.map((c) => c[0]);
      const lats = coords.map((c) => c[1]);
      mapInst.fitBounds(
        [[Math.min(...lons), Math.min(...lats)], [Math.max(...lons), Math.max(...lats)]],
        { padding: 80, duration: 800 }
      );
    }
  }

  // Close suggestions when clicking outside
  function handleBlur() {
    setTimeout(() => { suggestions = []; }, 200);
  }

  // Load routes in viewport
  async function loadRoutes(bbox: string | null, departure: string) {
    if (!bbox) return;
    discoveryLoading.set(true);
    discoveryError.set(null);
    try {
      const res = await discovery.viewport({ bbox });
      discoveryRoutes.set(res.routes);
      // Prefetch conditions for first 3 routes only (avoid flooding API)
      await Promise.allSettled(
        res.routes.slice(0, 3).map(async (r) => {
          if (!conditionsCache.has(r.id)) {
            try {
              const c = await routesApi.conditions(r.id, departure);
              conditionsCache = new Map(conditionsCache).set(r.id, c.segments ?? []);
            } catch { /* skip */ }
          }
        })
      );
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      discoveryError.set(
        msg.includes('Failed to fetch')
          ? 'Cannot connect to API server (localhost:8080)'
          : `Error: ${msg}`
      );
    } finally {
      discoveryLoading.set(false);
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

  // Highlight route: fetch full geometry
  $effect(() => {
    const id = $highlightedRouteId;
    if (!id) return;
    if (!routeGeometryCache.has(id)) {
      routesApi.get(id).then((r) => {
        if (Array.isArray(r.geometry)) {
          routeGeometryCache = new Map(routeGeometryCache).set(id, r.geometry);
          const mapInst = $mapInstance;
          if (mapInst && r.geometry.length > 0) {
            const lons = r.geometry.map((c: number[]) => c[0]);
            const lats = r.geometry.map((c: number[]) => c[1]);
            mapInst.fitBounds(
              [[Math.min(...lons), Math.min(...lats)], [Math.max(...lons), Math.max(...lats)]],
              { padding: 80, duration: 800 }
            );
          }
        }
      }).catch(() => {});
    }
  });

  let filteredRoutes = $derived($discoveryRoutes);
</script>

<!-- Floating panel -->
<div class="absolute top-4 left-4 bottom-4 z-10 w-80
            flex flex-col gap-3 pointer-events-none">

  <!-- Navigation card -->
  <div class="bg-slate-900/90 backdrop-blur-lg border border-slate-700/50
              rounded-2xl shadow-2xl p-4 pointer-events-auto">

    <!-- Stop inputs with icon rail -->
    <div class="flex flex-col gap-0">
      {#each stops as stop, i (stop.id)}
        <!-- Stop row -->
        <div class="flex items-center gap-2">
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
                placeholder={i === 0 ? 'Start location' : i === stops.length - 1 ? 'Destination' : 'Via stop'}
                value={stop.query}
                onfocus={() => focusInput(i)}
                onblur={handleBlur}
                oninput={(e) => handleInput(i, e)}
                class="w-full bg-slate-800/80 border text-slate-100 text-xs rounded-lg
                       px-3 py-2 transition-colors
                       {activeInputIndex === i
                  ? 'border-sky-500 ring-1 ring-sky-500/30'
                  : 'border-slate-700 hover:border-slate-600'}
                       focus:outline-none placeholder:text-slate-500"
              />
              {#if i > 0 && i < stops.length - 1}
                <button
                  onclick={() => removeStop(i)}
                  class="text-slate-500 hover:text-red-400 text-sm w-5 h-5
                         flex items-center justify-center shrink-0"
                  aria-label="Remove stop"
                >&times;</button>
              {/if}
            </div>

            <!-- Address suggestions dropdown -->
            {#if activeInputIndex === i && suggestions.length > 0}
              <div class="absolute top-full left-0 right-0 mt-1 z-50
                          bg-slate-800 border border-slate-700 rounded-lg shadow-xl
                          overflow-hidden">
                {#each suggestions as s}
                  <button
                    onmousedown={() => selectSuggestion(i, s)}
                    class="w-full text-left px-3 py-2 text-xs text-slate-300
                           hover:bg-slate-700 transition-colors border-b border-slate-700/50
                           last:border-b-0"
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
          <div class="flex items-center gap-2 my-2">
            <!-- Vertical dash line under icon column -->
            <div class="w-5 shrink-0 flex justify-center">
              <div class="w-px h-4 border-l border-dashed border-slate-600"></div>
            </div>
            <!-- Dashed line + plus button -->
            <div class="flex-1 flex items-center gap-2">
              <div class="flex-1 border-t border-dashed border-slate-700"></div>
              <button
                onclick={() => addStopAfter(i)}
                class="text-[10px] text-slate-500 hover:text-amber-400
                       bg-slate-800 hover:bg-slate-700 border border-slate-700
                       hover:border-amber-500/50
                       rounded-full w-5 h-5 flex items-center justify-center
                       transition-colors"
                aria-label="Add stop"
              >+</button>
              <div class="flex-1 border-t border-dashed border-slate-700"></div>
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
          ? 'bg-sky-800 text-sky-300 cursor-wait'
          : 'bg-sky-600 hover:bg-sky-500 text-white'}"
      >
        {isRouting ? 'Finding routes...' : 'Route'}
      </button>
    {/if}

    <!-- Route error -->
    {#if routeError}
      <div class="mt-2 text-xs text-red-400 bg-red-950/50 rounded-lg px-3 py-2">
        {routeError}
      </div>
    {/if}

    <!-- Layer control -->
    <div class="mt-3 pt-3 border-t border-slate-700/50 flex items-center justify-end">
      <MapLayerControl />
    </div>
  </div>

  <!-- Results panel: route alternatives OR suggested routes -->
  <div class="flex-1 min-h-0 overflow-y-auto pointer-events-auto
              bg-slate-900/80 backdrop-blur-lg border border-slate-700/50
              rounded-2xl shadow-2xl p-3 space-y-2">

    {#if alternatives.length > 0}
      <!-- Route alternatives -->
      <div class="text-[10px] text-slate-500 uppercase tracking-wider px-1 mb-1">
        Routes found
      </div>
      <div class="flex flex-col gap-1.5">
        {#each alternatives as alt (alt.profile)}
          <button
            onclick={() => selectAlternative(alt.profile)}
            class="w-full flex items-center gap-2 px-3 py-2.5 rounded-lg text-left
                   transition-colors border
                   {selectedProfile === alt.profile
              ? 'bg-sky-600/15 border-sky-500/40 text-slate-100'
              : 'bg-slate-800/50 border-slate-700/50 text-slate-400 hover:bg-slate-800 hover:text-slate-200'}"
          >
            <span class="text-sm">{profileIcons[alt.profile] ?? '🚲'}</span>
            <div class="flex-1 min-w-0">
              <div class="text-[11px] font-medium">{alt.label}</div>
              <div class="text-[10px] text-slate-500">
                {alt.total_distance_km.toFixed(1)} km · {Math.round(alt.total_duration_s / 60)} min
              </div>
            </div>
          </button>
        {/each}
      </div>
    {:else}
      <!-- Suggested routes -->
      <div class="text-[10px] text-slate-500 uppercase tracking-wider px-1 mb-1">
        Suggested routes
      </div>

      {#if $discoveryError}
        <div class="rounded-lg bg-red-950/80 border border-red-800 p-3 text-center space-y-2">
          <div class="text-red-400 text-xs">{$discoveryError}</div>
          <button onclick={retryLoad}
            class="text-[10px] bg-red-800 hover:bg-red-700 text-red-100 px-3 py-1 rounded">
            Retry
          </button>
        </div>
      {:else if $discoveryLoading}
        <div class="text-slate-500 text-xs text-center py-6">Loading...</div>
      {:else if filteredRoutes.length === 0}
        <div class="text-slate-500 text-xs text-center py-6">No routes in view</div>
      {:else}
        {#each filteredRoutes as route (route.id)}
          <RouteCard {route} conditions={conditionsCache.get(route.id) ?? []} />
        {/each}
      {/if}
    {/if}
  </div>
</div>
