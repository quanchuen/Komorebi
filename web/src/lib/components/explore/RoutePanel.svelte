<!-- web/src/lib/components/explore/RoutePanel.svelte -->
<!-- Frame D: the selected curated route. Title · subtitle · difficulty and
     tag badges · stats (distance, climb, time at your pace) · elevation ·
     CONDITIONS IF YOU LEAVE AT HH:MM tiles (shade %, wind, rain) · source
     line · Ride this / Customize / Save · a disclosure row for Reviews ·
     Stops · Surface. Desktop: the 400px left panel. Mobile: a half sheet with
     the same content, Customize and the details behind "More". -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import DifficultyBadge from '$lib/components/DifficultyBadge.svelte';
  import ReviewList from '$lib/components/ReviewList.svelte';
  import ElevationProfileMini from './ElevationProfileMini.svelte';
  import SectionLabel from './SectionLabel.svelte';
  import Sheet from './Sheet.svelte';
  import type { Route, RouteConditionSegment } from '$lib/api/types';
  import { departureAt, liveNavigationPosition } from '$lib/stores/map';
  import { isMobile, requestRide } from '$lib/stores/ui';
  import { savedRoutes, toggleSavedRoute } from '$lib/stores/recents';
  import {
    foregroundNavigation,
    setNavigationRoute,
    startForegroundNavigation,
    stopForegroundNavigation
  } from '$lib/stores/navigation';
  import { km, climb, duration, hhmm } from '$lib/utils/format';
  import { summarise, windTile, rainTile } from '$lib/utils/conditions';
  import { metresBetween, LOOP_M } from '$lib/utils/mapMarkers';

  interface Props {
    route: Route;
    conditions: RouteConditionSegment[];
    onclose: () => void;
    oncustomize: (route: Route) => void;
  }
  let { route, conditions, onclose, oncustomize }: Props = $props();

  let coords = $derived(route.geometry.coordinates);
  let line = $derived(coords.map((c) => [c[0], c[1]] as [number, number]));
  let isLoop = $derived(line.length > 1 && metresBetween(line[0], line[line.length - 1]) < LOOP_M);
  let waypoints = $derived(
    (route.waypoints ?? [])
      .slice()
      .sort((a, b) => a.sortOrder - b.sortOrder)
      .map((w) => ({
        name: w.name,
        type: w.type,
        lon: w.geometry.coordinates[0],
        lat: w.geometry.coordinates[1]
      }))
  );

  // "Loop · starts Asakusa-bashi · 4.6 km from you"
  let subtitle = $derived.by(() => {
    const parts = [isLoop ? 'Loop' : 'Point to point'];
    if (!$isMobile && waypoints[0]?.name) parts.push(`starts ${waypoints[0].name}`);
    const me = $liveNavigationPosition;
    if (me && line.length > 0) {
      parts.push(`${km(metresBetween([me.longitude, me.latitude], line[0]))} from you`);
    }
    return parts.join(' · ');
  });

  let summary = $derived(summarise(conditions));
  let departure = $derived(new Date($departureAt));
  let rideSeconds = $derived.by(() => {
    if (summary?.arrival) {
      const s = (summary.arrival.getTime() - departure.getTime()) / 1000;
      if (s > 0) return s;
    }
    // Fallback: 18 km/h plus climbing time, until conditions arrive.
    return route.distanceM / 5 + (route.elevationGainM / 100) * 60 * 2;
  });
  let wind = $derived(summary ? windTile(summary) : null);
  let rain = $derived(summary ? rainTile(summary) : null);
  let surfaces = $derived.by(() => {
    const seen = new Set<string>();
    for (const s of route.segments ?? []) seen.add(s.surfaceType);
    return [...seen];
  });

  let saved = $derived($savedRoutes.some((r) => r.id === route.id));
  let more = $state(false);
  let nav = $derived($foregroundNavigation);

  function ride() {
    requestRide(() => {
      setNavigationRoute(line);
      startForegroundNavigation();
    });
  }
</script>

