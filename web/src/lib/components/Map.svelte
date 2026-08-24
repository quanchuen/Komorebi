<!-- web/src/lib/components/Map.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
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
    bboxString
  } from '$lib/stores/map';
  import { weather } from '$lib/api/client';
  import { buildLineGradient } from '$lib/utils/conditionColors';
  import type { RouteConditionSegment } from '$lib/api/types';

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
    venues: {
      id: 'venue-circles',
      type: 'circle' as const,
      source: 'martin-venues',
      'source-layer': 'venues',
      minzoom: 13,
      paint: {
        'circle-radius': ['interpolate', ['linear'], ['zoom'], 13, 3, 16, 5],
        'circle-color': '#8b5cf6',
        'circle-stroke-color': '#1e1b4b',
        'circle-stroke-width': 1,
        'circle-opacity': 0.6
      },
      layout: { visibility: 'none' as const }
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
          // 2. Curated routes — subtle, always visible
          {
            id: 'curated-routes',
            type: 'line',
            source: 'martin-routes',
            'source-layer': 'routes',
            paint: {
              'line-color': '#10b981',
              'line-width': ['interpolate', ['linear'], ['zoom'], 8, 1, 14, 3],
              'line-opacity': 0.5
            }
          }
          // Everything else is added dynamically and hidden by default
        ]
      },
      center: initialCenter,
      zoom: initialZoom,
      interactive
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
          paint: {
            'fill-color': '#1e3a8a',
            'fill-antialias': false,
            'fill-opacity': [
              'interpolate',
              ['linear'],
              ['get', 'shade_coverage'],
              0.05,
              0,
              0.4,
              0.16,
              1,
              0.38
            ]
          },
          layout: { visibility: 'none' }
        },
        'curated-routes'
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
            'fill-color': [
              'interpolate',
              ['linear'],
              ['get', 'precip'],
              0.05,
              '#67e8f9',
              1,
              '#6366f1',
              4,
              '#4c1d95'
            ],
            'fill-antialias': false,
            'fill-opacity': [
              'interpolate',
              ['linear'],
              ['get', 'precip'],
              0,
              0,
              0.1,
              0.12,
              1,
              0.3,
              4,
              0.45
            ]
          },
          layout: { visibility: 'none' }
        },
        'curated-routes'
      );

      // Add optional layers (hidden by default)
      for (const def of Object.values(OPTIONAL_LAYERS)) {
        map.addLayer(def as any);
      }

      // Route alternatives (up to 3 dimmed + 1 highlighted)
      for (let i = 0; i < 3; i++) {
        map.addSource(`route-alt-${i}`, {
          type: 'geojson',
          lineMetrics: true,
          data: { type: 'FeatureCollection', features: [] }
        });
        map.addLayer({
          id: `route-alt-line-${i}`,
          type: 'line',
          source: `route-alt-${i}`,
          layout: { 'line-cap': 'round', 'line-join': 'round' },
          paint: { 'line-width': 3, 'line-color': '#64748b', 'line-opacity': 0.3 }
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
        paint: { 'line-width': 9, 'line-color': '#172033', 'line-opacity': 0.78 }
      });
      map.addLayer({
        id: 'highlight-route-line',
        type: 'line',
        source: 'highlight-route',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: { 'line-width': 5, 'line-color': '#38BDF8', 'line-opacity': 0.9 }
      });

      // Planner stops
      map.addSource('planner-stops', {
        type: 'geojson',
        data: { type: 'FeatureCollection', features: [] }
      });
      map.addLayer({
        id: 'planner-stops-circle',
        type: 'circle',
        source: 'planner-stops',
        paint: {
          'circle-radius': 8,
          'circle-color': '#38BDF8',
          'circle-stroke-color': '#0F172A',
          'circle-stroke-width': 2
        }
      });

      // Foreground navigation position. Browser location is never persisted.
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
          'circle-color': '#38bdf8',
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
          'circle-color': '#38bdf8',
          'circle-stroke-color': '#f8fafc',
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
      venues: 'venue-circles',
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
        map.setPaintProperty(`route-alt-line-${i}`, 'line-color', alt.color);
        map.setPaintProperty(`route-alt-line-${i}`, 'line-opacity', 0.3);
        map.setPaintProperty(`route-alt-line-${i}`, 'line-width', 3);
      } else {
        src.setData({ type: 'FeatureCollection', features: [] });
      }
    }
  });

  // Dim curated routes when routes are displayed. Read every input
  // unconditionally so the effect never drops a dependency behind a
  // short-circuit and gets stuck dimmed.
  $effect(() => {
    if (!map || !mapLoaded) return;
    const alts = $routeDisplays;
    const highlighted = highlightGeometry !== null && highlightGeometry.length > 0;
    const hasHighlight = highlighted || alts.length > 0;
    if (map.getLayer('curated-routes')) {
      map.setPaintProperty('curated-routes', 'line-opacity', hasHighlight ? 0.15 : 0.5);
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
          map.setPaintProperty('highlight-route-line', 'line-color', '#38BDF8');
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

  {#if tileError}
    <div
      class="absolute bottom-4 left-4 z-10
                bg-amber-950/90 border border-amber-700 text-amber-300 text-xs
                px-3 py-2 rounded-lg backdrop-blur flex items-center gap-2"
    >
      <span>{tileError}</span>
      <button onclick={() => (tileError = null)} class="text-amber-500 hover:text-amber-300"
        >x</button
      >
    </div>
  {/if}
</div>
