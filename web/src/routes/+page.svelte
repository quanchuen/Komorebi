<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import NavigationPanel from '$lib/components/NavigationPanel.svelte';
  import WeatherTimeline from '$lib/components/WeatherTimeline.svelte';
  import { highlightedRouteId, departureAt } from '$lib/stores/map';
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
    if (!id) {
      highlightedGeometry = null;
      highlightedConditions = [];
      highlightedDistanceM = 0;
      routeError = null;
      return;
    }

    routesApi.get(id)
      .then((fullRoute) => {
        routeError = null;
        const coords = fullRoute.geometry;
        if (Array.isArray(coords) && coords.length > 0) {
          highlightedGeometry = coords.map((c: number[]) => [c[0], c[1]] as [number, number]);
        } else {
          highlightedGeometry = null;
        }
        highlightedDistanceM = fullRoute.distance_m ?? fullRoute.distanceM ?? 0;
        return routesApi.conditions(id, $departureAt);
      })
      .then((c) => {
        highlightedConditions = c.segments ?? [];
      })
      .catch((e) => {
        highlightedConditions = [];
        const msg = e instanceof Error ? e.message : String(e);
        if (msg.includes('Failed to fetch')) routeError = 'Cannot connect to API';
      });
  });

  function handleMapClick(detail: { lng: number; lat: number }) {
    navPanel?.handleMapClick(detail.lat, detail.lng);
  }
</script>

<svelte:head>
  <title>Cyclist Map — Discover Routes</title>
  <meta name="description" content="Discover cycling routes with shade, wind, and rain forecasts." />
</svelte:head>

<!-- Vertical flex: map area (grows) + timeline (fixed at bottom) -->
<div class="flex flex-col h-full w-full overflow-hidden bg-slate-900">

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
      <div class="absolute top-4 left-1/2 -translate-x-1/2 z-20
                  bg-red-950/90 border border-red-800 text-red-300 text-xs
                  px-4 py-2 rounded-lg backdrop-blur">
        {routeError}
      </div>
    {/if}
  </div>

  <!-- Weather timeline — fixed at bottom, never overlaps -->
  <WeatherTimeline />
</div>
