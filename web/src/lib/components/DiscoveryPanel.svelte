<!-- web/src/lib/components/DiscoveryPanel.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { discovery, routes as routesApi } from '$lib/api/client';
  import { discoveryRoutes, discoveryLoading, discoveryFilters, discoveryError } from '$lib/stores/discovery';
  import { departureAt, bboxString, highlightedRouteId, activeOverlay, mapInstance } from '$lib/stores/map';
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import RouteCard from './RouteCard.svelte';
  import DepartureTimePicker from './DepartureTimePicker.svelte';
  import FilterChips from './FilterChips.svelte';
  import MapOverlayToggle from './MapOverlayToggle.svelte';

  interface Props {
    initialRoutes?: Route[];
  }

  let { initialRoutes = [] }: Props = $props();

  let conditionsCache = $state(new Map<string, RouteConditionSegment[]>());
  let sheetOpen = $state(false);

  onMount(() => {
    if (initialRoutes.length > 0) {
      discoveryRoutes.set(initialRoutes);
    }
  });

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
            } catch {
              // conditions fetch failed for this route — skip
            }
          }
        })
      );
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('Failed to fetch') || msg.includes('fetch')) {
        discoveryError.set('Cannot connect to API server (localhost:8080). Is it running?');
      } else {
        discoveryError.set(`Error loading routes: ${msg}`);
      }
    } finally {
      discoveryLoading.set(false);
    }
  }

  $effect(() => {
    loadRoutes($bboxString, $departureAt);
  });

  let filteredRoutes = $derived($discoveryRoutes.filter((r) => {
    const f = $discoveryFilters;
    if (f.difficulty && r.difficulty !== f.difficulty) return false;
    if (f.searchQuery && !r.name.toLowerCase().includes(f.searchQuery.toLowerCase())) return false;
    return true;
  }));

  $effect(() => {
    const id = $highlightedRouteId;
    const mapInst = $mapInstance;
    if (id && mapInst) {
      const route = $discoveryRoutes.find((r) => r.id === id);
      if (route && route.geometry.coordinates.length > 0) {
        const coords = route.geometry.coordinates;
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
  });

  let searchQuery = $state($discoveryFilters.searchQuery);

  function handleSearchInput(e: Event) {
    const v = (e.target as HTMLInputElement).value;
    searchQuery = v;
    discoveryFilters.update((f) => ({ ...f, searchQuery: v }));
  }

  function retryLoad() {
    discoveryError.set(null);
    loadRoutes($bboxString, $departureAt);
  }
</script>

<!-- Desktop: left panel -->
<aside class="hidden md:flex flex-col w-96 h-full bg-slate-900 border-r border-slate-800 z-10">
  <div class="p-4 border-b border-slate-800 space-y-3">
    <h1 class="text-lg font-bold text-slate-100">Cyclist Map</h1>
    <DepartureTimePicker />
    <FilterChips />
    <div class="flex items-center justify-between">
      <span class="text-xs text-slate-500">Overlay</span>
      <MapOverlayToggle />
    </div>
    <div class="relative">
      <input
        type="text"
        placeholder="Search routes..."
        value={searchQuery}
        oninput={handleSearchInput}
        class="w-full bg-slate-800 border border-slate-700 text-slate-100 text-sm rounded-lg
               pl-3 pr-3 py-2 focus:outline-none focus:ring-2 focus:ring-sky-500"
      />
    </div>
  </div>

  <div class="flex-1 overflow-y-auto p-3 space-y-2">
    {#if $discoveryError}
      <div class="rounded-lg bg-red-950 border border-red-800 p-4 text-center space-y-2">
        <div class="text-red-400 text-sm font-medium">Connection Error</div>
        <div class="text-red-300 text-xs">{$discoveryError}</div>
        <button
          onclick={retryLoad}
          class="mt-2 text-xs bg-red-800 hover:bg-red-700 text-red-100 px-3 py-1 rounded"
        >
          Retry
        </button>
      </div>
    {:else if $discoveryLoading}
      <div class="text-slate-500 text-sm text-center py-8">Loading routes...</div>
    {:else if filteredRoutes.length === 0}
      <div class="text-slate-500 text-sm text-center py-8">No routes found</div>
    {:else}
      {#each filteredRoutes as route (route.id)}
        <RouteCard {route} conditions={conditionsCache.get(route.id) ?? []} />
      {/each}
    {/if}
  </div>
</aside>

<!-- Mobile: bottom sheet -->
<div class="md:hidden fixed bottom-0 inset-x-0 z-20">
  <div
    class="bg-slate-900 border-t border-slate-800 rounded-t-2xl transition-all duration-300"
    style="height: {sheetOpen ? '70vh' : '6rem'};"
  >
    <button
      onclick={() => (sheetOpen = !sheetOpen)}
      class="w-full flex flex-col items-center pt-3 pb-2 gap-1"
      aria-label="Toggle route list"
    >
      <div class="w-10 h-1 rounded-full bg-slate-600"></div>
      <span class="text-xs text-slate-400">
        {#if $discoveryError}
          Connection error
        {:else}
          {sheetOpen ? 'Hide' : `${filteredRoutes.length} routes`}
        {/if}
      </span>
    </button>

    {#if sheetOpen}
      <div class="px-4 pb-2 space-y-2">
        <DepartureTimePicker />
        <FilterChips />
        <MapOverlayToggle />
      </div>
      <div class="overflow-y-auto px-3 space-y-2" style="height: calc(70vh - 8rem);">
        {#if $discoveryError}
          <div class="rounded-lg bg-red-950 border border-red-800 p-4 text-center">
            <div class="text-red-400 text-sm">{$discoveryError}</div>
            <button onclick={retryLoad} class="mt-2 text-xs bg-red-800 text-red-100 px-3 py-1 rounded">Retry</button>
          </div>
        {:else}
          {#each filteredRoutes as route (route.id)}
            <RouteCard {route} conditions={conditionsCache.get(route.id) ?? []} />
          {/each}
        {/if}
      </div>
    {/if}
  </div>
</div>
