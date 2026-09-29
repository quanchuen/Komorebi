<!-- web/src/lib/components/PlannerPanel.svelte -->
<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    plannerStops,
    plannerPreferences,
    plannerResult,
    plannerPlan,
    plannerLoading,
    plannerError
  } from '$lib/stores/planner';
  import { departureAt } from '$lib/stores/map';
  import { routing, plans } from '$lib/api/client';
  import DepartureTimePicker from './DepartureTimePicker.svelte';
  import PreferenceSliders from './PreferenceSliders.svelte';
  import PlannerStopList from './PlannerStopList.svelte';
  import TaskInput from './TaskInput.svelte';
  import type { DirectionsRequest } from '$lib/api/types';

  // Trigger routing whenever stops (≥2) or preferences change
  let routeDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  function scheduleRoute() {
    if ($plannerStops.length < 2) return;
    if (routeDebounceTimer) clearTimeout(routeDebounceTimer);
    routeDebounceTimer = setTimeout(computeRoute, 600);
  }

  async function computeRoute() {
    plannerLoading.set(true);
    plannerError.set(null);
    try {
      const req: DirectionsRequest = {
        stops: $plannerStops.map((s) => ({ type: 'manual', lat: s.lat, lon: s.lon })),
        departure_at: $departureAt,
        speed_model: 'elevation',
        preferences: $plannerPreferences
      };
      const res = await routing.directions(req);
      plannerResult.set(res.alternatives?.[0] ?? null);
      if (!res.alternatives?.length) plannerError.set('No route found between these stops');
    } catch (e) {
      plannerError.set(e instanceof Error ? e.message : 'Routing failed');
    } finally {
      plannerLoading.set(false);
    }
  }

  const unsubStops = plannerStops.subscribe(() => scheduleRoute());
  const unsubPrefs = plannerPreferences.subscribe(() => scheduleRoute());
  const unsubDep = departureAt.subscribe(() => scheduleRoute());

  onDestroy(() => {
    unsubStops();
    unsubPrefs();
    unsubDep();
    if (routeDebounceTimer) clearTimeout(routeDebounceTimer);
  });

  let distanceLabel = $derived(
    $plannerResult ? `${$plannerResult.total_distance_km.toFixed(1)} km` : null
  );
  let durationLabel = $derived(
    $plannerResult ? `${Math.round($plannerResult.total_duration_s / 60)} min` : null
  );

  async function savePlan() {
    if (!$plannerPlan) {
      const p = await plans.create(
        $departureAt,
        $plannerPreferences.shade,
        $plannerPreferences.greenery,
        $plannerPreferences.wind
      );
      plannerPlan.set(p);
    }
  }
</script>

<aside class="flex flex-col w-full md:w-96 h-full bg-surface-canvas border-r border-line z-10">
  <div class="p-4 border-b border-line space-y-4">
    <div class="flex items-center gap-2">
      <a href="/" class="text-text-subtle hover:text-text-default text-sm">← Discover</a>
      <h1 class="text-base font-bold text-text-default ml-auto">Route Planner</h1>
    </div>

    <DepartureTimePicker />

    {#if $plannerStops.length >= 2}
      <PlannerStopList />
    {:else}
      <div
        class="text-sm text-text-subtle text-center py-3 border border-dashed border-line rounded-xl"
      >
        Click the map to add two or more stops
      </div>
    {/if}

    <div>
      <div class="text-xs text-text-subtle mb-2 uppercase tracking-wide">Preferences</div>
      <PreferenceSliders />
    </div>

    {#if $plannerPlan}
      <div>
        <div class="text-xs text-text-subtle mb-2 uppercase tracking-wide">Tasks</div>
        <TaskInput />
      </div>
    {/if}
  </div>

  {#if $plannerLoading}
    <div class="p-4 text-sm text-text-subtle text-center">Computing route…</div>
  {:else if $plannerError}
    <div class="p-4 text-sm text-danger">{$plannerError}</div>
  {:else if $plannerResult}
    <div class="p-4 border-b border-line">
      <div class="flex gap-4 text-sm">
        <div>
          <div class="text-text-subtle text-xs">Distance</div>
          <div class="text-text-default font-semibold">{distanceLabel}</div>
        </div>
        <div>
          <div class="text-text-subtle text-xs">Time</div>
          <div class="text-text-default font-semibold">{durationLabel}</div>
        </div>
      </div>
    </div>

    <div class="p-4">
      <button
        onclick={savePlan}
        class="w-full bg-primary hover:bg-primary-hover text-on-primary text-sm font-semibold rounded-xl py-2.5 transition-colors"
      >
        Save plan
      </button>
    </div>
  {/if}
</aside>
