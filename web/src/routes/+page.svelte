<!-- web/src/routes/+page.svelte -->
<!-- Home / discover (Penpot frames A–D, G, H, I). The map is quiet until the
     rider asks something of it: search pill top-left, Layers top-right, zoom
     and locate bottom-right, the routes-in-view peek bottom-left and the
     time chip bottom-centre. One floating panel at a time on desktop (left,
     400px); one sheet at a time on mobile. -->
<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import Map from '$lib/components/Map.svelte';
  import AttributionPill from '$lib/components/AttributionPill.svelte';
  import DataSourcesSheet from '$lib/components/DataSourcesSheet.svelte';
  import SearchPill from '$lib/components/explore/SearchPill.svelte';
  import SearchPanel, { type PlaceHit } from '$lib/components/explore/SearchPanel.svelte';
  import MenuPopover from '$lib/components/explore/MenuPopover.svelte';
  import MapControls from '$lib/components/explore/MapControls.svelte';
  import LayersPanel from '$lib/components/explore/LayersPanel.svelte';
  import TimeChip from '$lib/components/explore/TimeChip.svelte';
  import TimeScrubber from '$lib/components/explore/TimeScrubber.svelte';
  import Legend from '$lib/components/explore/Legend.svelte';
  import RoutesPeek from '$lib/components/explore/RoutesPeek.svelte';
  import RoutesListPanel from '$lib/components/explore/RoutesListPanel.svelte';
  import RoutePanel from '$lib/components/explore/RoutePanel.svelte';
  import PlanPanel from '$lib/components/explore/PlanPanel.svelte';
  import DescribePanel from '$lib/components/explore/DescribePanel.svelte';
  import {
    highlightedRouteId,
    departureAt,
    mapInstance,
    mapCentre,
    bboxString,
    visibleLayers,
    activeOverlay,
    overlayForLayers,
    liveNavigationPosition,
    fitToLine,
    chromePadding
  } from '$lib/stores/map';
  import {
    explorePanel,
    layersOpen,
    scrubberOpen,
    menuOpen,
    isMobile,
    describeAvailable,
    toast
  } from '$lib/stores/ui';
  import {
    routesInView,
    routeDetails,
    routeConditions,
    refreshRoutesInView,
    ensureDetail,
    ensureConditions
  } from '$lib/stores/routesInView';
  import { refreshHourly } from '$lib/stores/weather';
  import { foregroundNavigation } from '$lib/stores/navigation';
  import { summarise, windChipPhrase } from '$lib/utils/conditions';
  import { metresBetween, LOOP_M } from '$lib/utils/mapMarkers';
  import type { Route } from '$lib/api/types';

  let { data }: { data: PageData } = $props();

  onMount(() => {
    if (data.routes.length > 0 && $routesInView.length === 0) routesInView.set(data.routes);
  });

  // --- Search field and panels ------------------------------------------------
  let query = $state('');
  let places = $state<PlaceHit[]>([]);
  let placesLoading = $state(false);
  let placesTimer: ReturnType<typeof setTimeout>;
  let placesSeq = 0;
  let searchPill: SearchPill;
  let planPanel = $state<PlanPanel | null>(null);
  let planStops = $state<{ label: string; lat: number; lon: number }[] | null>(null);
  let describeText = $state('');

  function onQueryInput(value: string) {
    clearTimeout(placesTimer);
    const q = value.trim();
    if (q.length < 3) {
      places = [];
      placesLoading = false;
      return;
    }
    placesLoading = true;
    placesTimer = setTimeout(async () => {
      const seq = ++placesSeq;
      try {
        const res = await fetch(
          `/nominatim/search?format=json&q=${encodeURIComponent(q)}&limit=5&countrycodes=jp&accept-language=en`
        );
        const hits: { display_name: string; lat: string; lon: string }[] = res.ok
          ? await res.json()
          : [];
        if (seq !== placesSeq) return;
        places = hits.map((h) => {
          const parts = h.display_name.split(',').map((p) => p.trim());
          return {
            name: parts[0],
            detail: parts.slice(1, 3).join(' · '),
            lat: Number(h.lat),
            lon: Number(h.lon)
          };
        });
      } catch {
        if (seq === placesSeq) places = [];
      } finally {
        if (seq === placesSeq) placesLoading = false;
      }
    }, 300);
  }

  function openSearch() {
    highlightedRouteId.set(null);
    explorePanel.set('search');
  }
  // Keyboard focus alone (Tab) never opens the search panel: it would tear
  // down whatever the rider is in and hide the chrome behind the field. A
  // click, a tap or typing does.
  function closePanel() {
    explorePanel.set('none');
    query = '';
    places = [];
    planStops = null;
  }
  function openPlan(stops: { label: string; lat: number; lon: number }[] | null = null) {
    planStops = stops;
    highlightedRouteId.set(null);
    explorePanel.set('plan');
  }
  function openDescribe() {
    describeText = query.trim();
    explorePanel.set('describe');
  }
  function submitSearch() {
    // Enter with a sentence describes a ride; with a place name it takes the first match.
    if (query.trim().split(/\s+/).length > 3 && $describeAvailable) openDescribe();
    else if (places[0]) goToPlace(places[0]);
  }
  function goToPlace(place: PlaceHit) {
    $mapInstance?.flyTo({ center: [place.lon, place.lat], zoom: 14, duration: 800 });
    query = place.name;
    places = [];
    explorePanel.set('none');
    searchPill?.blur();
  }
  function selectRoute(id: string) {
    explorePanel.set('none');
    highlightedRouteId.set(id);
  }
  // Customize: the route's ends become From / To and its waypoints the
  // stops, except those that sit on an end (a loop's start marker).
  function customize(route: Route) {
    const coords = route.geometry.coordinates;
    if (coords.length < 2) return;
    const start: [number, number] = [coords[0][0], coords[0][1]];
    const end: [number, number] = [coords[coords.length - 1][0], coords[coords.length - 1][1]];
    const wps = (route.waypoints ?? [])
      .slice()
      .sort((a, b) => a.sortOrder - b.sortOrder)
      .map((w) => ({
        name: w.name,
        lon: w.geometry.coordinates[0],
        lat: w.geometry.coordinates[1]
      }));
    const near = (w: { lon: number; lat: number }, p: [number, number]) =>
      metresBetween([w.lon, w.lat], p) < LOOP_M;
    const startName = wps.find((w) => near(w, start))?.name ?? route.name;
    const endName =
      wps.find((w) => near(w, end))?.name ??
      (metresBetween(start, end) < LOOP_M ? startName : 'Finish');
    const vias = wps.filter((w) => !near(w, start) && !near(w, end)).slice(0, 8);
    openPlan([
      { label: startName, lat: start[1], lon: start[0] },
      ...vias.map((w) => ({ label: w.name, lat: w.lat, lon: w.lon })),
      { label: endName, lat: end[1], lon: end[0] }
    ]);
  }

  // A stop URL (?from=…&to=…) opens Plan a route straight away.
  onMount(() => {
    const p = new URLSearchParams(window.location.search);
    if (p.get('from') && p.get('to')) explorePanel.set('plan');
  });

  // --- Selected route (frame D) -------------------------------------------------
  let selectedRoute = $derived.by(() => {
    const id = $highlightedRouteId;
    if (!id) return null;
    return $routeDetails.get(id) ?? $routesInView.find((r) => r.id === id) ?? null;
  });
  let selectedConditions = $derived(
    $highlightedRouteId ? ($routeConditions.get($highlightedRouteId) ?? []) : []
  );
  let selectedGeometry = $derived.by(() => {
    const r = selectedRoute;
    if (!r || r.geometry.coordinates.length === 0) return null;
    return r.geometry.coordinates.map((c) => [c[0], c[1]] as [number, number]);
  });
  let selectedWaypoints = $derived(
    (selectedRoute?.waypoints ?? [])
      .slice()
      .sort((a, b) => a.sortOrder - b.sortOrder)
      .map((w) => [w.geometry.coordinates[0], w.geometry.coordinates[1]] as [number, number])
  );
  let routeWind = $derived(windChipPhrase(summarise(selectedConditions)));

  // Selecting a route loads its geometry, frames it and fetches conditions;
  // the conditions follow the departure time.
  let framedFor = '';
  $effect(() => {
    const id = $highlightedRouteId;
    const dep = $departureAt;
    if (!id) {
      framedFor = '';
      return;
    }
    void ensureDetail(id).then((r) => {
      if (r && framedFor !== id && $highlightedRouteId === id) {
        framedFor = id;
        fitToLine(
          r.geometry.coordinates.map((c) => [c[0], c[1]] as [number, number]),
          chromePadding($isMobile)
        );
      }
    });
    void ensureConditions(id, dep);
  });

  // --- Map-driven data --------------------------------------------------------------
  let viewportTimer: ReturnType<typeof setTimeout>;
  $effect(() => {
    const bbox = $bboxString;
    if (!bbox) return;
    clearTimeout(viewportTimer);
    viewportTimer = setTimeout(() => void refreshRoutesInView(bbox), 400);
  });
  let weatherTimer: ReturnType<typeof setTimeout>;
  $effect(() => {
    const c = $mapCentre;
    if (!c) return;
    clearTimeout(weatherTimer);
    weatherTimer = setTimeout(() => void refreshHourly(c), 1500);
  });
  $effect(() => {
    activeOverlay.set(overlayForLayers($visibleLayers));
  });
  $effect(() => {
    const pos = $foregroundNavigation.position;
    if (pos) liveNavigationPosition.set(pos);
  });

  // Highlighting a route while a plan is on the map ends the plan.
  $effect(() => {
    if ($highlightedRouteId && $explorePanel === 'plan') explorePanel.set('none');
  });

  function handleMapClick(detail: { lng: number; lat: number }) {
    if ($explorePanel === 'plan') planPanel?.handleMapClick(detail.lat, detail.lng);
    else if ($menuOpen) menuOpen.set(false);
    else if ($layersOpen) layersOpen.set(false);
    // The mobile route sheet has no close button (Penpot D mobile): a tap on
    // the map puts it away.
    else if ($isMobile && $highlightedRouteId) highlightedRouteId.set(null);
  }

  // Escape closes the topmost thing: menu, layers, scrubber, then the panel or route.
  function handleKeydown(e: KeyboardEvent) {
    if (e.key !== 'Escape') return;
    if ($menuOpen) return menuOpen.set(false);
    if ($layersOpen) return layersOpen.set(false);
    if ($scrubberOpen) return scrubberOpen.set(false);
    if ($highlightedRouteId) return highlightedRouteId.set(null);
    if ($explorePanel !== 'none') closePanel();
  }

  let shadeVisible = $derived($visibleLayers.has('shadows'));
  let weatherVisible = $derived(
    $visibleLayers.has('rain-cells') ||
      $visibleLayers.has('wind') ||
      ($activeOverlay !== null && selectedConditions.length > 0)
  );
  // Mobile: the resting sheet is 140px; panels are taller. The attribution
  // and the docked scrubber sit just above whichever sheet is open.
  let sheetOffset = $derived(
    !$isMobile
      ? 0
      : $layersOpen
        ? 'sheet-layers'
        : $highlightedRouteId
          ? 'sheet-route'
          : $explorePanel === 'none'
            ? 'sheet-peek'
            : 'sheet-tall'
  );
