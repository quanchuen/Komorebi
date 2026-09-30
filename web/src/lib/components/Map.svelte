<!-- web/src/lib/components/Map.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import maplibregl from 'maplibre-gl';
  import 'maplibre-gl/dist/maplibre-gl.css';
  import { env } from '$env/dynamic/public';
  import {
    mapInstance,
    mapBounds,
    activeOverlay,
    visibleLayers,
    routeDisplays,
    selectedRouteGeometry,
    liveNavigationPosition,
    departureAt,
    shadowSlice,
    bboxString,
    visibleVenueTypes,
    routeStops,
    VENUE_TYPE_CATEGORIES,
    type VenueType
  } from '$lib/stores/map';
  import { weather } from '$lib/api/client';
  import {
    buildRouteMarker,
    buildSelectedVenueMarker,
    markerSizeForZoom,
    metresBetween,
    metresToPolyline,
    renderVenuePinImage,
    type MarkerSize
  } from '$lib/utils/mapMarkers';
  import {
    buildLineGradient,
    ROUTE,
    ROUTE_MUTED,
    ROUTE_CASING,
    SHADE_RAMP,
    RAIN_RAMP,
    RAIN_STEPS_MMH,
    CYCLING_INFRA,
    GREENERY_FILL
  } from '$lib/utils/conditionColors';
  import type { RouteConditionSegment } from '$lib/api/types';
  import AttributionPill from './AttributionPill.svelte';
  import DataSourcesSheet from './DataSourcesSheet.svelte';

  import type { RouteAlternative } from '$lib/api/types';
  import type { FeatureCollection } from 'geojson';

  interface RouteDisplay {
    coords: [number, number][];
    selected: boolean;
    profile: string;
    color: string;
  }

  interface Props {
    interactive?: boolean;
    showControls?: boolean;
    initialCenter?: [number, number];
    initialZoom?: number;
    conditionSegments?: RouteConditionSegment[];
    conditionRouteDistanceM?: number;
    highlightGeometry?: [number, number][] | null;
    /** Numbered stops along the highlighted route, in order. */
    highlightWaypoints?: [number, number][];
    routeAlternatives?: RouteDisplay[];
    onclick?: (detail: { lng: number; lat: number }) => void;
    onmoveend?: (detail: { bounds: maplibregl.LngLatBounds }) => void;
  }

  let {
    interactive = true,
    showControls = true,
    initialCenter = [139.6917, 35.6895] as [number, number],
    initialZoom = 12,
    conditionSegments = [],
    conditionRouteDistanceM = 0,
    highlightGeometry = null,
    highlightWaypoints = [],
    routeAlternatives = [],
    onclick,
    onmoveend
  }: Props = $props();

  // Venue pin colours for the rasterised symbol icons: surface.base, line and
  // text.default from app.css. Hex is allowed here (MapLibre paint).
  const VENUE_PIN_COLORS = { fill: '#FFFFFF', ring: '#E4E4E7', icon: '#18181B' };
  const VENUE_TYPES: VenueType[] = ['konbini', 'cafe', 'water', 'repair'];
  /** Venues within this distance of the selected route stay fully opaque. */
  const ON_ROUTE_M = 150;
  /** Start and end closer than this are one loop marker. */
  const LOOP_M = 50;

  let container: HTMLDivElement;
  let map: maplibregl.Map;
  let tileError = $state<string | null>(null);
  let mapLoaded = $state(false);

  const MARTIN_URL = 'http://localhost:3000';

  // CARTO basemaps require a key since August 2026 (free: 5M tiles/month
  // non-commercial, 1M commercial; carto.com/basemaps/apikey). Read at
  // runtime so the web container needs no rebuild; without a key the tiles
  // still load, watermarked "API KEY REQUIRED".
  const cartoKey = env.PUBLIC_CARTO_API_KEY?.trim();
  const cartoTiles = ['a', 'b', 'c'].map(
    (s) =>
      `https://${s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}@2x.png${cartoKey ? `?key=${cartoKey}` : ''}`
  );

  // Shadow tiles are sliced per (hour, month); the slider swaps the tile URL.
  // MapLibre needs absolute tile URLs, so resolve the /tiles proxy against the
  // page origin (the map only exists in the browser).
  function shadowTileUrls(slice: { hourSlot: number; month: number }): string[] {
    return [
      `${window.location.origin}/tiles/shadow_cells/{z}/{x}/{y}?hour_slot=${slice.hourSlot}&month=${slice.month}`
    ];
  }

  // Optional layers: defined here but hidden by default
  const OPTIONAL_LAYERS = {
    'cycling-roads': {
      id: 'cycling-roads',
      type: 'line' as const,
      source: 'martin-roads',
      'source-layer': 'osm_roads',
      filter: ['in', 'highway', 'cycleway', 'path', 'track'],
      // Neutral zinc so infrastructure never competes with route blue or the
      // greenery ramp (spec: "Cycling infrastructure layer").
      paint: {
        'line-color': CYCLING_INFRA,
        'line-width': ['interpolate', ['linear'], ['zoom'], 10, 0.5, 16, 2],
        'line-opacity': 0.8
      },
      layout: { visibility: 'none' as const }
    },
    landuse: {
      id: 'landuse-fill',
      type: 'fill' as const,
      source: 'martin-landuse',
      'source-layer': 'osm_landuse',
      paint: {
        'fill-color': GREENERY_FILL,
        'fill-opacity': 0.15
      },
      layout: { visibility: 'none' as const }
    }
  };

  // Venue pins are a symbol layer over rasterised SVG icons (one per type),
  // added once the images are ready. Opt-in: the filter is empty until the
  // rider picks a type.
  async function addVenuePins() {
    await Promise.all(
      VENUE_TYPES.map(async (type) => {
        const image = await renderVenuePinImage(type, VENUE_PIN_COLORS, 2);
        if (!map.hasImage(`venue-${type}`)) map.addImage(`venue-${type}`, image, { pixelRatio: 2 });
      })
    );
    if (map.getLayer('venue-pins')) return;
    const categoryToType: unknown[] = ['match', ['get', 'category']];
    for (const type of VENUE_TYPES) {
      categoryToType.push(VENUE_TYPE_CATEGORIES[type], `venue-${type}`);
    }
    categoryToType.push('venue-konbini');
    map.addLayer({
      id: 'venue-pins',
      type: 'symbol',
      source: 'martin-venues',
      'source-layer': 'venues',
      minzoom: 13,
      filter: ['in', ['get', 'category'], ['literal', []]],
      layout: {
        'icon-image': categoryToType as never,
        'icon-size': 1,
        'icon-allow-overlap': false,
        'icon-padding': 2,
        visibility: 'none'
      },
      paint: { 'icon-opacity': 1 }
    });
    venuePinsReady = true;
  }

  let venuePinsReady = $state(false);

  // Route markers (start / end / loop / numbered stops) are DOM markers so
  // they can use the chrome tokens; the symbol layer only draws venues.
  let routeMarkers: maplibregl.Marker[] = [];
  let markerSize: MarkerSize = 24;
  let selectedVenue = $state<{
    lngLat: [number, number];
    type: VenueType;
    name: string;
    osmId: number | null;
  } | null>(null);
  let selectedVenueMarker: maplibregl.Marker | null = null;
  // The line venues are measured against for dimming (the selected route).
  let activeRouteLine: [number, number][] | null = null;
  // Sorted on-route osm_ids of the last dimming pass ('' = nothing dimmed).
  let dimmedKey = '';

  function venueTypeFor(category: unknown): VenueType {
    for (const type of VENUE_TYPES) {
      if (VENUE_TYPE_CATEGORIES[type].includes(String(category))) return type;
    }
    return 'konbini';
  }

  function clearRouteMarkers() {
    for (const m of routeMarkers) m.remove();
    routeMarkers = [];
  }

  function addRouteMarker(el: HTMLElement, lngLat: [number, number]) {
    const marker = new maplibregl.Marker({ element: el, anchor: 'center' })
      .setLngLat(lngLat)
      .addTo(map);
    routeMarkers.push(marker);
  }

  function applyMarkerSize() {
    const size = markerSizeForZoom(map.getZoom());
    if (size === markerSize) return;
    markerSize = size;
    for (const m of routeMarkers) m.getElement().dataset.size = String(size);
  }

  // Dim venues that are off the selected route to 45%. Runs when the map
  // settles so it sees the tiles that are actually on screen.
  function refreshVenueDimming() {
    if (!map.getLayer('venue-pins')) return;
    const line = activeRouteLine;
    if (!line || line.length < 2) {
      if (dimmedKey !== '') {
        dimmedKey = '';
        map.setPaintProperty('venue-pins', 'icon-opacity', 1);
      }
      return;
    }
    const onRoute: number[] = [];
    for (const f of map.queryRenderedFeatures({ layers: ['venue-pins'] })) {
      if (f.geometry.type !== 'Point') continue;
      const id = Number(f.properties?.osm_id);
      if (!Number.isFinite(id)) continue;
      const p = f.geometry.coordinates as [number, number];
      if (metresToPolyline(p, line) <= ON_ROUTE_M) onRoute.push(id);
    }
    // Only repaint when the on-route set changed; a repaint fires idle again.
    const key = onRoute.sort((a, b) => a - b).join(',');
    if (key === dimmedKey) return;
    dimmedKey = key;
    map.setPaintProperty('venue-pins', 'icon-opacity', [
      'case',
      ['in', ['get', 'osm_id'], ['literal', onRoute]],
      1,
      0.45
    ]);
  }

  onMount(() => {
    map = new maplibregl.Map({
      container,
      style: {
        version: 8,
        glyphs: 'https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf',
        sources: {
          'carto-light': {
            type: 'raster',
            tiles: cartoTiles,
            tileSize: 256,
            attribution:
              '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>'
          },
          'martin-roads': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/osm_roads/{z}/{x}/{y}`],
            minzoom: 8,
            maxzoom: 18
          },
          'martin-landuse': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/osm_landuse/{z}/{x}/{y}`],
            minzoom: 10,
            maxzoom: 18
          },
          'martin-routes': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/routes/{z}/{x}/{y}`],
            minzoom: 8,
            maxzoom: 18
          },
          'martin-venues': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/venues/{z}/{x}/{y}`],
            minzoom: 12,
            maxzoom: 18
          }
        },
        layers: [
          // 1. Basemap — the only always-visible layer
          {
            id: 'basemap',
            type: 'raster',
            source: 'carto-light',
            paint: { 'raster-opacity': 1 }
          },
          // 2. Curated routes — always visible. Route blue over a white
          // casing (resting 3px over 6.5px); route-muted when a route is
          // selected or a data layer is on (see the dimming effect below).
          {
            id: 'curated-routes-casing',
            type: 'line',
            source: 'martin-routes',
            'source-layer': 'routes',
            layout: { 'line-cap': 'round', 'line-join': 'round' },
            paint: {
              'line-color': ROUTE_CASING,
              'line-width': ['interpolate', ['linear'], ['zoom'], 8, 2.5, 14, 6.5]
            }
          },
          {
            id: 'curated-routes',
            type: 'line',
            source: 'martin-routes',
            'source-layer': 'routes',
            layout: { 'line-cap': 'round', 'line-join': 'round' },
            paint: {
              'line-color': ROUTE,
              'line-width': ['interpolate', ['linear'], ['zoom'], 8, 1, 14, 3]
            }
          }
          // Everything else is added dynamically and hidden by default
        ]
      },
      center: initialCenter,
      zoom: initialZoom,
      interactive,
      // Credits are our own attribution pill + Data sources sheet.
      attributionControl: false
    });

    if (showControls) {
      map.addControl(new maplibregl.NavigationControl(), 'top-right');
    }

    let tileErrorShown = false;
    map.on('error', (e) => {
      if (tileErrorShown) return;
      const msg = e.error?.message ?? '';
      if (msg.includes('Failed to fetch') || msg.includes('localhost:3000')) {
        tileError = 'Tile server offline. Run: make dev-martin';
        tileErrorShown = true;
      }
    });

    map.on('load', () => {
      // Time-scrubbed area layers (building shadows, rain cells). Inserted
      // below curated-routes so every line layer stays readable above them;
      // visibility is driven by the visibleLayers store like other layers.
      map.addSource('shadow-cells', {
        type: 'vector',
        tiles: shadowTileUrls($shadowSlice),
        minzoom: 11,
        maxzoom: 16
      });
      map.addLayer(
        {
          id: 'shadow-cells-fill',
          type: 'fill',
          source: 'shadow-cells',
          'source-layer': 'shadow_cells',
          // Shade ramp by coverage: a faint sun wash on open cells, then
          // shadow polygons from partial to deep (darker = deeper shade).
          paint: {
            'fill-color': [
              'interpolate',
              ['linear'],
              ['get', 'shade_coverage'],
              0,
              SHADE_RAMP[0],
              0.33,
              SHADE_RAMP[1],
              0.66,
              SHADE_RAMP[2],
              1,
              SHADE_RAMP[3]
            ],
            'fill-antialias': false,
            'fill-opacity': [
              'interpolate',
              ['linear'],
              ['get', 'shade_coverage'],
              0,
              0.1,
              0.3,
              0.28,
              1,
              0.45
            ]
          },
          layout: { visibility: 'none' }
        },
        'curated-routes-casing'
      );

      map.addSource('rain-cells', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer(
        {
          id: 'rain-cells-fill',
          type: 'fill',
          source: 'rain-cells',
          // Rain ramp by intensity (mm/h): drizzle < 0.5, light 0.5–2,
          // moderate 2–8, heavy > 8. Filled areas, so it stays apart from
          // route blue.
          paint: {
            'fill-color': [
              'step',
              ['get', 'precip'],
              RAIN_RAMP.drizzle,
              RAIN_STEPS_MMH[0],
              RAIN_RAMP.light,
              RAIN_STEPS_MMH[1],
              RAIN_RAMP.moderate,
              RAIN_STEPS_MMH[2],
              RAIN_RAMP.heavy
            ],
            'fill-antialias': false,
            'fill-opacity': ['step', ['get', 'precip'], 0, 0.05, 0.4]
          },
          layout: { visibility: 'none' }
        },
        'curated-routes-casing'
      );

      // Add optional layers (hidden by default)
      for (const def of Object.values(OPTIONAL_LAYERS)) {
        map.addLayer(def as any);
      }

      // Route alternatives: unselected ones in route-muted over a white
      // casing (resting weight); the selected one is drawn by highlight-route.
      for (let i = 0; i < 3; i++) {
        map.addSource(`route-alt-${i}`, {
          type: 'geojson',
          lineMetrics: true,
          data: { type: 'FeatureCollection', features: [] }
        });
        map.addLayer({
          id: `route-alt-casing-${i}`,
          type: 'line',
          source: `route-alt-${i}`,
          layout: { 'line-cap': 'round', 'line-join': 'round' },
          paint: { 'line-width': 6.5, 'line-color': ROUTE_CASING }
        });
        map.addLayer({
          id: `route-alt-line-${i}`,
          type: 'line',
          source: `route-alt-${i}`,
          layout: { 'line-cap': 'round', 'line-join': 'round' },
          paint: { 'line-width': 3, 'line-color': ROUTE_MUTED }
        });
      }

      // Selected route: route blue 5px over a 9px white casing.
      map.addSource('highlight-route', {
        type: 'geojson',
        lineMetrics: true,
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer({
        id: 'highlight-route-casing',
        type: 'line',
        source: 'highlight-route',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: { 'line-width': 9, 'line-color': ROUTE_CASING }
      });
      map.addLayer({
        id: 'highlight-route-line',
        type: 'line',
        source: 'highlight-route',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: { 'line-width': 5, 'line-color': ROUTE }
      });

      void addVenuePins().catch(() => {
        // Icon rasterisation failed (no canvas); venues stay off.
      });

      // Foreground navigation position (route blue with a white ring and an
      // accuracy halo — decided 2026-09-30). Browser location is never persisted.
      map.addSource('live-navigation-position', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer({
        id: 'live-navigation-accuracy',
        type: 'circle',
        source: 'live-navigation-position',
        paint: {
          'circle-radius': 18,
          'circle-color': ROUTE,
          'circle-opacity': 0.14,
          'circle-stroke-width': 0
        }
      });
      map.addLayer({
        id: 'live-navigation-dot',
        type: 'circle',
        source: 'live-navigation-position',
        paint: {
          'circle-radius': 7,
          'circle-color': ROUTE,
          'circle-stroke-color': ROUTE_CASING,
          'circle-stroke-width': 3
        }
      });

      markerSize = markerSizeForZoom(map.getZoom());
      mapLoaded = true;
      mapInstance.set(map);
      // Dev-only handle for browser tests and manual inspection.
      if (import.meta.env.DEV)
        (window as unknown as { __komorebiMap?: maplibregl.Map }).__komorebiMap = map;
      publishBounds(); // seed viewport-driven layers before the first pan
    });

    function publishBounds() {
      const bounds = map.getBounds();
      mapBounds.set({
        minLon: bounds.getWest(),
        minLat: bounds.getSouth(),
        maxLon: bounds.getEast(),
        maxLat: bounds.getNorth()
      });
      onmoveend?.({ bounds });
    }

    map.on('moveend', publishBounds);
    map.on('zoom', applyMarkerSize);
    map.on('idle', refreshVenueDimming);

    map.on('click', (e) => {
      // A tap on a venue pin selects it instead of reaching the page (which
      // would otherwise drop a planner stop there).
      if (map.getLayer('venue-pins')) {
        const hit = map.queryRenderedFeatures(e.point, { layers: ['venue-pins'] })[0];
        if (hit && hit.geometry.type === 'Point') {
          const [lng, lat] = hit.geometry.coordinates;
          const osmId = Number(hit.properties?.osm_id);
          selectedVenue =
            selectedVenue && selectedVenue.osmId === osmId
              ? null
              : {
                  lngLat: [lng, lat],
                  type: venueTypeFor(hit.properties?.category),
                  name: String(hit.properties?.name ?? ''),
                  osmId: Number.isFinite(osmId) ? osmId : null
                };
          return;
        }
      }
      selectedVenue = null;
      onclick?.({ lng: e.lngLat.lng, lat: e.lngLat.lat });
    });
    map.on('mouseenter', 'venue-pins', () => (map.getCanvas().style.cursor = 'pointer'));
    map.on('mouseleave', 'venue-pins', () => (map.getCanvas().style.cursor = ''));
  });

  // Route markers: the planner's stops when any are set (start, numbered
  // waypoints, end), otherwise the ends of the highlighted route plus its
  // waypoints. Start and end within LOOP_M collapse into one loop marker.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const stops = $routeStops;
    const geom = highlightGeometry;
    const waypoints = highlightWaypoints;
    clearRouteMarkers();

    if (stops.length > 0) {
      const start = stops.find((s) => s.kind === 'start');
      const end = stops.find((s) => s.kind === 'end');
      const startLngLat: [number, number] | null = start ? [start.lon, start.lat] : null;
      const endLngLat: [number, number] | null = end ? [end.lon, end.lat] : null;
      const loop = startLngLat && endLngLat && metresBetween(startLngLat, endLngLat) < LOOP_M;
      if (startLngLat) {
        addRouteMarker(buildRouteMarker(loop ? 'loop' : 'start', markerSize), startLngLat);
      }
      if (endLngLat && !loop) addRouteMarker(buildRouteMarker('end', markerSize), endLngLat);
      for (const s of stops.filter((s) => s.kind === 'via')) {
        addRouteMarker(buildRouteMarker('waypoint', markerSize, { number: s.index }), [
          s.lon,
          s.lat
        ]);
      }
      return;
    }

    if (geom && geom.length > 1) {
      const startLngLat = geom[0];
      const endLngLat = geom[geom.length - 1];
      const loop = metresBetween(startLngLat, endLngLat) < LOOP_M;
      addRouteMarker(buildRouteMarker(loop ? 'loop' : 'start', markerSize), startLngLat);
      if (!loop) addRouteMarker(buildRouteMarker('end', markerSize), endLngLat);
      waypoints.forEach((wp, i) => {
        addRouteMarker(buildRouteMarker('waypoint', markerSize, { number: i + 1 }), wp);
      });
    }
  });

  // Opt-in venue pins: filter the symbol layer to the picked types.
  $effect(() => {
    if (!map || !mapLoaded || !venuePinsReady) return;
    const types = $visibleVenueTypes;
    const categories = [...types].flatMap((t) => VENUE_TYPE_CATEGORIES[t]);
    map.setFilter('venue-pins', ['in', ['get', 'category'], ['literal', categories]]);
    map.setLayoutProperty('venue-pins', 'visibility', categories.length > 0 ? 'visible' : 'none');
    if (categories.length === 0) selectedVenue = null;
  });

  // The selected venue is a DOM marker (near-black fill, white icon, label
  // chip) drawn over its pin.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const venue = selectedVenue;
    selectedVenueMarker?.remove();
    selectedVenueMarker = null;
    if (!venue) return;
    selectedVenueMarker = new maplibregl.Marker({
      element: buildSelectedVenueMarker(venue.type, venue.name),
      anchor: 'center'
    })
      .setLngLat(venue.lngLat)
      .addTo(map);
  });

  $effect(() => {
    if (!map || !mapLoaded) return;
    const position = $liveNavigationPosition;
    const source = map.getSource('live-navigation-position') as maplibregl.GeoJSONSource;
    if (!source) return;
    source.setData(
      position
        ? {
            type: 'Feature',
            geometry: { type: 'Point', coordinates: [position.longitude, position.latitude] },
            properties: { accuracy: position.accuracy }
          }
        : { type: 'FeatureCollection', features: [] }
    );
  });

  onDestroy(() => {
    if (map) {
      clearRouteMarkers();
      selectedVenueMarker?.remove();
      mapInstance.set(null);
      map.remove();
    }
  });

  // Toggle optional layers based on store
  $effect(() => {
    if (!map || !mapLoaded) return;
    const layers = $visibleLayers;
    const layerMap: Record<string, string> = {
      'cycling-roads': 'cycling-roads',
      landuse: 'landuse-fill',
      shadows: 'shadow-cells-fill',
      'rain-cells': 'rain-cells-fill'
    };
    for (const [key, layerId] of Object.entries(layerMap)) {
      const vis = layers.has(key as any) ? 'visible' : 'none';
      if (map.getLayer(layerId)) {
        map.setLayoutProperty(layerId, 'visibility', vis);
      }
    }
  });

  // Swap the shadow tile slice when the departure time moves. Debounced so a
  // slider drag settles before tiles reload; already-seen slices come back
  // from the browser HTTP cache.
  let shadowTimer: ReturnType<typeof setTimeout>;
  $effect(() => {
    const slice = $shadowSlice;
    if (!map || !mapLoaded) return;
    clearTimeout(shadowTimer);
    shadowTimer = setTimeout(() => {
      const src = map.getSource('shadow-cells') as maplibregl.VectorTileSource | undefined;
      src?.setTiles(shadowTileUrls(slice));
    }, 150);
  });

  // Repaint the rain cells when the departure time or viewport moves.
  // Snapshots are cached per quantized time step so scrubbing back and forth
  // repaints from memory.
  let rainTimer: ReturnType<typeof setTimeout>;
  let rainSeq = 0;
  const rainCache = new Map<string, FeatureCollection>();
  // Whether the current rain snapshot paints anything; drives route muting.
  let rainHasCells = $state(false);

  async function refreshRainCells(bbox: string, at: string) {
    const src = map.getSource('rain-cells') as maplibregl.GeoJSONSource | undefined;
    if (!src) return;
    const t = new Date(at);
    t.setSeconds(0, 0);
    t.setMinutes(Math.round(t.getMinutes() / 10) * 10);
    const key = `${t.toISOString()}|${bbox}`;
    let fc = rainCache.get(key);
    const seq = ++rainSeq;
    if (!fc) {
      try {
        const res = await weather.grid(bbox, t.toISOString());
        fc = {
          type: 'FeatureCollection',
          features: res.cells.map((c) => ({
            type: 'Feature',
            geometry: {
              type: 'Polygon',
              coordinates: [
                [
                  [c.min_lon, c.min_lat],
                  [c.max_lon, c.min_lat],
                  [c.max_lon, c.max_lat],
                  [c.min_lon, c.max_lat],
                  [c.min_lon, c.min_lat]
                ]
              ]
            },
            properties: { precip: c.precip_intensity_mmh, source: c.precip_source }
          }))
        };
        rainCache.set(key, fc);
        if (rainCache.size > 200) {
          rainCache.delete(rainCache.keys().next().value as string);
        }
      } catch {
        return; // API offline or no data — keep the previous snapshot
      }
      if (seq !== rainSeq) return; // a newer request superseded this one
    }
    src.setData(fc);
    rainHasCells = fc.features.some((f) => ((f.properties?.precip as number) ?? 0) >= 0.05);
  }

  $effect(() => {
    const at = $departureAt;
    const bbox = $bboxString;
    const visible = $visibleLayers.has('rain-cells');
    if (!map || !mapLoaded || !bbox || !visible) return;
    clearTimeout(rainTimer);
    rainTimer = setTimeout(() => void refreshRainCells(bbox, at), 180);
  });

  // Show all route alternatives on map (dimmed unselected, bright selected)
  $effect(() => {
    if (!map || !mapLoaded) return;
    const alts = $routeDisplays;
    for (let i = 0; i < 3; i++) {
      const src = map.getSource(`route-alt-${i}`) as maplibregl.GeoJSONSource;
      if (!src) continue;
      const alt = alts[i];
      if (alt && alt.coords.length > 0 && !alt.selected) {
        src.setData({
          type: 'Feature',
          geometry: { type: 'LineString', coordinates: alt.coords },
          properties: {}
        });
      } else {
        src.setData({ type: 'FeatureCollection', features: [] });
      }
    }
  });

  // Curated routes fall back to route-muted when a route is selected or a
  // data layer is showing something (shadows, or rain cells that actually
  // contain rain). Read every input unconditionally so the effect never drops
  // a dependency behind a short-circuit and gets stuck muted.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const alts = $routeDisplays;
    const layers = $visibleLayers;
    const raining = rainHasCells;
    const highlighted = highlightGeometry !== null && highlightGeometry.length > 0;
    const hasHighlight = highlighted || alts.length > 0;
    const dataLayerOn = layers.has('shadows') || (layers.has('rain-cells') && raining);
    if (map.getLayer('curated-routes')) {
      map.setPaintProperty(
        'curated-routes',
        'line-color',
        hasHighlight || dataLayerOn ? ROUTE_MUTED : ROUTE
      );
    }
  });

  // Update highlight route geometry and condition overlay. This effect is the
  // single writer for the highlight-route source: it draws the highlighted
  // discovery route when one is set, otherwise the selected planner route, so
  // clearing a highlight can never wipe a planner line that is still active.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const highlighted = highlightGeometry;
    const planner = $selectedRouteGeometry;
    const segs = conditionSegments;
    const distM = conditionRouteDistanceM;
    const overlay = $activeOverlay;

    const src = map.getSource('highlight-route') as maplibregl.GeoJSONSource;
    if (!src) return;

    const geom = highlighted !== null && highlighted.length > 0 ? highlighted : planner;
    activeRouteLine = geom !== null && geom.length > 1 ? geom : null;
    refreshVenueDimming();
    if (geom !== null && geom.length > 0) {
      src.setData({
        type: 'Feature',
        geometry: { type: 'LineString', coordinates: geom },
        properties: {}
      });
      if (geom === highlighted) {
        if (overlay && segs.length > 0) {
          map.setPaintProperty(
            'highlight-route-line',
            'line-gradient',
            buildLineGradient(segs, overlay, distM)
          );
        } else {
          map.setPaintProperty('highlight-route-line', 'line-gradient', null);
          map.setPaintProperty('highlight-route-line', 'line-color', ROUTE);
        }
      }
      // Planner geometry keeps the paint set by NavigationPanel (profile
      // color, overlay gradient) untouched.
    } else {
      src.setData({ type: 'FeatureCollection', features: [] });
    }
  });
