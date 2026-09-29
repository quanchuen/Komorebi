<!-- web/src/lib/components/RouteCard.svelte -->
<script lang="ts">
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import ConditionSparkline from './ConditionSparkline.svelte';
  import ElevationSparkline from './ElevationSparkline.svelte';
  import DifficultyBadge from './ui/DifficultyBadge.svelte';
  import { highlightedRouteId } from '$lib/stores/map';

  interface Props {
    route: Route;
    conditions?: RouteConditionSegment[];
  }

  let { route, conditions = [] }: Props = $props();

  function distanceLabel(m: number): string {
    return m >= 1000 ? `${(m / 1000).toFixed(1)} km` : `${m} m`;
  }

  // Compute weather summaries from conditions
  let avgShade = $derived(
    conditions.length > 0 ? conditions.reduce((s, c) => s + c.shade, 0) / conditions.length : 0
  );
  let avgWind = $derived(
    conditions.length > 0
      ? conditions.reduce((s, c) => s + c.wind_benefit, 0) / conditions.length
      : 0
  );
  let maxPrecip = $derived(
    conditions.length > 0 ? Math.max(...conditions.map((c) => c.precip)) : 0
  );
  let totalSignals = $derived(conditions.reduce((s, c) => s + c.signals, 0));
  function distanceBetween(a: number[], b: number[]): number {
    const radians = Math.PI / 180;
    const lat1 = a[1] * radians;
    const lat2 = b[1] * radians;
    const deltaLat = (b[1] - a[1]) * radians;
    const deltaLon = (b[0] - a[0]) * radians;
    const h =
      Math.sin(deltaLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLon / 2) ** 2;
    return 6371000 * 2 * Math.atan2(Math.sqrt(h), Math.sqrt(1 - h));
  }

  let elevationSamples = $derived.by(() => {
    let distanceM = 0;
    return route.geometry.coordinates.flatMap((coordinate, index, coordinates) => {
      if (index > 0) distanceM += distanceBetween(coordinates[index - 1], coordinate);
      return coordinate[2] === undefined ? [] : [{ distanceM, elevationM: coordinate[2] }];
    });
  });

  function windLabel(v: number): string {
    if (v > 0.3) return 'Tailwind';
    if (v < -0.3) return 'Headwind';
    return 'Crosswind';
  }

  function windIcon(v: number): string {
    if (v > 0.3) return '↗';
    if (v < -0.3) return '↙';
    return '→';
  }

  function precipLabel(v: number): string {
    if (v <= 0) return 'Dry';
    if (v < 0.3) return 'Light';
    if (v < 0.6) return 'Moderate';
    return 'Heavy';
  }

  let isHighlighted = $derived($highlightedRouteId === route.id);

  function handleClick() {
    highlightedRouteId.set(isHighlighted ? null : route.id);
  }
</script>

<button
  onclick={handleClick}
  class="w-full text-left rounded-xl p-4 border transition-colors
         {isHighlighted
    ? 'bg-surface-overlay border-accent'
    : 'bg-surface-raised border-border hover:bg-surface-overlay hover:border-border-strong'}"
>
  <div class="flex items-start justify-between gap-2 mb-1">
    <h3 class="text-sm font-semibold text-text leading-snug">{route.name}</h3>
    <span class="shrink-0">
      <DifficultyBadge difficulty={route.difficulty} />
    </span>
  </div>

  {#if route.description}
    <p class="text-xs text-text-subtle mb-2 line-clamp-1">{route.description}</p>
  {/if}

  <div class="flex gap-3 text-xs text-text-muted mb-2">
    <span>{distanceLabel(route.distanceM)}</span>
    <span title="Estimated climbing">↗ {Math.round(route.elevationGainM)} m</span>
    <span title="Estimated descent">↘ {Math.round(route.elevationLossM)} m</span>
    {#if route.tags && route.tags.length > 0}
      <span class="text-text-subtle">{route.tags.slice(0, 2).join(' · ')}</span>
    {/if}
  </div>

  {#if elevationSamples.length > 1}
    <div class="mb-2">
      <div class="text-3xs text-text-subtle">Elevation · area / effort · line</div>
      <ElevationSparkline samples={elevationSamples} />
    </div>
  {/if}

  <!-- Weather / conditions summary -->
  {#if conditions.length > 0}
    <div class="flex gap-3 text-2xs mb-3">
      <span class="text-shade" title="Shade coverage">
        ☀ {Math.round(avgShade * 100)}% shade
      </span>
      <span
        class={avgWind > 0.1
          ? 'text-wind'
          : avgWind < -0.1
            ? 'text-wind-adverse'
            : 'text-text-muted'}
        title={windLabel(avgWind)}
      >
        {windIcon(avgWind)}
        {windLabel(avgWind)}
      </span>
      <span class={maxPrecip > 0 ? 'text-rain' : 'text-text-subtle'} title="Precipitation">
        🌧 {precipLabel(maxPrecip)}
      </span>
    </div>

    <!-- Condition sparklines -->
    <div class="flex gap-3">
      <div class="flex-1">
        <div class="text-3xs text-text-subtle mb-0.5">Shade</div>
        <ConditionSparkline segments={conditions} overlay="shade" />
      </div>
      <div class="flex-1">
        <div class="text-3xs text-text-subtle mb-0.5">Wind</div>
        <ConditionSparkline segments={conditions} overlay="wind" />
      </div>
      <div class="flex-1">
        <div class="text-3xs text-text-subtle mb-0.5">Rain</div>
        <ConditionSparkline segments={conditions} overlay="rain" />
      </div>
    </div>

    {#if totalSignals > 0}
      <div class="text-3xs text-text-subtle mt-2">
        🚦 {totalSignals} signals along route
      </div>
    {/if}
  {:else}
    <div class="text-3xs text-text-subtle italic">Set departure time to see conditions</div>
  {/if}
</button>
