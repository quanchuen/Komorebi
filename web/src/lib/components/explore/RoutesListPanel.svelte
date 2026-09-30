<!-- web/src/lib/components/explore/RoutesListPanel.svelte -->
<!-- Frame I: "Routes in view". A sort control (Nearest · Shadiest now ·
     Shortest), filter chips (distance, difficulty, loops only) and one row
     per route: name, distance · climb · km away, difficulty badge, shade %
     now. The list follows the viewport; hovering a row lifts its line on the
     map; selecting opens the route card (frame D). -->
<script lang="ts">
  import DifficultyBadge from '$lib/components/DifficultyBadge.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PanelHeader from './PanelHeader.svelte';
  import Sheet from './Sheet.svelte';
  import type { Difficulty, Route } from '$lib/api/types';
  import { departureAt, hoveredRouteId, liveNavigationPosition, mapCentre } from '$lib/stores/map';
  import { isMobile } from '$lib/stores/ui';
  import {
    routesInView,
    routesInViewLoading,
    routesInViewError,
    routeDetails,
    routeConditions,
    ensureDetail,
    ensureConditions
  } from '$lib/stores/routesInView';
  import { km, climb } from '$lib/utils/format';
  import { metresBetween, LOOP_M } from '$lib/utils/mapMarkers';
  import { sunTimes } from '$lib/utils/sun';

  interface Props {
    onselect: (id: string) => void;
    onclose: () => void;
  }
  let { onselect, onclose }: Props = $props();

  type Sort = 'nearest' | 'shadiest' | 'shortest';
  let sort = $state<Sort>('nearest');
  let maxKm = $state<number | null>(null);
  let difficulty = $state<Difficulty | null>(null);
  let loopsOnly = $state(false);

  const ROW_CAP = 20;

  // Every row needs its geometry (loops, start point) and conditions (shade
  // now), fetched lazily and capped so a busy viewport never floods the API.
  $effect(() => {
    const dep = $departureAt;
    for (const r of $routesInView.slice(0, ROW_CAP)) {
      void ensureDetail(r.id);
      void ensureConditions(r.id, dep);
    }
  });

  let origin = $derived.by(() => {
    const me = $liveNavigationPosition;
    if (me) return { lat: me.latitude, lon: me.longitude, mine: true };
    const c = $mapCentre;
    return c ? { ...c, mine: false } : null;
  });

  function startOf(r: Route): [number, number] | null {
    const d = $routeDetails.get(r.id);
    const c = d?.geometry.coordinates[0];
    return c ? [c[0], c[1]] : null;
  }
  function isLoop(r: Route): boolean {
    const d = $routeDetails.get(r.id);
    const cs = d?.geometry.coordinates;
    if (!cs || cs.length < 2) return false;
    const a = cs[0];
    const b = cs[cs.length - 1];
    return metresBetween([a[0], a[1]], [b[0], b[1]]) < LOOP_M;
  }
  function shadeNow(r: Route): number | null {
    const segs = $routeConditions.get(r.id);
    if (!segs || segs.length === 0) return null;
    return segs.reduce((s, c) => s + c.shade, 0) / segs.length;
  }
  function awayM(r: Route): number | null {
    const s = startOf(r);
    return s && origin ? metresBetween([origin.lon, origin.lat], s) : null;
  }

  let afterSunset = $derived.by(() => {
    const c = $mapCentre ?? { lat: 35.6895, lon: 139.6917 };
    const t = sunTimes(new Date($departureAt), c.lat, c.lon);
    if (!t) return false;
    const d = new Date($departureAt).getTime();
    return d < t.sunrise.getTime() || d > t.sunset.getTime();
  });

  let rows = $derived.by(() => {
    let list = $routesInView.slice(0, ROW_CAP);
    const limitKm = maxKm;
    if (limitKm !== null) list = list.filter((r) => r.distanceM <= limitKm * 1000);
    if (difficulty) list = list.filter((r) => r.difficulty === difficulty);
    if (loopsOnly) list = list.filter(isLoop);
    const by =
      (f: (r: Route) => number | null, desc = false) =>
      (a: Route, b: Route) => {
        const x = f(a);
        const y = f(b);
        if (x === null && y === null) return 0;
        if (x === null) return 1;
        if (y === null) return -1;
        return desc ? y - x : x - y;
      };
    if (sort === 'nearest') return [...list].sort(by(awayM));
    if (sort === 'shadiest') return [...list].sort(by(shadeNow, true));
    return [...list].sort((a, b) => a.distanceM - b.distanceM);
  });

  const sorts: { id: Sort; label: string }[] = [
    { id: 'nearest', label: 'Nearest' },
    { id: 'shadiest', label: 'Shadiest now' },
    { id: 'shortest', label: 'Shortest' }
  ];

  function pick(id: string) {
    hoveredRouteId.set(null);
    onselect(id);
  }
