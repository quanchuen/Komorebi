<!-- web/src/lib/components/RouteCard.svelte -->
<script lang="ts">
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import ConditionSparkline from './ConditionSparkline.svelte';
  import { highlightedRouteId } from '$lib/stores/map';

  interface Props {
    route: Route;
    conditions?: RouteConditionSegment[];
  }

  let { route, conditions = [] }: Props = $props();

  const difficultyColors: Record<string, string> = {
    easy: 'text-green-400 bg-green-400/10',
    moderate: 'text-yellow-400 bg-yellow-400/10',
    hard: 'text-orange-400 bg-orange-400/10',
    expert: 'text-red-400 bg-red-400/10'
  };

  function distanceLabel(m: number): string {
    return m >= 1000 ? `${(m / 1000).toFixed(1)} km` : `${m} m`;
  }

  // Compute weather summaries from conditions
  let avgShade = $derived(
    conditions.length > 0
      ? conditions.reduce((s, c) => s + c.shade, 0) / conditions.length
      : 0
  );
  let avgWind = $derived(
    conditions.length > 0
      ? conditions.reduce((s, c) => s + c.wind_benefit, 0) / conditions.length
      : 0
  );
  let maxPrecip = $derived(
    conditions.length > 0
      ? Math.max(...conditions.map((c) => c.precip))
      : 0
  );
  let totalSignals = $derived(
    conditions.reduce((s, c) => s + c.signals, 0)
  );

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
    ? 'bg-slate-700 border-sky-500'
    : 'bg-slate-800 border-slate-700 hover:bg-slate-700 hover:border-slate-600'}"
>
  <div class="flex items-start justify-between gap-2 mb-1">
    <h3 class="text-sm font-semibold text-slate-100 leading-snug">{route.name}</h3>
    <span class="text-[11px] font-medium capitalize px-2 py-0.5 rounded-full shrink-0 {difficultyColors[route.difficulty]}">
      {route.difficulty}
    </span>
  </div>

  {#if route.description}
    <p class="text-xs text-slate-500 mb-2 line-clamp-1">{route.description}</p>
  {/if}

  <div class="flex gap-3 text-xs text-slate-400 mb-2">
    <span>{distanceLabel(route.distanceM)}</span>
    <span>+{route.elevationGainM}m</span>
    {#if route.tags && route.tags.length > 0}
      <span class="text-slate-500">{route.tags.slice(0, 2).join(' · ')}</span>
    {/if}
  </div>

  <!-- Weather / conditions summary -->
  {#if conditions.length > 0}
    <div class="flex gap-3 text-[11px] mb-3">
      <span class="text-blue-400" title="Shade coverage">
        ☀ {Math.round(avgShade * 100)}% shade
      </span>
      <span class="{avgWind > 0.1 ? 'text-green-400' : avgWind < -0.1 ? 'text-red-400' : 'text-slate-400'}"
            title="{windLabel(avgWind)}">
        {windIcon(avgWind)} {windLabel(avgWind)}
      </span>
      <span class="{maxPrecip > 0 ? 'text-purple-400' : 'text-slate-500'}" title="Precipitation">
        🌧 {precipLabel(maxPrecip)}
      </span>
    </div>

    <!-- Condition sparklines -->
    <div class="flex gap-3">
      <div class="flex-1">
        <div class="text-[10px] text-slate-500 mb-0.5">Shade</div>
        <ConditionSparkline segments={conditions} overlay="shade" />
      </div>
      <div class="flex-1">
        <div class="text-[10px] text-slate-500 mb-0.5">Wind</div>
        <ConditionSparkline segments={conditions} overlay="wind" />
      </div>
      <div class="flex-1">
        <div class="text-[10px] text-slate-500 mb-0.5">Rain</div>
        <ConditionSparkline segments={conditions} overlay="rain" />
      </div>
    </div>

    {#if totalSignals > 0}
      <div class="text-[10px] text-slate-500 mt-2">
        🚦 {totalSignals} signals along route
      </div>
    {/if}
  {:else}
    <div class="text-[10px] text-slate-500 italic">Set departure time to see conditions</div>
  {/if}
</button>