</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
  <title>Komorebi — Discover Routes</title>
  <meta
    name="description"
    content="Discover cycling routes with shade, wind, and rain forecasts."
  />
</svelte:head>

<div class="relative h-full w-full overflow-hidden bg-surface-canvas">
  <Map
    attribution={false}
    highlightGeometry={selectedGeometry}
    highlightWaypoints={selectedWaypoints}
    conditionSegments={selectedConditions}
    conditionRouteDistanceM={selectedRoute?.distanceM ?? 0}
    onclick={handleMapClick}
  />

  <!-- Search pill (frames A–D) and, on desktop, the panel under it. -->
  <div
    class="absolute z-30 flex flex-col gap-2 pointer-events-none
           {$isMobile ? 'left-3 right-3 top-13.5' : 'left-4 top-4 bottom-4 w-100'}
           {$isMobile && $explorePanel === 'search' ? 'inset-0 bg-surface-base' : ''}"
  >
    <div class="pointer-events-auto {$isMobile && $explorePanel === 'search' ? 'px-3 pt-13' : ''}">
      <SearchPill
        bind:this={searchPill}
        bind:value={query}
        placeholder={$isMobile ? 'Search or describe a ride' : 'Search places, or describe a ride'}
        onclick={openSearch}
        oninput={(v) => {
          if ($explorePanel !== 'search') openSearch();
          onQueryInput(v);
        }}
        onsubmit={submitSearch}
        oncancel={closePanel}
      />
      <div class="relative">
        <div class="absolute right-0 top-2 z-40 w-56">
          <MenuPopover />
        </div>
      </div>
    </div>

    {#if $explorePanel === 'search'}
      <div
        class="pointer-events-auto min-h-0 {$isMobile
          ? 'flex-1 overflow-y-auto'
          : 'bg-surface-base border border-line rounded-lg shadow-lg overflow-y-auto'}"
        data-scroll="true"
        role="region"
        aria-label="Search"
      >
        <SearchPanel
          {query}
          {places}
          {placesLoading}
          describeAvailable={$describeAvailable}
          onplan={() => openPlan()}
          ondescribe={openDescribe}
          onplace={goToPlace}
          onsavedroute={selectRoute}
        />
      </div>
    {/if}

    {#if !$isMobile}
      {#if $highlightedRouteId && selectedRoute}
        <div class="pointer-events-auto min-h-0">
          <RoutePanel
            route={selectedRoute}
            conditions={selectedConditions}
            onclose={() => highlightedRouteId.set(null)}
            oncustomize={customize}
          />
        </div>
      {:else if $explorePanel === 'plan'}
        <div class="pointer-events-auto min-h-0">
          <PlanPanel
            bind:this={planPanel}
            initialStops={planStops}
            onback={openSearch}
            onclose={closePanel}
          />
        </div>
      {:else if $explorePanel === 'describe'}
        <div class="pointer-events-auto min-h-0">
          <DescribePanel
            initialText={describeText}
            onback={openSearch}
            onclose={closePanel}
            onplan={() => openPlan()}
          />
        </div>
      {:else if $explorePanel === 'routes'}
        <div class="pointer-events-auto min-h-0">
          <RoutesListPanel onselect={selectRoute} onclose={closePanel} />
        </div>
      {/if}
    {/if}
  </div>

  <MapControls />
  <LayersPanel />

  {#if $isMobile}
    <!-- One sheet at a time. -->
    {#if $highlightedRouteId && selectedRoute}
      <RoutePanel
        route={selectedRoute}
        conditions={selectedConditions}
        onclose={() => highlightedRouteId.set(null)}
        oncustomize={customize}
      />
    {:else if $explorePanel === 'plan'}
      <PlanPanel
        bind:this={planPanel}
        initialStops={planStops}
        onback={openSearch}
        onclose={closePanel}
      />
    {:else if $explorePanel === 'describe'}
      <DescribePanel
        initialText={describeText}
        onback={openSearch}
        onclose={closePanel}
        onplan={() => openPlan()}
      />
    {:else if $explorePanel === 'routes'}
      <RoutesListPanel onselect={selectRoute} onclose={closePanel} />
    {:else if $explorePanel === 'none' && !$layersOpen}
      <RoutesPeek onopen={() => explorePanel.set('routes')} {routeWind} />
    {/if}

    {#if $scrubberOpen}
      <div class="absolute left-3 right-3 z-30 dock-{sheetOffset}">
        <TimeScrubber />
      </div>
    {/if}
    {#if $explorePanel !== 'search'}
      <div class="absolute left-1.5 z-20 attr-{sheetOffset}">
        <AttributionPill shade={shadeVisible} weather={weatherVisible} />
      </div>
    {/if}
  {:else}
    <!-- Bottom row: peek (left) · time chip (centre) · attribution (right). -->
    <div class="absolute left-4 bottom-4 z-20 flex flex-col items-start gap-2">
      {#if $scrubberOpen}
        <Legend />
        <div class="w-180 scrubber-dock">
          <TimeScrubber />
        </div>
      {:else if $explorePanel === 'none' && !$highlightedRouteId}
        <RoutesPeek onopen={() => explorePanel.set('routes')} />
      {/if}
    </div>
    {#if !$scrubberOpen}
      <div class="absolute left-1/2 -translate-x-1/2 bottom-2 z-20">
        <TimeChip {routeWind} />
      </div>
    {/if}
    <div class="absolute right-2.5 bottom-1.5 z-20">
      <AttributionPill shade={shadeVisible} weather={weatherVisible} />
    </div>
  {/if}

  <DataSourcesSheet />

  {#if $toast}
    <div
      role="status"
      class="absolute left-1/2 -translate-x-1/2 z-50 px-3.5 h-10 inline-flex items-center rounded-pill bg-primary text-on-primary text-13 font-medium shadow-lg
             {$isMobile ? 'top-3' : 'bottom-16'}"
    >
      {$toast}
    </div>
  {/if}
</div>

<style>
  /* Mobile: things docked above the current sheet. The peek sheet is 140px
     (frame A mobile); the route sheet 62dvh; other sheets 72dvh. */
  .attr-sheet-peek {
    bottom: 148px;
  }
  .attr-sheet-route {
    bottom: calc(62dvh + 8px);
  }
  .attr-sheet-tall {
    bottom: calc(72dvh + 8px);
  }
  .attr-sheet-layers {
    bottom: calc(60dvh + 8px);
  }
  .dock-sheet-layers {
    bottom: calc(60dvh + 12px);
  }
  .dock-sheet-peek {
    bottom: 152px;
  }
  .dock-sheet-route {
    bottom: calc(62dvh + 12px);
  }
  .dock-sheet-tall {
    bottom: calc(72dvh + 12px);
  }
  .scrubber-dock {
    max-width: calc(100vw - 2rem);
  }
</style>