{#snippet stats(size: string)}
  <div class="grid grid-cols-3 gap-2">
    <div>
      <div class="{size} font-semibold text-text-strong tabular-nums">{km(route.distanceM)}</div>
      <div class="text-2xs text-text-subtle">Distance</div>
    </div>
    <div>
      <div class="{size} font-semibold text-text-strong tabular-nums">
        {climb(route.elevationGainM)}
      </div>
      <div class="text-2xs text-text-subtle">Climb</div>
    </div>
    <div>
      <div class="{size} font-semibold text-text-strong tabular-nums">{duration(rideSeconds)}</div>
      <div class="text-2xs text-text-subtle">At your pace</div>
    </div>
  </div>
{/snippet}

{#snippet tiles()}
  <div class="grid grid-cols-3 gap-2.5">
    <div class="tile">
      <span class="flex items-center gap-2">
        <Icon name="sunHorizon" class="size-6 text-shade-mostly" />
        <span class="text-sm font-semibold text-text-strong tabular-nums">
          {summary ? `${Math.round(summary.shade * 100)}%` : '—'}
        </span>
      </span>
      <span class="text-2xs text-text-subtle">{$isMobile ? 'shaded' : 'of route shaded'}</span>
    </div>
    <div class="tile">
      <span class="flex items-center gap-2">
        <Icon name="wind" class="size-6 text-wind-tail" />
        <span class="text-sm font-semibold text-text-strong"
          >{wind ? ($isMobile ? wind.value.replace('wind', '') : wind.value) : '—'}</span
        >
      </span>
      <span class="text-2xs text-text-subtle"
        >{wind
          ? $isMobile && wind.note === 'home stretch'
            ? 'way back'
            : wind.note
          : 'loading'}</span
      >
    </div>
    <div class="tile">
      <span class="flex items-center gap-2">
        <Icon name="rain" class="size-6 text-rain-moderate" />
        <span class="text-sm font-semibold text-text-strong">{rain ? rain.value : '—'}</span>
      </span>
      <span class="text-2xs text-text-subtle">{rain ? rain.note : 'loading'}</span>
    </div>
  </div>
{/snippet}

{#snippet badges()}
  <div class="flex flex-wrap items-center gap-1.5">
    <DifficultyBadge difficulty={route.difficulty} />
    {#each route.tags.slice(0, $isMobile ? 1 : 2) as tag (tag)}
      <span
        class="inline-flex items-center h-5.5 px-2 rounded-sm bg-surface-tint border border-hairline text-xs font-medium text-text"
        >{tag}</span
      >
    {/each}
  </div>
{/snippet}

{#snippet guidance()}
  {#if nav.status !== 'idle'}
    <div
      class="flex items-center justify-between gap-3 rounded-md bg-surface-elevated border border-hairline px-3 py-2"
    >
      <div class="min-w-0">
        <div class="text-13 font-medium text-text">
          {nav.status === 'requesting'
            ? 'Waiting for GPS…'
            : nav.status === 'paused'
              ? 'Guidance paused while Komorebi was in the background'
              : nav.status === 'error'
                ? 'Navigation unavailable'
                : nav.offRoute
                  ? 'Off route'
                  : 'Guidance active'}
        </div>
        <div class="text-2xs text-text-subtle">
          {#if nav.error}{nav.error}{:else if nav.remainingDistanceM !== null}{km(
              nav.remainingDistanceM
            )} remaining · GPS ±{Math.round(nav.position?.accuracy ?? 0)} m{:else}Keep Komorebi
            visible for continuous guidance{/if}
        </div>
      </div>
      <button type="button" onclick={stopForegroundNavigation} class="btn-secondary px-3 text-13"
        >Stop</button
      >
    </div>
  {/if}
{/snippet}

{#snippet details()}
  <div class="flex flex-col gap-4 px-4 pb-4 pt-3">
    {#if waypoints.length > 0}
      <div>
        <SectionLabel text="Stops" class="pb-1.5" />
        <ol class="flex flex-col gap-1">
          {#each waypoints as wp, i (wp.name + i)}
            <li class="flex items-center gap-2 text-13 text-text">
              <span
                class="inline-flex items-center justify-center size-5 rounded-pill border-2 border-route text-route text-3xs font-semibold"
                >{i + 1}</span
              >
              {wp.name}
              <span class="text-2xs text-text-subtle capitalize">{wp.type.replace('_', ' ')}</span>
            </li>
          {/each}
        </ol>
      </div>
    {/if}
    {#if surfaces.length > 0}
      <div>
        <SectionLabel text="Surface" class="pb-1.5" />
        <div class="text-13 text-text capitalize">{surfaces.join(' · ')}</div>
      </div>
    {/if}
    <ReviewList routeId={route.id} />
  </div>
{/snippet}

{#if $isMobile}
  <Sheet label={route.name} class="sheet-route">
    <div class="px-5 pt-1 pb-5 flex flex-col gap-3">
      <div>
        <h2 class="text-lg font-semibold text-text-strong leading-tight">{route.name}</h2>
        <div class="text-13 text-text-subtle mt-0.5">{subtitle}</div>
      </div>
      {@render badges()}
      {@render stats('text-17')}
      <div class="h-px bg-hairline"></div>
      <SectionLabel text="If you leave at {hhmm(departure)}" />
      {@render tiles()}
      {@render guidance()}
      {#if nav.status === 'idle'}
        <div class="flex items-center gap-2">
          <button type="button" onclick={ride} class="btn-primary btn-tall flex-1 text-sm">
            <Icon name="bicycle" class="size-6" />
            Ride this
          </button>
          <IconButton
            icon="bookmark"
            label={saved ? 'Remove from saved routes' : 'Save route'}
            size={40}
            variant="raised"
            aria-pressed={saved}
            onclick={() => toggleSavedRoute(route)}
            class={saved ? 'text-route' : ''}
          />
          <IconButton
            icon={more ? 'chevronUp' : 'chevronDown'}
            label={more ? 'Less' : 'More'}
            size={40}
            variant="raised"
            aria-expanded={more}
            onclick={() => (more = !more)}
          />
        </div>
      {/if}
      <p class="text-2xs text-text-placeholder">Shade: PLATEAU (MLIT) · Weather: Open-Meteo</p>
      {#if more}
        <button
          type="button"
          onclick={() => oncustomize(route)}
          class="btn-secondary btn-tall text-sm">Customize</button
        >
        <div class="-mx-5 border-t border-hairline">{@render details()}</div>
      {/if}
    </div>
  </Sheet>
{:else}
  <div
    class="flex flex-col bg-surface-base border border-line rounded-lg shadow-lg overflow-hidden max-h-full"
  >
    <div class="min-h-0 overflow-y-auto" data-scroll="true">
      <div class="flex items-start pl-4 pr-2 pt-2">
        <div class="flex-1 min-w-0 pt-1.5">
          <h2 class="text-17 font-semibold text-text-strong leading-tight">{route.name}</h2>
          <div class="text-xs text-text-subtle mt-0.5">{subtitle}</div>
        </div>
        <IconButton icon="x" label="Close" size={32} onclick={onclose} />
      </div>
      <div class="px-4 pt-2 flex flex-col gap-3.5">
        {@render badges()}
        {@render stats('text-lg')}
        <ElevationProfileMini coordinates={coords} {waypoints} />
        <div class="h-px bg-hairline"></div>
        <SectionLabel text="Conditions if you leave at {hhmm(departure)}" />
        {@render tiles()}
        <p class="text-2xs text-text-placeholder -mt-1">
          Shade: PLATEAU (MLIT) · Weather: Open-Meteo
        </p>
        {@render guidance()}
        {#if nav.status === 'idle'}
          <div class="flex items-center gap-2">
            <button type="button" onclick={ride} class="btn-primary w-44 text-sm">
              <Icon name="bicycle" class="size-6" />
              Ride this
            </button>
            <button
              type="button"
              onclick={() => oncustomize(route)}
              class="btn-secondary flex-1 min-w-0 text-sm">Customize</button
            >
            <IconButton
              icon="bookmark"
              label={saved ? 'Remove from saved routes' : 'Save route'}
              size={40}
              variant="raised"
              aria-pressed={saved}
              onclick={() => toggleSavedRoute(route)}
              class={saved ? 'text-route' : ''}
            />
          </div>
        {/if}
      </div>
      <div class="mt-3 border-t border-hairline">
        <button
          type="button"
          onclick={() => (more = !more)}
          aria-expanded={more}
          class="w-full flex items-center justify-between h-11 pl-4 pr-2 text-13 font-medium text-text hover:bg-surface-tint focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <span>Reviews · Stops · Surface</span>
          <Icon name={more ? 'chevronUp' : 'chevronDown'} class="size-6 text-text-subtle mr-1" />
        </button>
        {#if more}
          {@render details()}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  /* Penpot "Cond / …" tile: 116×56, surface.elevated, hairline ring, radius 8. */
  .tile {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-height: 56px;
    padding: 8px 10px 8px 10px;
    border-radius: var(--radius-md);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-hairline);
  }
  :global(.sheet-route) {
    max-height: 62dvh;
  }
</style>
