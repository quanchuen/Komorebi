<!-- web/src/lib/components/Map.svelte -->
<script lang="ts">
  import { onMount, onDestroy, mount, unmount } from 'svelte';
  import maplibregl from 'maplibre-gl';
  import 'maplibre-gl/dist/maplibre-gl.css';
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
    routeWaypoints,
    VENUE_CATEGORIES
  } from '$lib/stores/map';
  import { foregroundNavigation } from '$lib/stores/navigation';
  import { sourceFreshness } from '$lib/stores/attribution';
  import { weather } from '$lib/api/client';
  import {
    buildLineGradient,
    ROUTE_COLORS,
    SHADOW_FILL_COLOR,
    SHADOW_FILL_OPACITY,
    RAIN_FILL_COLOR,
    RAIN_FILL_OPACITY,
    POSITION_COLOR,
    MARKER_COLORS
  } from '$lib/utils/conditionColors';
  import { markerImage, ENDPOINT_ICON_SIZE } from '$lib/utils/mapMarkers';
  import AttributionPill from './AttributionPill.svelte';
  import type { RouteConditionSegment } from '$lib/api/types';

  import type { RouteAlternative } from '$lib/api/types';
  import type { Feature, FeatureCollection } from 'geojson';
  import type { ExpressionSpecification } from 'maplibre-gl';

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
    routeAlternatives = [],
    onclick,
    onmoveend
  }: Props = $props();

  let container: HTMLDivElement;
  let map: maplibregl.Map;
  let attributionPill: ReturnType<typeof mount> | undefined;

  function endpointFeatures(geom: [number, number][]): Feature[] {
    const start = geom[0];
    const end = geom[geom.length - 1];
    const point = (coordinates: [number, number], kind: string): Feature => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates },
      properties: { kind }
    });
    return distanceM(start, end) <= LOOP_THRESHOLD_M
      ? [point(start, 'loop')]
      : [point(start, 'start'), point(end, 'end')];
  }
  let tileError = $state<string | null>(null);
  let mapLoaded = $state(false);

  const MARTIN_URL = 'http://localhost:3000';

  // Shadow tiles are sliced per (hour, month); the slider swaps the tile URL.
  // MapLibre needs absolute tile URLs, so resolve the /tiles proxy against the
  // page origin (the map only exists in the browser).
  function shadowTileUrls(slice: { hourSlot: number; month: number }): string[] {
    return [
      `${window.location.origin}/tiles/shadow_cells/{z}/{x}/{y}?hour_slot=${slice.hourSlot}&month=${slice.month}`
    ];
  }

  // Route line widths (spec § Routes): resting ≈ 3px over a 6.5px white
  // casing, selected ≈ 5px over 9px. Curated routes thin out at low zoom.
  const CURATED_WIDTH: ExpressionSpecification = [
    'interpolate',
    ['linear'],
    ['zoom'],
    8,
    1.5,
    14,
    3
  ];
  const CURATED_CASING_WIDTH: ExpressionSpecification = [
    'interpolate',
    ['linear'],
    ['zoom'],
    8,
    4,
    14,
    6.5
  ];

  // A loop's start and end share one marker (start marker + end badge).
  const LOOP_THRESHOLD_M = 50;

  function distanceM(a: [number, number], b: [number, number]): number {
    const rad = Math.PI / 180;
    const dLat = (b[1] - a[1]) * rad;
    const dLon = (b[0] - a[0]) * rad;
    const h =
      Math.sin(dLat / 2) ** 2 +
      Math.cos(a[1] * rad) * Math.cos(b[1] * rad) * Math.sin(dLon / 2) ** 2;
    return 6371000 * 2 * Math.atan2(Math.sqrt(h), Math.sqrt(1 - h));
  }

  // Optional layers: defined here but hidden by default
  const OPTIONAL_LAYERS = {
    'cycling-roads': {
      id: 'cycling-roads',
      type: 'line' as const,
      source: 'martin-roads',
      'source-layer': 'osm_roads',
      filter: ['in', 'highway', 'cycleway', 'path', 'track'],
      paint: {
        'line-color': '#22d3ee',
        'line-width': ['interpolate', ['linear'], ['zoom'], 10, 0.5, 16, 2],
        'line-opacity': 0.4
      },
      layout: { visibility: 'none' as const }
    },
    landuse: {
      id: 'landuse-fill',
      type: 'fill' as const,
      source: 'martin-landuse',
      'source-layer': 'osm_landuse',
      paint: {
        'fill-color': '#166534',
        'fill-opacity': 0.12
      },
      layout: { visibility: 'none' as const }
    },
    // Venues are opt-in per type (visibleVenueTypes); the filter is set by
    // an effect below. Icon ids resolve through styleimagemissing.
    venues: {
      id: 'venue-pins',
      type: 'symbol' as const,
      source: 'martin-venues',
      'source-layer': 'venues',
      minzoom: 13,
      filter: ['in', ['get', 'category'], ['literal', []]],
      layout: {
        visibility: 'none' as const,
        'icon-image': [
          'match',
          ['get', 'category'],
          'konbini',
          'venue-konbini',
          'cafe',
          'venue-cafe',
          'water',
          'venue-water',
          ['bike-shop', 'bike-repair'],
          'venue-repair',
          ''
        ],
        'icon-allow-overlap': false,
        'icon-padding': 2
      }
    }
  };

  onMount(() => {
    map = new maplibregl.Map({
      container,
      style: {
        version: 8,
        glyphs: 'https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf',
        sources: {
          'carto-light': {
            type: 'raster',
            tiles: [
              'https://a.basemaps.cartocdn.com/light_all/{z}/{x}/{y}@2x.png',
              'https://b.basemaps.cartocdn.com/light_all/{z}/{x}/{y}@2x.png',
              'https://c.basemaps.cartocdn.com/light_all/{z}/{x}/{y}@2x.png'
            ],
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
          // 2. Curated routes — route blue over a white casing, always visible
          {
            id: 'curated-routes-casing',
            type: 'line',
            source: 'martin-routes',
            'source-layer': 'routes',
            layout: { 'line-cap': 'round', 'line-join': 'round' },
            paint: {
              'line-color': ROUTE_COLORS.casing,
              'line-width': CURATED_CASING_WIDTH
            }
          },
          {
            id: 'curated-routes',
            type: 'line',
            source: 'martin-routes',
            'source-layer': 'routes',
            layout: { 'line-cap': 'round', 'line-join': 'round' },
            paint: {
              'line-color': ROUTE_COLORS.route,
              'line-width': CURATED_WIDTH
            }
          }
          // Everything else is added dynamically and hidden by default
        ]
      },
      center: initialCenter,
      zoom: initialZoom,
      interactive,
      // Replaced by the attribution pill + Data sources sheet.
      attributionControl: false
    });

    // Bottom-right stack: controls added later sit above earlier ones, so the
    // attribution pill goes first and the zoom buttons land on top of it.
    const attributionEl = document.createElement('div');
    attributionEl.className = 'maplibregl-ctrl';
    attributionPill = mount(AttributionPill, { target: attributionEl });
    map.addControl(
      {
        onAdd: () => attributionEl,
        onRemove: () => attributionEl.remove()
      },
      'bottom-right'
    );

    if (showControls) {
      map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
    }

    // Marker and venue icons are drawn on demand (lib/utils/mapMarkers.ts).
    map.on('styleimagemissing', (e) => {
      if (map.hasImage(e.id)) return;
      const img = markerImage(e.id);
      if (img) map.addImage(e.id, img.image, img.options);
    });

    let tileErrorShown = false;
    map.on('error', (e) => {
      // A failed tile fetch does not schedule a frame. When every tile fails
      // (tile servers offline) the render loop can go idle before the last
      // failure lands, and 'load' — which seeds the viewport and every layer
      // below — never fires. Ask for one more frame so it can.
      map.triggerRepaint();
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
          paint: {
            'fill-color': SHADOW_FILL_COLOR,
            'fill-antialias': false,
            'fill-opacity': SHADOW_FILL_OPACITY
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
          paint: {
            'fill-color': RAIN_FILL_COLOR,
            'fill-antialias': false,
            'fill-opacity': RAIN_FILL_OPACITY
          },
          layout: { visibility: 'none' }
        },
        'curated-routes-casing'
      );

      // Add optional layers (hidden by default). Venues are markers, so
      // they go on top after the route lines instead.
      for (const def of Object.values(OPTIONAL_LAYERS)) {
        if (def.id !== 'venue-pins') map.addLayer(def as any);
      }

      // Route alternatives (up to 3 route-muted + 1 highlighted)
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
          paint: { 'line-width': 6.5, 'line-color': ROUTE_COLORS.casing }
        });
        map.addLayer({
          id: `route-alt-line-${i}`,
          type: 'line',
          source: `route-alt-${i}`,
          layout: { 'line-cap': 'round', 'line-join': 'round' },
          paint: { 'line-width': 3, 'line-color': ROUTE_COLORS.muted }
        });
      }

      // Highlight route source + layer (above alternatives)
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
        paint: { 'line-width': 9, 'line-color': ROUTE_COLORS.casing }
      });
      map.addLayer({
        id: 'highlight-route-line',
        type: 'line',
        source: 'highlight-route',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: { 'line-width': 5, 'line-color': ROUTE_COLORS.route }
      });

      // Venue pins sit above the route lines, below route markers.
      map.addLayer(OPTIONAL_LAYERS.venues as unknown as maplibregl.LayerSpecification);

      // Waypoints / stops: numbered markers between start and end.
      map.addSource('route-waypoints', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer({
        id: 'route-waypoints-marker',
        type: 'symbol',
        source: 'route-waypoints',
        layout: {
          'icon-image': ['concat', 'marker-waypoint-', ['to-string', ['get', 'n']]],
          'icon-allow-overlap': true,
          'icon-ignore-placement': true
        }
      });

      // Route endpoints: start, end, or one combined loop marker.
      map.addSource('route-endpoints', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer({
        id: 'route-endpoints-marker',
        type: 'symbol',
        source: 'route-endpoints',
        layout: {
          'icon-image': ['concat', 'marker-', ['get', 'kind']],
          'icon-size': ENDPOINT_ICON_SIZE,
          'icon-allow-overlap': true,
          'icon-ignore-placement': true,
          // The end marker draws over the start when they nearly touch.
          'symbol-sort-key': ['match', ['get', 'kind'], 'end', 1, 0]
        }
      });

      // Foreground navigation position. Browser location is never persisted.
      // Accuracy halo in metres, heading cone when the device reports one,
      // hollow gray when the fix is stale (ADR 0007: honest state).
      map.addSource('live-navigation-position', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer({
        id: 'live-navigation-accuracy',
        type: 'circle',
        source: 'live-navigation-position',
        filter: ['!', ['get', 'stale']],
        paint: {
          // accuracy (m) → px: r0 is the radius at z0; doubles per zoom level.
          'circle-radius': [
            'interpolate',
            ['exponential', 2],
            ['zoom'],
            0,
            ['get', 'r0'],
            22,
            ['*', ['get', 'r0'], 4194304]
          ],
          'circle-color': POSITION_COLOR,
          'circle-opacity': 0.12,
          'circle-stroke-color': POSITION_COLOR,
          'circle-stroke-opacity': 0.3,
          'circle-stroke-width': 1,
          'circle-pitch-alignment': 'map'
        }
      });
      map.addLayer({
        id: 'live-navigation-heading',
        type: 'symbol',
        source: 'live-navigation-position',
        filter: ['all', ['!', ['get', 'stale']], ['has', 'heading']],
        layout: {
          'icon-image': 'position-heading',
          'icon-rotate': ['get', 'heading'],
          'icon-rotation-alignment': 'map',
          'icon-allow-overlap': true,
          'icon-ignore-placement': true
        }
      });
      map.addLayer({
        id: 'live-navigation-dot',
        type: 'circle',
        source: 'live-navigation-position',
        paint: {
          'circle-radius': 7,
          'circle-color': ['case', ['get', 'stale'], MARKER_COLORS.surface, POSITION_COLOR],
          'circle-stroke-color': [
            'case',
            ['get', 'stale'],
            MARKER_COLORS.stale,
            MARKER_COLORS.surface
          ],
          'circle-stroke-width': 3
        }
      });

      mapLoaded = true;
      mapInstance.set(map);
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

    map.on('click', (e) => {
      onclick?.({ lng: e.lngLat.lng, lat: e.lngLat.lat });
    });
  });

  // Metres per pixel at z0 for MapLibre's 512px tiles, at the equator.
  const METRES_PER_PX_Z0 = 78271.517;

  $effect(() => {
    if (!map || !mapLoaded) return;
    const position = $liveNavigationPosition;
    const status = $foregroundNavigation.status;
    const source = map.getSource('live-navigation-position') as maplibregl.GeoJSONSource;
    if (!source) return;
    if (!position) {
      source.setData({ type: 'FeatureCollection', features: [] });
      return;
    }
    const mpp = METRES_PER_PX_Z0 * Math.cos((position.latitude * Math.PI) / 180);
    const heading =
      position.heading !== null && Number.isFinite(position.heading) ? position.heading : undefined;
    source.setData({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [position.longitude, position.latitude] },
      properties: {
        r0: position.accuracy / mpp,
        stale: status === 'paused' || status === 'error',
        ...(heading !== undefined ? { heading } : {})
      }
    });
  });

  onDestroy(() => {
    if (attributionPill) unmount(attributionPill);
    if (map) {
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

  // Venues: opt-in by type. Nothing is drawn until a type is picked.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const types = [...$visibleVenueTypes];
    const categories = types.flatMap((t) => VENUE_CATEGORIES[t]);
    if (!map.getLayer('venue-pins')) return;
    map.setFilter('venue-pins', ['in', ['get', 'category'], ['literal', categories]]);
    map.setLayoutProperty('venue-pins', 'visibility', categories.length > 0 ? 'visible' : 'none');
  });

  // Numbered waypoint markers for the stops between start and end.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const points = $routeWaypoints;
    const src = map.getSource('route-waypoints') as maplibregl.GeoJSONSource | undefined;
    src?.setData({
      type: 'FeatureCollection',
      features: points.map((coordinates, i) => ({
        type: 'Feature',
        geometry: { type: 'Point', coordinates },
        properties: { n: i + 1 }
      }))
    });
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
        const validAt = new Date(res.valid_at);
        if (!Number.isNaN(validAt.getTime())) {
          const hm = validAt.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
          sourceFreshness.update((f) => ({ ...f, 'open-meteo': `Forecast for ${hm}` }));
        }
        if (rainCache.size > 200) {
          rainCache.delete(rainCache.keys().next().value as string);
        }
      } catch {
        return; // API offline or no data — keep the previous snapshot
      }
      if (seq !== rainSeq) return; // a newer request superseded this one
    }
    src.setData(fc);
  }

  $effect(() => {
    const at = $departureAt;
    const bbox = $bboxString;
    const visible = $visibleLayers.has('rain-cells');
    if (!map || !mapLoaded || !bbox || !visible) return;
    clearTimeout(rainTimer);
    rainTimer = setTimeout(() => void refreshRainCells(bbox, at), 180);
  });

  // Show unselected route alternatives on the map in route-muted; the
  // selected one is drawn by the highlight layers.
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

  // Curated routes turn route-muted when a route is selected or a data layer
  // (shade, rain) is on. Read every input unconditionally so the effect never
  // drops a dependency behind a short-circuit and gets stuck muted.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const alts = $routeDisplays;
    const layers = $visibleLayers;
    const highlighted = highlightGeometry !== null && highlightGeometry.length > 0;
    const planner = $selectedRouteGeometry;
    const hasSelection = highlighted || alts.length > 0 || (planner?.length ?? 0) > 0;
    const dataLayersOn = layers.has('shadows') || layers.has('rain-cells');
    if (map.getLayer('curated-routes')) {
      map.setPaintProperty(
        'curated-routes',
        'line-color',
        hasSelection || dataLayersOn ? ROUTE_COLORS.muted : ROUTE_COLORS.route
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
    const endpoints = map.getSource('route-endpoints') as maplibregl.GeoJSONSource | undefined;
    if (!src) return;

    const geom = highlighted !== null && highlighted.length > 0 ? highlighted : planner;
    endpoints?.setData({
      type: 'FeatureCollection',
      features: geom && geom.length > 1 ? endpointFeatures(geom) : []
    });
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
          map.setPaintProperty('highlight-route-line', 'line-color', ROUTE_COLORS.route);
        }
      }
      // Planner geometry keeps the paint set by NavigationPanel (route
      // colour, overlay gradient) untouched.
    } else {
      src.setData({ type: 'FeatureCollection', features: [] });
    }
  });
</script>

<div class="relative w-full h-full">
  <div bind:this={container} class="w-full h-full"></div>

  {#if tileError}
    <!-- Raised clear of the bottom-right attribution pill below lg widths,
         where the two would share the bottom row. -->
    <div
      role="status"
      class="absolute bottom-16 left-4 z-10 flex items-center gap-1 rounded-control border
             lg:bottom-4
             border-warning/50 bg-warning-surface py-0.5 pl-3 pr-0.5 text-xs text-warning-strong
             shadow-xs"
    >
      <span>{tileError}</span>
      <button
        type="button"
        onclick={() => (tileError = null)}
        aria-label="Dismiss tile server message"
        class="flex size-11 items-center justify-center rounded-control hover:bg-warning/20"
        >×</button
      >
    </div>
  {/if}
</div>