</script>

{#snippet body()}
  <div class="px-4 pt-1 pb-2">
    <div
      class="flex h-8.5 px-0.5 rounded-md bg-surface-recessed"
      role="group"
      aria-label="Sort routes"
    >
      {#each sorts as s (s.id)}
        <button
          type="button"
          aria-pressed={sort === s.id}
          onclick={() => (sort = s.id)}
          class="seg {sort === s.id ? 'seg-on' : ''}"
        >
          {s.label}
        </button>
      {/each}
    </div>
    <div class="flex flex-nowrap gap-2 pt-3 -mb-1.5 overflow-x-auto" data-scroll="true">
      <label class="chip">
        <span class="sr-only">Maximum distance</span>
        <select
          class="chip-select"
          onchange={(e) => (maxKm = e.currentTarget.value ? Number(e.currentTarget.value) : null)}
        >
          <option value="">Any distance</option>
          <option value="10">Up to 10 km</option>
          <option value="25">Up to 25 km</option>
          <option value="50">Up to 50 km</option>
        </select>
        <Icon name="chevronDown" class="size-6 text-text-subtle pointer-events-none" />
      </label>
      <label class="chip">
        <span class="sr-only">Difficulty</span>
        <select
          class="chip-select"
          onchange={(e) => (difficulty = (e.currentTarget.value || null) as Difficulty | null)}
        >
          <option value="">Any difficulty</option>
          <option value="easy">Easy</option>
          <option value="moderate">Moderate</option>
          <option value="hard">Hard</option>
          <option value="expert">Expert</option>
        </select>
        <Icon name="chevronDown" class="size-6 text-text-subtle pointer-events-none" />
      </label>
      <button
        type="button"
        aria-pressed={loopsOnly}
        onclick={() => (loopsOnly = !loopsOnly)}
        class="chip chip-toggle {loopsOnly ? 'bg-primary text-on-primary border-primary' : ''}"
      >
        <span class="px-0.5">Loops only</span>
      </button>
    </div>
  </div>
  <div class="h-px bg-hairline"></div>
  <div class="flex-1 min-h-0 overflow-y-auto px-2 py-2" data-scroll="true">
    {#if $routesInViewError}
      <p class="px-2 py-3 text-13 text-text-subtle">{$routesInViewError} — showing the map only.</p>
    {:else if rows.length === 0}
      <p class="px-2 py-3 text-13 text-text-subtle">
        {$routesInViewLoading
          ? 'Looking for curated routes…'
          : 'No curated routes match here — zoom out or clear a filter.'}
      </p>
    {:else}
      <ul class="flex flex-col">
        {#each rows as r, i (r.id)}
          {@const shade = shadeNow(r)}
          {@const away = awayM(r)}
          <li>
            {#if i > 0}
              <div class="mx-2 h-px bg-hairline"></div>
            {/if}
            <button
              type="button"
              onclick={() => pick(r.id)}
              onmouseenter={() => hoveredRouteId.set(r.id)}
              onmouseleave={() => hoveredRouteId.set(null)}
              onfocus={() => hoveredRouteId.set(r.id)}
              onblur={() => hoveredRouteId.set(null)}
              class="row"
            >
              <span class="flex-1 min-w-0">
                <span class="block text-sm font-semibold text-text-strong truncate">{r.name}</span>
                <span class="block text-xs text-text-subtle pt-1">
                  {km(r.distanceM)} · {climb(r.elevationGainM)}{away !== null && origin?.mine
                    ? ` · ${km(away)} away`
                    : ''}
                </span>
                <span class="block pt-1.5"><DifficultyBadge difficulty={r.difficulty} /></span>
              </span>
              <span class="self-end text-xs text-text tabular-nums whitespace-nowrap pb-0.5">
                {afterSunset
                  ? '— after sunset'
                  : shade === null
                    ? ''
                    : `${Math.round(shade * 100)}% shade now`}
              </span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/snippet}

{#if $isMobile}
  <Sheet label="Routes in view" role="dialog" class="sheet-routes">
    <PanelHeader title="Routes in view" count={$routesInView.length} {onclose} />
    {@render body()}
  </Sheet>
{:else}
  <div
    class="flex flex-col bg-surface-base border border-line rounded-lg shadow-lg max-h-full overflow-hidden"
  >
    <PanelHeader title="Routes in view" count={$routesInView.length} {onclose} />
    {@render body()}
  </div>
{/if}

<style>
  /* Penpot "Segmented / sort": a 34px track with 30px segments. Each button
     is 44px tall with unpainted 7px borders above and below, so the hit box
     meets the rule while the face stays 30px. */
  .seg {
    flex: 1;
    height: 44px;
    margin: -5px 0;
    border: 0 solid transparent;
    border-top-width: 7px;
    border-bottom-width: 7px;
    background-clip: padding-box;
    border-radius: calc(var(--radius-sm) + 7px);
    background-color: transparent;
    font-size: var(--text-13);
    font-weight: 500;
    color: var(--color-text-subtle);
    transition: color 0.15s ease;
  }
  .seg:hover {
    color: var(--color-text);
  }
  .seg-on {
    background-color: var(--color-surface-base);
    box-shadow: inset 0 0 0 1px var(--color-line);
    font-weight: 600;
    color: var(--color-text-strong);
  }
  .seg:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -6px;
  }
  /* Penpot "Chip / filter": 30px pills, 12px medium, line ring. The 44px hit
     box comes from the select / button inside spanning 7px above and below. */
  .chip {
    position: relative;
    display: inline-flex;
    align-items: center;
    height: 44px;
    flex-shrink: 0;
  }
  .chip::before {
    content: '';
    position: absolute;
    inset: 7px 0;
    border-radius: 999px;
    border: 1px solid var(--color-line);
    background: var(--color-surface-base);
    pointer-events: none;
  }
  .chip > :global(*) {
    position: relative;
  }
  .chip-select {
    appearance: none;
    height: 44px;
    padding: 0 0 0 12px;
    background: transparent;
    border: 0;
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--color-text-strong);
  }
  .chip-select:focus-visible {
    outline: none;
  }
  .chip:has(.chip-select:focus-visible)::before {
    box-shadow: 0 0 0 2px var(--color-focus);
  }
  .chip-toggle {
    padding: 0 12px;
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--color-text-strong);
    background: transparent;
    border: 0;
  }
  .chip-toggle[aria-pressed='true']::before {
    background: var(--color-primary);
    border-color: var(--color-primary);
  }
  .chip-toggle[aria-pressed='true'] {
    color: var(--color-on-primary);
  }
  .chip-toggle:focus-visible {
    outline: none;
  }
  .chip-toggle:focus-visible::before {
    box-shadow: 0 0 0 2px var(--color-focus);
  }
  /* Penpot "RouteRow": 76px, radius 8, hover surface.elevated + hairline. */
  .row {
    display: flex;
    width: 100%;
    gap: 8px;
    min-height: 76px;
    padding: 10px 12px;
    border-radius: var(--radius-md);
    border: 1px solid transparent;
    text-align: left;
    background: transparent;
  }
  .row:hover,
  .row:focus-visible {
    background: var(--color-surface-elevated);
    border-color: var(--color-hairline);
    outline: none;
  }
  :global(.sheet-routes) {
    height: 72dvh;
  }
</style>
