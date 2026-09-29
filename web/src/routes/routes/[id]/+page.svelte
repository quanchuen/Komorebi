<!-- web/src/routes/routes/[id]/+page.svelte -->
<script lang="ts">
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import ElevationProfile from '$lib/components/ElevationProfile.svelte';
  import ConditionPanel from '$lib/components/ConditionPanel.svelte';
  import ReviewList from '$lib/components/ReviewList.svelte';
  import MapOverlayToggle from '$lib/components/MapOverlayToggle.svelte';
  import DifficultyBadge from '$lib/components/ui/DifficultyBadge.svelte';
  import { departureAt } from '$lib/stores/map';
  import { plans } from '$lib/api/client';

  let { data }: { data: PageData } = $props();

  let route = $derived(data.route);
  let conditions = $derived(data.conditions?.segments ?? []);
  let geometry = $derived(route.geometry.coordinates.map((c) => [c[0], c[1]] as [number, number]));

  let planLoading = $state(false);
  async function planThisRide() {
    planLoading = true;
    try {
      const plan = await plans.createFromRoute(route.id, $departureAt);
      window.location.href = `/planner?plan=${plan.id}`;
    } catch {
      planLoading = false;
    }
  }
</script>

<svelte:head>
  <title>{route.name} — Komorebi</title>
  <meta
    name="description"
    content="{route.description || route.name} — {(route.distanceM / 1000).toFixed(
      1
    )} km cycling route."
  />
</svelte:head>

<div class="flex h-dvh w-screen overflow-hidden bg-surface-canvas">
  <!-- Detail panel -->
  <aside
    class="w-full md:w-96 h-full bg-surface-canvas border-r border-line overflow-y-auto flex-shrink-0 z-10"
  >
    <div class="p-5 space-y-5">
      <!-- Back -->
      <a
        href="/"
        class="inline-flex items-center gap-1 text-sm text-text-subtle hover:text-text-default"
      >
        ← Back
      </a>

      <!-- Title + meta -->
      <div class="space-y-1">
        <h1 class="text-xl font-bold text-text-default">{route.name}</h1>
        <div class="flex items-center gap-3 text-sm text-text-subtle">
          <span>{(route.distanceM / 1000).toFixed(1)} km</span>
          <span>+{route.elevationGainM} m</span>
          <DifficultyBadge difficulty={route.difficulty} />
        </div>
        {#if route.tags.length > 0}
          <div class="flex flex-wrap gap-1 mt-1">
            {#each route.tags as tag}
              <span class="text-xs bg-surface-base text-text-subtle rounded px-2 py-0.5">{tag}</span
              >
            {/each}
          </div>
        {/if}
      </div>

      {#if route.description}
        <p class="text-sm text-text-subtle leading-relaxed">{route.description}</p>
      {/if}

      <!-- Overlay toggle -->
      <div>
        <div class="text-xs text-text-subtle mb-2 uppercase tracking-wide">Condition overlay</div>
        <MapOverlayToggle />
      </div>

      <!-- Condition panel -->
      {#if conditions.length > 0}
        <div class="bg-surface-base rounded-xl p-4">
          <h2 class="text-sm font-semibold text-text-subtle mb-3">Conditions at departure</h2>
          <ConditionPanel segments={conditions} />
        </div>
      {/if}

      <!-- Elevation profile -->
      <div class="bg-surface-base rounded-xl p-4">
        <h2 class="text-sm font-semibold text-text-subtle mb-3">Elevation</h2>
        <ElevationProfile
          coordinates={route.geometry.coordinates as Array<[number, number, number?]>}
        />
      </div>

      <!-- Waypoints -->
      {#if route.waypoints && route.waypoints.length > 0}
        <div>
          <h2 class="text-sm font-semibold text-text-subtle mb-2">Stops</h2>
          <ul class="space-y-1">
            {#each route.waypoints.sort((a, b) => a.sortOrder - b.sortOrder) as wp}
              <li class="flex items-center gap-2 text-sm text-text-subtle">
                <span class="w-2 h-2 rounded-full bg-primary shrink-0"></span>
                {wp.name}
                <span class="text-xs text-text-subtle capitalize">({wp.type})</span>
              </li>
            {/each}
          </ul>
        </div>
      {/if}

      <!-- Reviews -->
      <ReviewList routeId={route.id} />

      <!-- Plan CTA -->
      <button
        onclick={planThisRide}
        disabled={planLoading}
        class="w-full bg-primary hover:bg-primary-hover disabled:opacity-50 text-on-primary font-semibold
               rounded-xl py-3 text-sm transition-colors"
      >
        {planLoading ? 'Creating plan…' : 'Plan this ride'}
      </button>
    </div>
  </aside>

  <!-- Map -->
  <div class="hidden md:block flex-1 relative">
    <Map
      highlightGeometry={geometry}
      conditionSegments={conditions}
      conditionRouteDistanceM={route.distanceM}
    />
  </div>
</div>
