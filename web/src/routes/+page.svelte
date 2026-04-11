<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import DiscoveryPanel from '$lib/components/DiscoveryPanel.svelte';
  import { highlightedRouteId, departureAt } from '$lib/stores/map';
  import { discoveryRoutes } from '$lib/stores/discovery';
  import { routes as routesApi } from '$lib/api/client';
  import type { RouteConditionSegment } from '$lib/api/types';

  let { data }: { data: PageData } = $props();

  // Condition segments for the currently highlighted route
  let highlightedConditions = $state<RouteConditionSegment[]>([]);
  let highlightedDistanceM = $state(0);
  let highlightedGeometry = $state<[number, number][] | null>(null);

  $effect(() => {
    const id = $highlightedRouteId;
    if (id) {
      const route = $discoveryRoutes.find((r) => r.id === id);
      if (route) {
        highlightedGeometry = route.geometry.coordinates.map(
          (c) => [c[0], c[1]] as [number, number]
        );
        highlightedDistanceM = route.distanceM;

        // Fetch conditions for this route
        routesApi
          .conditions(id, $departureAt)
          .then((c) => {
            highlightedConditions = c.segments;
          })
          .catch(() => {
            highlightedConditions = [];
          });
      }
    } else {
      highlightedGeometry = null;
      highlightedConditions = [];
    }
  });
</script>

<svelte:head>
  <title>Cyclist Map — Discover Routes</title>
  <meta name="description" content="Discover cycling routes with shade, wind, and rain forecasts." />
</svelte:head>

<div class="flex h-full w-full overflow-hidden bg-slate-900">
  <DiscoveryPanel initialRoutes={data.routes} />

  <div class="flex-1 relative">
    <Map
      highlightGeometry={highlightedGeometry}
      conditionSegments={highlightedConditions}
      conditionRouteDistanceM={highlightedDistanceM}
    />
  </div>
</div>