</script>

<div class="relative w-full h-full">
  <div bind:this={container} class="w-full h-full"></div>

  <!-- Attribution grows with the visible data: PLATEAU when shadows or the
       shade overlay show, Open-Meteo when rain cells or a weather overlay show. -->
  <div class="absolute bottom-3 right-3 z-20">
    <AttributionPill
      shade={$visibleLayers.has('shadows') ||
        ($activeOverlay === 'shade' && conditionSegments.length > 0)}
      weather={$visibleLayers.has('rain-cells') ||
        (($activeOverlay === 'rain' || $activeOverlay === 'wind') && conditionSegments.length > 0)}
    />
  </div>
  <DataSourcesSheet />

  {#if tileError}
    <div
      class="absolute bottom-4 left-4 z-10 max-w-56
                bg-warning-surface border border-warning text-warning-strong text-xs
                px-3 py-2 rounded-md backdrop-blur flex items-center gap-2"
    >
      <span>{tileError}</span>
      <button
        onclick={() => (tileError = null)}
        aria-label="Dismiss"
        class="text-warning-strong hover:text-text">x</button
      >
    </div>
  {/if}
</div>

<style>
  /* Route markers (Penpot "Markers"). Global because the elements are built
     outside Svelte's template; colours come from the app.css tokens. */
  :global(.km-marker) {
    width: var(--km-size);
    height: var(--km-size);
    --km-size: 24px;
    --km-ring: 2.5px;
    filter: drop-shadow(0 4px 10px rgba(0, 0, 0, 0.14)) drop-shadow(0 0 1px rgba(0, 0, 0, 0.1));
    pointer-events: none;
  }
  :global(.km-marker[data-size='20']) {
    --km-size: 20px;
    --km-ring: 2px;
  }
  :global(.km-marker[data-size='28']) {
    --km-size: 28px;
  }
  :global(.km-marker__disc) {
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    border-radius: 999px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-surface-base);
    border: var(--km-ring) solid var(--color-route);
    color: var(--color-route);
    font-weight: 600;
    font-size: calc(var(--km-size) * 0.5);
    line-height: 1;
  }
  :global(.km-marker__disc > svg) {
    width: 62%;
    height: 62%;
  }
  /* End: route-blue fill, 2px white outer ring, white flag. */
  :global(.km-marker[data-kind='end'] .km-marker__disc) {
    background: var(--color-route);
    border: 2px solid var(--color-route-casing);
    color: var(--color-on-primary);
  }
  /* Waypoints: 22px white, 2px route-blue ring, number in route blue. */
  :global(.km-marker[data-kind='waypoint']) {
    --km-size: 22px;
    --km-ring: 2px;
  }
  /* Loop: the start marker plus a small end badge at its top-right. */
  :global(.km-marker__badge) {
    position: absolute;
    top: -22%;
    right: -22%;
    width: 58%;
    height: 58%;
    border-radius: 999px;
    background: var(--color-route);
    border: 1.5px solid var(--color-route-casing);
    color: var(--color-on-primary);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  :global(.km-marker__badge > svg) {
    width: 64%;
    height: 64%;
  }

  /* Selected venue: near-black fill, white icon, label chip. */
  :global(.km-venue) {
    width: 28px;
    height: 28px;
    pointer-events: none;
    filter: drop-shadow(0 4px 10px rgba(0, 0, 0, 0.14));
  }
  :global(.km-venue__disc) {
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    border-radius: 999px;
    background: var(--color-primary);
    border: 1px solid var(--color-primary);
    color: var(--color-on-primary);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  :global(.km-venue__disc > svg) {
    width: 60%;
    height: 60%;
  }
  :global(.km-venue__chip) {
    position: absolute;
    left: 50%;
    top: calc(100% + 4px);
    transform: translateX(-50%);
    white-space: nowrap;
    background: var(--color-primary);
    color: var(--color-on-primary);
    font-size: 11px;
    font-weight: 500;
    line-height: 1;
    padding: 5px 8px;
    border-radius: 6px;
  }
</style>
