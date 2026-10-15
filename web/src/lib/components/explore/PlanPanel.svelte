<!-- web/src/lib/components/explore/PlanPanel.svelte -->
<!-- Frame G: Plan a route. Stops (From defaults to Your location, To,
     + Add stop), a Leave chip (the time chip's value) and a Preferences
     chip, then one row per alternative — Suggested, Fast, Quiet streets —
     with time, distance · climb and a conditions line. The selected row has
     a 3px route-blue bar; the map draws the selected alternative in route
     blue and the others route-muted, each with a time pill. Actions: Ride
     this, Save, Share. No route found is a normal outcome (ADR 0004). -->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { SvelteMap } from 'svelte/reactivity';
  import { replaceState } from '$app/navigation';
  import { resolve } from '$app/paths';
  import { browser } from '$app/environment';
  import { ApiError, routing } from '$lib/api/client';
  import type { RouteAlternative, RouteConditionSegment } from '$lib/api/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import PreferenceSliders from '$lib/components/PreferenceSliders.svelte';
  import PanelHeader from './PanelHeader.svelte';
  import SectionLabel from './SectionLabel.svelte';
  import Sheet from './Sheet.svelte';
  import {
    departureAt,
    highlightedRouteId,
    liveNavigationPosition,
    routeDisplays,
    routeStops,
    selectedRouteGeometry,
    selectedRouteDistanceM,
    alternativePick,
    fitToLine,
    chromePadding
  } from '$lib/stores/map';
  import { plannerPreferences } from '$lib/stores/planner';
  import { isMobile, requestRide, scrubberOpen, showToast } from '$lib/stores/ui';
  import { rememberPlace, savedPlans, toggleSavedPlan } from '$lib/stores/recents';
  import {
    foregroundNavigation,
    setNavigationRoute,
    startForegroundNavigation,
    stopForegroundNavigation
  } from '$lib/stores/navigation';
  import { km, climb, duration, hhmm, isNow } from '$lib/utils/format';
  import { summarise, conditionsLine } from '$lib/utils/conditions';

  export interface PlanStop {
    id: string;
    label: string;
    query: string;
    lat: number | null;
    lon: number | null;
  }

  interface Props {
    /** Stops to start from (Customize on a route card). */
    initialStops?: { label: string; lat: number; lon: number }[] | null;
    onback: () => void;
    onclose: () => void;
  }
  let { initialStops = null, onback, onclose }: Props = $props();

  const newStop = (label = '', lat: number | null = null, lon: number | null = null): PlanStop => ({
    id: crypto.randomUUID(),
    label,
    query: label,
    lat,
    lon
  });

  let stops = $state<PlanStop[]>([newStop(), newStop()]);

  // --- Place lookup (Nominatim) ------------------------------------------
  let activeInputIndex = $state<number | null>(null);
  let suggestions = $state<{ display_name: string; lat: string; lon: string }[]>([]);
  let highlightedSuggIdx = $state(-1);
  let searchDebounce: ReturnType<typeof setTimeout>;
  let inputRefs: HTMLInputElement[] = [];

  async function searchAddress(query: string) {
    if (query.length < 3) {
      suggestions = [];
      return;
    }
    try {
      const res = await fetch(
        `/nominatim/search?format=json&q=${encodeURIComponent(query)}&limit=5&countrycodes=jp&accept-language=en`
      );
      suggestions = res.ok ? await res.json() : [];
    } catch {
      suggestions = [];
    }
    highlightedSuggIdx = -1;
  }

  function handleInput(index: number, e: Event) {
    const val = (e.target as HTMLInputElement).value;
    stops = stops.map((s, i) => (i === index ? { ...s, query: val, lat: null, lon: null } : s));
    clearTimeout(searchDebounce);
    searchDebounce = setTimeout(() => searchAddress(val), 300);
  }

  function handleKeydown(index: number, e: KeyboardEvent) {
    if (suggestions.length === 0) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      highlightedSuggIdx = Math.min(highlightedSuggIdx + 1, suggestions.length - 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      highlightedSuggIdx = Math.max(highlightedSuggIdx - 1, 0);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (highlightedSuggIdx >= 0) selectSuggestion(index, suggestions[highlightedSuggIdx]);
    } else if (e.key === 'Escape') {
      suggestions = [];
    }
  }

  function selectSuggestion(index: number, s: { display_name: string; lat: string; lon: string }) {
    const lat = parseFloat(s.lat);
    const lon = parseFloat(s.lon);
    const parts = s.display_name.split(',').map((p) => p.trim());
    const name = parts[0];
    stops = stops.map((st, i) =>
      i === index ? { ...st, lat, lon, label: name, query: name } : st
    );
    rememberPlace({ name, detail: parts.slice(1, 3).join(' · '), lat, lon });
    suggestions = [];
    activeInputIndex = null;
    focusNextEmpty(index);
  }

  function focusNextEmpty(after: number) {
    const next = stops.findIndex((s, i) => i > after && s.lat === null);
    if (next !== -1) {
      setTimeout(() => {
        inputRefs[next]?.focus();
        activeInputIndex = next;
      }, 50);
    }
  }

  function useMyLocation(index: number) {
    const me = $liveNavigationPosition;
    if (!me) return;
    stops = stops.map((s, i) =>
      i === index
        ? {
            ...s,
            lat: me.latitude,
            lon: me.longitude,
            label: 'Your location',
            query: 'Your location'
          }
        : s
    );
  }

  /** The page forwards map taps here: they fill the active stop, else the first empty one. */
  export function handleMapClick(lat: number, lon: number) {
    const idx = activeInputIndex ?? stops.findIndex((s) => s.lat === null);
    if (idx === -1) return;
    const label = `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
    stops = stops.map((s, i) => {
      if (i !== idx) return s;
      const name = s.query.trim() && s.lat === null ? s.query : label;
      return { ...s, lat, lon, label: name, query: name };
    });
    suggestions = [];
    activeInputIndex = null;
    focusNextEmpty(idx);
  }

  function addStop() {
    stops = [...stops.slice(0, -1), newStop(), stops[stops.length - 1]];
    const idx = stops.length - 2;
    setTimeout(() => {
      inputRefs[idx]?.focus();
      activeInputIndex = idx;
    }, 50);
  }
  function removeStop(index: number) {
    if (stops.length <= 2) return;
    stops = stops.filter((_, i) => i !== index);
  }
  function swapEnds() {
    const first = stops[0];
    const last = stops[stops.length - 1];
    stops = [last, ...stops.slice(1, -1), first];
  }

  // --- Shareable URL (?from=lat,lon,label&via=…&to=…) -----------------------
  const MAX_URL_VIAS = 8;
  function stopToParam(s: PlanStop): string {
    const base = `${s.lat!.toFixed(5)},${s.lon!.toFixed(5)}`;
    const label = s.label.trim();
    return label && !/^-?\d+\.\d+,\s*-?\d+\.\d+$/.test(label) ? `${base},${label}` : base;
  }
  function parseStopParam(value: string): PlanStop | null {
    const parts = value.split(',');
    if (parts.length < 2) return null;
    const lat = Number(parts[0]);
    const lon = Number(parts[1]);
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) return null;
    if (lat < -90 || lat > 90 || lon < -180 || lon > 180) return null;
    const label = parts.slice(2).join(',').trim() || `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
    return newStop(label, lat, lon);
  }
  function planUrl(): string | null {
    const first = stops[0];
    const last = stops[stops.length - 1];
    if (first?.lat === null || last?.lat === null) return null;
    const p = new URLSearchParams();
    p.set('from', stopToParam(first));
    for (const via of stops.slice(1, -1)) if (via.lat !== null) p.append('via', stopToParam(via));
    p.set('to', stopToParam(last));
    return `${resolve('/')}?${p.toString()}`;
  }
  function syncStopsToUrl() {
    const url = planUrl();
    const target = url ?? resolve('/');
    if (`${window.location.pathname}${window.location.search}` !== target) {
      // eslint-disable-next-line svelte/no-navigation-without-resolve -- path from resolve(), plus the stop query
      replaceState(target, {});
    }
  }
  let urlSyncTimeout: ReturnType<typeof setTimeout>;
  $effect(() => {
    stops.forEach((s) => [s.lat, s.lon, s.label]);
    if (!browser) return;
    clearTimeout(urlSyncTimeout);
    urlSyncTimeout = setTimeout(syncStopsToUrl, 300);
  });
  function restoreStopsFromUrl(): boolean {
    const params = new URLSearchParams(window.location.search);
    const from = params.get('from');
    const to = params.get('to');
    if (!from || !to) return false;
    const a = parseStopParam(from);
    const b = parseStopParam(to);
    if (!a || !b) return false;
    const vias = params
      .getAll('via')
      .slice(0, MAX_URL_VIAS)
      .map(parseStopParam)
      .filter((s): s is PlanStop => s !== null);
    stops = [a, ...vias, b];
    return true;
  }

  onMount(() => {
    if (initialStops && initialStops.length >= 2) {
      stops = initialStops.map((s) => newStop(s.label, s.lat, s.lon));
    } else if (!restoreStopsFromUrl()) {
      const me = $liveNavigationPosition;
      if (me) useMyLocation(0);
      setTimeout(() => {
        const idx = me ? 1 : 0;
        inputRefs[idx]?.focus();
        activeInputIndex = idx;
      }, 50);
    }
  });

  // --- Routing ---------------------------------------------------------------
  let isRouting = $state(false);
  let alternatives = $state<RouteAlternative[]>([]);
  let selectedProfile = $state<string | null>(null);
  let routeError = $state<string | null>(null);
  const altConditions = new SvelteMap<string, RouteConditionSegment[]>();
  let conditionsSeq = 0;

  const LABELS: Record<string, string> = {
    suggested: 'Suggested',
    fast: 'Fast',
    avoid_main_roads: 'Quiet streets'
  };
  const label = (alt: RouteAlternative) => LABELS[alt.profile] ?? alt.label;

  let selectedAlt = $derived(alternatives.find((a) => a.profile === selectedProfile) ?? null);
  let resolvedStops = $derived(stops.filter((s) => s.lat !== null && s.lon !== null));
  let stopsKey = $derived(resolvedStops.map((s) => `${s.lat},${s.lon}`).join('|'));

  function publishDisplays() {
    routeDisplays.set(
      alternatives.map((alt) => ({
        profile: alt.profile,
        label: label(alt),
        coords: (alt.geometry?.coordinates ?? []).map((c) => [c[0], c[1]] as [number, number]),
        selected: alt.profile === selectedProfile,
        distanceM: alt.total_distance_km * 1000,
        durationS: alt.total_duration_s
      }))
    );
  }

  async function doRoute() {
    if (resolvedStops.length < 2) return;
    isRouting = true;
    routeError = null;
    conditionsSeq += 1;
    try {
      const res = await routing.directions({
        stops: resolvedStops.map((s) => ({ type: 'manual' as const, lat: s.lat!, lon: s.lon! })),
        departure_at: $departureAt,
        speed_model: 'elevation',
        preferences: $plannerPreferences
      });
      alternatives = res.alternatives ?? [];
      altConditions.clear();
      if (alternatives.length > 0) {
        selectAlternative(alternatives[0].profile, true);
        void fetchConditions(alternatives);
      } else {
        routeError = 'No route found between these stops.';
        clearMap();
      }
      publishDisplays();
    } catch (e) {
      alternatives = [];
      clearMap();
      const msg = e instanceof Error ? e.message : String(e);
      const api = e instanceof ApiError ? e : null;
      // 422 is a request the engine rejected (a stop off any road it knows,
      // no path between the stops) and carries the reason; only a 5xx means
      // the engine itself could not be reached.
      routeError = msg.includes('Failed to fetch')
        ? "Can't reach Komorebi — showing the map only."
        : api && api.status >= 500
          ? 'The routing engine is not running.'
          : api?.status === 422 && api.detail
            ? api.detail
            : api?.status === 422 || api?.status === 400
              ? 'No route found. Move a stop onto a public road.'
              : 'No route found between these stops.';
    } finally {
      isRouting = false;
    }
  }

  async function fetchConditions(alts: RouteAlternative[]) {
    const departure = $departureAt;
    const seq = conditionsSeq;
    await Promise.allSettled(
      alts.map(async (alt) => {
        try {
          const res = await routing.conditions(alt.geometry, alt.elevation_profile, departure);
          if (seq !== conditionsSeq) return;
          altConditions.set(alt.profile, res.segments ?? []);
        } catch {
          /* conditions are enrichment */
        }
      })
    );
  }

  function selectAlternative(profile: string, fit = false) {
    selectedProfile = profile;
    publishDisplays();
    const alt = alternatives.find((a) => a.profile === profile);
    if (!alt) return;
    const coords = (alt.geometry?.coordinates ?? []).map((c) => [c[0], c[1]] as [number, number]);
    highlightedRouteId.set(null);
    selectedRouteGeometry.set(coords);
    selectedRouteDistanceM.set(alt.total_distance_km * 1000);
    setNavigationRoute(coords);
    if (fit) fitToLine(coords, chromePadding($isMobile));
  }

  function clearMap() {
    routeDisplays.set([]);
    selectedRouteGeometry.set(null);
    selectedRouteDistanceM.set(0);
  }

  // Route whenever the resolved stops, the departure or the preferences change.
  let routeDebounce: ReturnType<typeof setTimeout>;
  $effect(() => {
    const key = stopsKey;
    const dep = $departureAt;
    const prefs = $plannerPreferences;
    void prefs;
    clearTimeout(routeDebounce);
    if (!key || resolvedStops.length < 2) {
      alternatives = [];
      clearMap();
      return;
    }
    routeDebounce = setTimeout(() => {
      void dep;
      void doRoute();
    }, 350);
  });

  // A tap on an alternative's line or time pill selects it.
  $effect(() => {
    const pick = $alternativePick;
    if (pick && pick !== selectedProfile && alternatives.some((a) => a.profile === pick)) {
      selectAlternative(pick);
    }
    if (pick) alternativePick.set(null);
  });

  // Markers for the stops (start, numbered vias, end).
  $effect(() => {
    let via = 0;
    routeStops.set(
      stops.flatMap((s, i) => {
        if (s.lat === null || s.lon === null) return [];
        const kind = i === 0 ? 'start' : i === stops.length - 1 ? 'end' : 'via';
        return [{ lat: s.lat, lon: s.lon, kind, index: kind === 'via' ? ++via : 0 }];
      })
    );
  });

  $effect(() => {
    liveNavigationPosition.set($foregroundNavigation.position ?? $liveNavigationPosition);
  });

  onDestroy(() => {
    clearTimeout(routeDebounce);
    clearTimeout(urlSyncTimeout);
    clearMap();
    routeStops.set([]);
    if (browser) {
      const clean = resolve('/');
      if (window.location.search) replaceState(clean, {});
    }
  });

  // --- Chips, actions ----------------------------------------------------------
  let leaveLabel = $derived(
    isNow($departureAt)
      ? `Leave now · ${hhmm(new Date($departureAt))}`
      : `Leave ${hhmm(new Date($departureAt))}`
  );
  let prefsLabel = $derived.by(() => {
    const p = $plannerPreferences;
    const wants = [
      p.shade > 0.6 && 'shade',
      p.greenery > 0.6 && 'greenery',
      p.wind > 0.6 && 'tailwind'
    ].filter(Boolean);
    return wants.length ? `Prefer ${wants.join(', ')}` : 'Preferences';
  });
  let prefsOpen = $state(false);

  let nav = $derived($foregroundNavigation);
  function ride() {
    if (!selectedAlt) return;
    requestRide(startForegroundNavigation);
  }
  let saved = $derived.by(() => {
    const url = planUrl();
    return url !== null && $savedPlans.some((p) => p.url === url);
  });
  function save() {
    const url = planUrl();
    if (!url || !selectedAlt) return;
    const name = `${stops[0].label || 'Start'} → ${stops[stops.length - 1].label || 'Destination'}`;
    toggleSavedPlan({ url, name, distanceM: selectedAlt.total_distance_km * 1000 });
    showToast(saved ? 'Removed from saved plans' : 'Plan saved on this device');
  }
  async function share() {
    const url = planUrl();
    if (!url) return;
    const href = new URL(url, window.location.origin).href;
    try {
      if (navigator.share) {
        await navigator.share({ title: 'Komorebi ride plan', url: href });
        return;
      }
      await navigator.clipboard.writeText(href);
      showToast('Link copied');
    } catch {
      showToast('Could not share the link');
    }
  }
</script>

{#snippet stopRow(stop: PlanStop, i: number)}
  {@const first = i === 0}
  {@const last = i === stops.length - 1}
  <div class="relative flex items-center gap-2.5 min-h-12">
    <span class="w-5 shrink-0 flex justify-center" aria-hidden="true">
      {#if first}
        <span class="marker marker-start"><Icon name="bicycle" stroke={2.2} class="size-3" /></span>
      {:else if last}
        <span class="marker marker-end"><Icon name="flag" stroke={2.2} class="size-3" /></span>
      {:else}
        <span class="marker marker-via">{i}</span>
      {/if}
    </span>
    <div class="relative flex-1 min-w-0">
      <input
        bind:this={inputRefs[i]}
        type="text"
        placeholder={first ? 'Your location' : last ? 'Destination' : 'Stop'}
        aria-label={first ? 'From' : last ? 'To' : `Stop ${i}`}
        value={stop.query}
        onfocus={() => (activeInputIndex = i)}
        onblur={() => setTimeout(() => (suggestions = []), 200)}
        oninput={(e) => handleInput(i, e)}
        onkeydown={(e) => handleKeydown(i, e)}
        class="input-field"
      />
      {#if activeInputIndex === i && suggestions.length > 0}
        <ul
          role="listbox"
          aria-label="Place suggestions"
          class="absolute left-0 right-0 top-full mt-1 z-50 overflow-hidden rounded-md bg-surface-base border border-line shadow-lg"
        >
          {#each suggestions as s, si (s.display_name)}
            <li>
              <button
                type="button"
                role="option"
                aria-selected={si === highlightedSuggIdx}
                onmousedown={() => selectSuggestion(i, s)}
                onmouseenter={() => (highlightedSuggIdx = si)}
                class="w-full min-h-11 text-left px-3 py-2 text-13 border-b border-hairline last:border-b-0
                       {si === highlightedSuggIdx
                  ? 'bg-surface-tint text-text'
                  : 'text-text-subtle hover:bg-surface-tint'}"
              >
                {s.display_name.split(',').slice(0, 3).join(',')}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
      {#if first && stop.lat === null && $liveNavigationPosition}
        <button
          type="button"
          onclick={() => useMyLocation(0)}
          class="absolute right-1 top-1/2 -translate-y-1/2 h-8 px-2 rounded-sm text-xs font-medium text-text-subtle hover:bg-surface-tint"
          >Use my location</button
        >
      {/if}
    </div>
    {#if first || last}
      <IconButton
        icon="swap"
        label="Swap start and destination"
        size={36}
        stroke={1.6}
        onclick={swapEnds}
        class="text-text-placeholder"
      />
    {:else}
      <IconButton
        icon="x"
        label="Remove stop {i}"
        size={36}
        stroke={1.6}
        onclick={() => removeStop(i)}
        class="text-text-placeholder"
      />
    {/if}
  </div>
{/snippet}

{#snippet body()}
  <div class="px-2 pt-1">
    <div class="flex flex-col">
      {#each stops as stop, i (stop.id)}
        {#if i > 0}
          <div class="flex items-center h-2 -my-1" aria-hidden="true">
            <span class="w-5 flex justify-center"><span class="connector"></span></span>
          </div>
        {/if}
        {@render stopRow(stop, i)}
      {/each}
    </div>
    <div class="flex items-center gap-2 pl-8 pt-1">
      <button type="button" onclick={addStop} class="btn-ghost -ml-2 pl-1 pr-3 text-13">
        <Icon name="plus" stroke={1.6} class="size-6" />
        Add stop
      </button>
      <span class="text-xs text-text-placeholder">or tap the map to drop one</span>
    </div>
    <div class="relative flex flex-wrap gap-2 px-2 pt-2 pb-3">
      <button type="button" onclick={() => scrubberOpen.set(true)} class="chip-btn">
        <span class="chip-face">
          {leaveLabel}
          <Icon name="chevronDown" stroke={1.6} class="size-6 text-text-subtle" />
        </span>
      </button>
      <button
        type="button"
        onclick={() => (prefsOpen = !prefsOpen)}
        aria-expanded={prefsOpen}
        class="chip-btn"
      >
        <span class="chip-face">
          {prefsLabel}
          <Icon name="chevronDown" stroke={1.6} class="size-6 text-text-subtle" />
        </span>
      </button>
      {#if prefsOpen}
        <div class="basis-full rounded-md border border-line bg-surface-base shadow-lg p-4">
          <PreferenceSliders />
          <p class="pt-2 text-2xs text-text-placeholder">
            Saved on this device. No account needed.
          </p>
        </div>
      {/if}
    </div>
  </div>
  <div class="h-px bg-hairline"></div>
  <SectionLabel text="Routes" class="px-4 pt-3 pb-1.5" />
  <div class="px-2 pb-1 flex flex-col gap-1.5" role="radiogroup" aria-label="Route alternatives">
    {#if resolvedStops.length < 2}
      <p class="px-2 py-3 text-13 text-text-subtle">Add a start and a destination to see routes.</p>
    {:else if isRouting && alternatives.length === 0}
      <p class="px-2 py-3 text-13 text-text-subtle">Finding routes…</p>
    {:else if routeError}
      <p class="px-2 py-3 text-13 text-text">{routeError}</p>
    {:else}
      {#each alternatives as alt (alt.profile)}
        {@const on = alt.profile === selectedProfile}
        {@const line = conditionsLine(summarise(altConditions.get(alt.profile) ?? []))}
        <button
          type="button"
          role="radio"
          aria-checked={on}
          onclick={() => selectAlternative(alt.profile)}
          class="alt-row {on ? 'alt-on' : ''}"
        >
          <span class="flex-1 min-w-0">
            <span class="block text-sm font-semibold text-text-strong">{label(alt)}</span>
            <span class="block text-xs text-text-subtle pt-1"
              >{km(alt.total_distance_km * 1000)} · {climb(alt.elevation_gain_m ?? 0)}</span
            >
            <span class="block text-xs text-text pt-0.5">{line ?? 'Conditions loading…'}</span>
          </span>
          <span
            class="text-base font-semibold tabular-nums {on ? 'text-route' : 'text-text-strong'}"
            >{duration(alt.total_duration_s)}</span
          >
        </button>
      {/each}
    {/if}
  </div>
  {#if alternatives.length > 0}
    <p class="px-4 pt-1 pb-3 text-2xs text-text-placeholder">
      If you leave at {hhmm(new Date($departureAt))} · Shade: PLATEAU (MLIT) · Weather: Open-Meteo
    </p>
  {/if}
  <div class="h-px bg-hairline"></div>
  <div class="px-4 pt-3 pb-3">
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
    {:else}
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={ride}
          disabled={!selectedAlt}
          class="btn-primary flex-1 text-13">Ride this</button
        >
        <button
          type="button"
          onclick={save}
          disabled={!selectedAlt}
          aria-pressed={saved}
          class="btn-secondary w-29 text-13 disabled:opacity-50">{saved ? 'Saved' : 'Save'}</button
        >
        <IconButton
          icon="share"
          label="Share this plan"
          size={40}
          stroke={1.6}
          variant="raised"
          onclick={share}
          disabled={!planUrl()}
          class="disabled:opacity-50"
        />
      </div>
    {/if}
    <p class="pt-3 text-2xs text-text-placeholder">
      Stops are kept in the link — share it to send this plan.
    </p>
  </div>
{/snippet}

{#if $isMobile}
  <Sheet label="Plan a route" role="dialog" class="sheet-plan">
    <PanelHeader title="Plan a route" {onback} {onclose} />
    {@render body()}
  </Sheet>
{:else}
  <div
    class="flex flex-col bg-surface-base border border-line rounded-lg shadow-lg max-h-full overflow-hidden"
  >
    <PanelHeader title="Plan a route" {onback} {onclose} />
    <div class="min-h-0 overflow-y-auto" data-scroll="true">
      {@render body()}
    </div>
  </div>
{/if}

<style>
  /* Stop glyphs (Penpot "Marker / start | stop | end" at 20 / 18 px). */
  .marker {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border-radius: 999px;
    box-sizing: border-box;
    font-size: 10px;
    font-weight: 600;
    line-height: 1;
  }
  .marker-start {
    background: var(--color-surface-base);
    border: 2.5px solid var(--color-route);
    color: var(--color-route);
  }
  .marker-via {
    width: 18px;
    height: 18px;
    background: var(--color-surface-base);
    border: 2px solid var(--color-route);
    color: var(--color-route);
  }
  .marker-end {
    background: var(--color-route);
    border: 2px solid var(--color-route-casing);
    color: var(--color-on-primary);
    box-shadow: 0 0 0 1px var(--color-route);
  }
  /* Dotted connector between glyphs (2×3 dots, line-strong). */
  .connector {
    display: block;
    width: 2px;
    height: 12px;
    background: repeating-linear-gradient(
      to bottom,
      var(--color-line-strong) 0 3px,
      transparent 3px 8px
    );
    border-radius: 1px;
  }
  /* Penpot "Chip / Leave now · 10:00": a 32px pill, 13px medium, inside a
     44px-tall transparent button. */
  .chip-btn {
    display: inline-flex;
    align-items: center;
    height: 44px;
    padding: 0;
    background: transparent;
  }
  .chip-face {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    height: 32px;
    padding: 0 6px 0 12px;
    border-radius: 999px;
    border: 1px solid var(--color-line);
    background: var(--color-surface-base);
    font-size: var(--text-13);
    font-weight: 500;
    color: var(--color-text-strong);
  }
  .chip-btn:hover .chip-face {
    background: var(--color-surface-tint);
  }
  .chip-btn:focus-visible {
    outline: none;
  }
  .chip-btn:focus-visible .chip-face {
    box-shadow: 0 0 0 2px var(--color-focus);
  }
  /* Penpot "AltRow": 70px, radius 8; selected = elevated surface, hairline
     ring and a 3px route-blue bar. */
  .alt-row {
    position: relative;
    display: flex;
    align-items: flex-start;
    gap: 12px;
    width: 100%;
    min-height: 70px;
    padding: 10px 14px;
    border-radius: var(--radius-md);
    border: 1px solid transparent;
    text-align: left;
    background: transparent;
  }
  .alt-row:hover {
    background: var(--color-surface-tint);
  }
  .alt-row:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }
  .alt-on {
    background: var(--color-surface-elevated);
    border-color: var(--color-hairline);
  }
  .alt-on::before {
    content: '';
    position: absolute;
    left: 0;
    top: 12px;
    bottom: 12px;
    width: 3px;
    border-radius: 2px;
    background: var(--color-route);
  }
  :global(.sheet-plan) {
    height: 72dvh;
  }
</style>
