<!-- web/src/routes/planner/+page.svelte -->
<script lang="ts">
  import Map from '$lib/components/Map.svelte';
  import PlannerPanel from '$lib/components/PlannerPanel.svelte';
  import MapOverlayToggle from '$lib/components/MapOverlayToggle.svelte';
  import { plannerStops, plannerResult } from '$lib/stores/planner';
  import { routeWaypoints } from '$lib/stores/map';
  import type { RouteConditionSegment } from '$lib/api/types';

  let stopCounter = $state(0);

  function handleMapClick(detail: { lng: number; lat: number }) {
    const { lng, lat } = detail;
    stopCounter++;
    plannerStops.update((stops) => [
      ...stops,
      {
        id: `stop-${stopCounter}`,
        lat,
        lon: lng,
        label: `Stop ${stopCounter} (${lat.toFixed(4)}, ${lng.toFixed(4)})`
      }
    ]);
  }

  // Stops between the first and last are numbered waypoint markers.
  $effect(() => {
    routeWaypoints.set($plannerStops.slice(1, -1).map((s) => [s.lon, s.lat]));
    return () => routeWaypoints.set([]);
  });

  let planGeometry = $derived(
    $plannerResult?.geometry.coordinates.map((c: number[]) => [c[0], c[1]] as [number, number]) ??
      null
  );

  // Directions alternatives carry no per-segment conditions; the overlay stays off here.
  let planConditions: RouteConditionSegment[] = $derived([] as RouteConditionSegment[]);
  let planDistanceM = $derived(($plannerResult?.total_distance_km ?? 0) * 1000);
</script>

<svelte:head>
  <title>Route Planner — Komorebi</title>
</svelte:head>

<div class="flex h-dvh w-screen overflow-hidden bg-surface">
  <PlannerPanel />

  <div class="flex-1 relative">
    <Map
      onclick={handleMapClick}
      highlightGeometry={planGeometry}
      conditionSegments={planConditions}
      conditionRouteDistanceM={planDistanceM}
    />

    <!-- Overlay toggle (floating, top-left of map) -->
    <div class="absolute top-4 left-4 z-10">
      <MapOverlayToggle />
    </div>

    {#if $plannerResult}
      <div
        class="absolute bottom-6 left-1/2 -translate-x-1/2 z-10
                  bg-surface/90 border border-border rounded-full px-4 py-2
                  text-xs text-text-muted backdrop-blur-sm"
      >
        Click map to add more stops
      </div>
    {:else if $plannerStops.length === 0}
      <div
        class="absolute bottom-6 left-1/2 -translate-x-1/2 z-10
                  bg-surface/90 border border-border rounded-full px-4 py-2
                  text-xs text-text-muted backdrop-blur-sm"
      >
        Click map to set origin
      </div>
    {:else if $plannerStops.length === 1}
      <div
        class="absolute bottom-6 left-1/2 -translate-x-1/2 z-10
                  bg-surface/90 border border-border rounded-full px-4 py-2
                  text-xs text-text-muted backdrop-blur-sm"
      >
        Click map to set destination
      </div>
    {/if}
  </div>
</div>
