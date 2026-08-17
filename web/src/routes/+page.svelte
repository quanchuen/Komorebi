<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import NavigationPanel from '$lib/components/NavigationPanel.svelte';
  import WeatherTimeline from '$lib/components/WeatherTimeline.svelte';
  import { highlightedRouteId, departureAt, mapInstance } from '$lib/stores/map';
  import { routes as routesApi } from '$lib/api/client';
  import type { RouteConditionSegment } from '$lib/api/types';

  let { data }: { data: PageData } = $props();

  let highlightedConditions = $state<RouteConditionSegment[]>([]);
  let highlightedDistanceM = $state(0);
  let highlightedGeometry = $state<[number, number][] | null>(null);
  let routeError = $state<string | null>(null);
  let navPanel: NavigationPanel;

  import { discoveryRoutes as dr } from '$lib/stores/discovery';
  import { onMount } from 'svelte';
  onMount(() => {
    if (data.routes.length > 0) dr.set(data.routes);
  });

  $effect(() => {
    const id = $highlightedRouteId;
    const selectedDepartureAt = $departureAt;
    let stale = false;
    if (!id) {
      highlightedGeometry = null;
      highlightedConditions = [];
      highlightedDistanceM = 0;
      routeError = null;
      return () => {
        stale = true;
      };
    }

    routesApi
      .get(id)
      .then((fullRoute) => {
        if (stale) return null;
        routeError = null;
        const coords = fullRoute.geometry.coordinates;
        if (coords.length > 0) {
          highlightedGeometry = coords.map((c) => [c[0], c[1]] as [number, number]);
          // Frame the highlighted route so a card click always brings it into view.
          const mapInst = $mapInstance;
          if (mapInst) {
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
        } else {
          highlightedGeometry = null;
        }
        highlightedDistanceM = fullRoute.distanceM;
        return routesApi.conditions(id, selectedDepartureAt);
      })
      .then((c) => {
        if (stale || !c) return;
        highlightedConditions = c.segments ?? [];
      })
      .catch((e) => {
        if (stale) return;
        // Clear all highlight state, not just conditions — a stale geometry
        // would keep the curated routes dimmed under a failed highlight.
        highlightedConditions = [];
        highlightedGeometry = null;
        highlightedDistanceM = 0;
        const msg = e instanceof Error ? e.message : String(e);
        if (msg.includes('Failed to fetch')) routeError = 'Cannot connect to API';
      });

    return () => {
      stale = true;
    };
  });

  function handleMapClick(detail: { lng: number; lat: number }) {
    navPanel?.handleMapClick(detail.lat, detail.lng);
  }

  // Escape hatch: no matter what state the highlight latched into, Esc
  // always returns the map to the undimmed all-routes view.
  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') highlightedRouteId.set(null);
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
  <title>Komorebi — Discover Routes</title>
  <meta
    name="description"
    content="Discover cycling routes with shade, wind, and rain forecasts."
  />
</svelte:head>

<!-- Vertical flex: map area (grows) + timeline (fixed at bottom) -->
<div class="flex flex-col h-full w-full overflow-hidden bg-surface">
  <!-- Map area with floating nav panel -->
  <div class="flex-1 relative min-h-0">
    <Map
      highlightGeometry={highlightedGeometry}
      conditionSegments={highlightedConditions}
      conditionRouteDistanceM={highlightedDistanceM}
      onclick={handleMapClick}
    />

    <!-- Floating navigation panel — inset from edges, doesn't touch bottom -->
    <NavigationPanel bind:this={navPanel} />

    <!-- Route error toast -->
    {#if routeError}
      <div
        class="absolute top-4 left-1/2 -translate-x-1/2 z-20
                  bg-danger-surface/90 border border-danger/40 text-danger text-xs
                  px-4 py-2 rounded-lg backdrop-blur"
      >
        {routeError}
      </div>
    {/if}
  </div>

  <!-- Weather timeline — fixed at bottom, never overlaps -->
  <WeatherTimeline />
</div>
