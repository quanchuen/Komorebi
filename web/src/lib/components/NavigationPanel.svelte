<!-- web/src/lib/components/NavigationPanel.svelte -->
<!-- Floating panel: From/To inputs with insertable stops, route suggestions -->
<script lang="ts">
  import { routing, discovery, routes as routesApi } from '$lib/api/client';
  import { departureAt, highlightedRouteId, bboxString, mapInstance } from '$lib/stores/map';
  import { discoveryRoutes, discoveryLoading, discoveryError } from '$lib/stores/discovery';
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import RouteCard from './RouteCard.svelte';
  import MapOverlayToggle from './MapOverlayToggle.svelte';

  interface Stop {
    id: string;
    label: string;
    lat: number | null;
    lon: number | null;
  }

  let stops = $state<Stop[]>([
    { id: crypto.randomUUID(), label: '', lat: null, lon: null },
    { id: crypto.randomUUID(), label: '', lat: null, lon: null }
  ]);

  let conditionsCache = $state(new Map<string, RouteConditionSegment[]>());
  let routeGeometryCache = $state(new Map<string, number[][]>());

  // Which input is awaiting a map click
  let awaitingClickIndex = $state<number | null>(null);

  function addStopAfter(index: number) {
    const newStop: Stop = { id: crypto.randomUUID(), label: '', lat: null, lon: null };
    stops = [...stops.slice(0, index + 1), newStop, ...stops.slice(index + 1)];
  }

  function removeStop(index: number) {
    if (stops.length <= 2) return;
    stops = stops.filter((_, i) => i !== index);
  }

  function setStopFromMap(index: number) {
    awaitingClickIndex = index;
  }

  // Called by parent when map is clicked
  export function handleMapClick(lat: number, lon: number) {
    if (awaitingClickIndex !== null) {
      const idx = awaitingClickIndex;
      stops = stops.map((s, i) =>
        i === idx ? { ...s, lat, lon, label: `${lat.toFixed(4)}, ${lon.toFixed(4)}` } : s
      );
      awaitingClickIndex = null;
    }
  }

  // Check if we have enough stops to route
  let canRoute = $derived(stops.filter((s) => s.lat !== null).length >= 2);

  // Load routes in viewport
  async function loadRoutes(bbox: string | null, departure: string) {
    if (!bbox) return;
    discoveryLoading.set(true);
    discoveryError.set(null);
    try {
      const res = await discovery.viewport({ bbox });
      discoveryRoutes.set(res.routes);
      await Promise.allSettled(
        res.routes.map(async (r) => {
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

  $effect(() => {
    loadRoutes($bboxString, $departureAt);
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
          // Fly to route
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
<div class="absolute top-4 left-4 bottom-20 z-10 w-80
            flex flex-col gap-3 pointer-events-none">

  <!-- Navigation card -->
  <div class="bg-slate-900/90 backdrop-blur-lg border border-slate-700/50
              rounded-2xl shadow-2xl p-4 pointer-events-auto">
    <h1 class="text-sm font-bold text-slate-100 mb-3">Cyclist Map</h1>

    <!-- Stop inputs -->
    <div class="flex flex-col gap-0">
      {#each stops as stop, i (stop.id)}
        <div class="flex items-center gap-2">
          <!-- Dot indicator -->
          <div class="flex flex-col items-center w-4 shrink-0">
            {#if i === 0}
              <div class="w-3 h-3 rounded-full bg-emerald-500 border-2 border-emerald-300"></div>
            {:else if i === stops.length - 1}
              <div class="w-3 h-3 rounded-full bg-red-500 border-2 border-red-300"></div>
            {:else}
              <div class="w-2.5 h-2.5 rounded-full bg-sky-400 border-2 border-sky-300"></div>
            {/if}
          </div>

          <!-- Input -->
          <div class="flex-1 relative">
            <input
              type="text"
              placeholder={i === 0 ? 'From...' : i === stops.length - 1 ? 'To...' : 'Stop...'}
              value={stop.label}
              readonly
              onclick={() => setStopFromMap(i)}
              class="w-full bg-slate-800/80 border text-slate-100 text-xs rounded-lg
                     px-3 py-2 cursor-pointer transition-colors
                     {awaitingClickIndex === i
                ? 'border-sky-500 ring-1 ring-sky-500/30'
                : 'border-slate-700 hover:border-slate-600'}
                     focus:outline-none placeholder:text-slate-500"
            />
            {#if awaitingClickIndex === i}
              <span class="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-sky-400 animate-pulse">
                Click map
              </span>
            {/if}
          </div>

          <!-- Remove button (only for intermediate stops) -->
          {#if i > 0 && i < stops.length - 1}
            <button
              onclick={() => removeStop(i)}
              class="text-slate-500 hover:text-red-400 text-xs w-5 h-5 flex items-center justify-center shrink-0"
              aria-label="Remove stop"
            >x</button>
          {/if}
        </div>

        <!-- Add stop button between entries -->
        {#if i < stops.length - 1}
          <div class="flex items-center ml-[7px] my-0.5">
            <div class="w-px h-3 bg-slate-600"></div>
            {#if canRoute || stops.every(s => s.lat !== null)}
              <button
                onclick={() => addStopAfter(i)}
                class="ml-3 text-[10px] text-slate-500 hover:text-sky-400
                       bg-slate-800 hover:bg-slate-700 border border-slate-700
                       rounded px-1.5 py-0 leading-4 transition-colors"
                aria-label="Add stop"
              >+</button>
            {/if}
          </div>
        {/if}
      {/each}
    </div>

    <!-- Overlay toggles -->
    <div class="mt-3 pt-3 border-t border-slate-700/50 flex items-center justify-between">
      <span class="text-[10px] text-slate-500 uppercase tracking-wider">Overlay</span>
      <MapOverlayToggle />
    </div>
  </div>

  <!-- Route suggestions -->
  <div class="flex-1 min-h-0 overflow-y-auto pointer-events-auto
              bg-slate-900/80 backdrop-blur-lg border border-slate-700/50
              rounded-2xl shadow-2xl p-3 space-y-2">

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
  </div>
</div>
