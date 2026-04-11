<!-- web/src/lib/components/Map.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import maplibregl from 'maplibre-gl';
  import 'maplibre-gl/dist/maplibre-gl.css';
  import { mapInstance, mapBounds, activeOverlay, visibleLayers } from '$lib/stores/map';
  import { buildLineGradient } from '$lib/utils/conditionColors';
  import type { RouteConditionSegment } from '$lib/api/types';

  interface Props {
    interactive?: boolean;
    showControls?: boolean;
    initialCenter?: [number, number];
    initialZoom?: number;
    conditionSegments?: RouteConditionSegment[];
    conditionRouteDistanceM?: number;
    highlightGeometry?: [number, number][] | null;
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
    onclick,
    onmoveend
  }: Props = $props();

  let container: HTMLDivElement;
  let map: maplibregl.Map;
  let tileError = $state<string | null>(null);
  let mapLoaded = $state(false);

  const MARTIN_URL = 'http://localhost:3000';

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
    'landuse': {
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
    'venues': {
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
          'carto-dark': {
            type: 'raster',
            tiles: [
              'https://a.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}@2x.png',
              'https://b.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}@2x.png',
              'https://c.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}@2x.png'
            ],
            tileSize: 256,
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>'
          },
          'martin-roads': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/osm_roads/{z}/{x}/{y}`],
            minzoom: 8, maxzoom: 18
          },
          'martin-landuse': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/osm_landuse/{z}/{x}/{y}`],
            minzoom: 10, maxzoom: 18
          },
          'martin-routes': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/routes/{z}/{x}/{y}`],
            minzoom: 8, maxzoom: 18
          },
          'martin-venues': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/venues/{z}/{x}/{y}`],
            minzoom: 12, maxzoom: 18
          }
        },
        layers: [
          // 1. Basemap — the only always-visible layer
          {
            id: 'basemap',
            type: 'raster',
            source: 'carto-dark',
            paint: { 'raster-opacity': 0.85 }
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
      // Add optional layers (hidden by default)
      for (const def of Object.values(OPTIONAL_LAYERS)) {
        map.addLayer(def as any);
      }

      // Highlight route source + layer (above everything)
      map.addSource('highlight-route', {
        type: 'geojson',
        lineMetrics: true,
        data: { type: 'FeatureCollection', features: [] }
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

      mapLoaded = true;
      mapInstance.set(map);
    });

    map.on('moveend', () => {
      const bounds = map.getBounds();
      mapBounds.set({
        minLon: bounds.getWest(), minLat: bounds.getSouth(),
        maxLon: bounds.getEast(), maxLat: bounds.getNorth()
      });
      onmoveend?.({ bounds });
    });

    map.on('click', (e) => {
      onclick?.({ lng: e.lngLat.lng, lat: e.lngLat.lat });
    });
  });

  onDestroy(() => {
    if (map) { mapInstance.set(null); map.remove(); }
  });

  // Toggle optional layers based on store
  $effect(() => {
    if (!map || !mapLoaded) return;
    const layers = $visibleLayers;
    const layerMap: Record<string, string> = {
      'cycling-roads': 'cycling-roads',
      'landuse': 'landuse-fill',
      'venues': 'venue-circles'
    };
    for (const [key, layerId] of Object.entries(layerMap)) {
      const vis = layers.has(key as any) ? 'visible' : 'none';
      if (map.getLayer(layerId)) {
        map.setLayoutProperty(layerId, 'visibility', vis);
      }
    }
  });

  // Dim curated routes when a highlight route is active
  $effect(() => {
    if (!map || !mapLoaded) return;
    const hasHighlight = highlightGeometry !== null && highlightGeometry.length > 0;
    if (map.getLayer('curated-routes')) {
      map.setPaintProperty('curated-routes', 'line-opacity', hasHighlight ? 0.15 : 0.5);
    }
  });

  // Update highlight route geometry and condition overlay
  $effect(() => {
    if (!map || !mapLoaded) return;
    const geom = highlightGeometry;
    const segs = conditionSegments;
    const distM = conditionRouteDistanceM;
    const overlay = $activeOverlay;

    const src = map.getSource('highlight-route') as maplibregl.GeoJSONSource;
    if (!src) return;

    if (geom !== null && geom.length > 0) {
      src.setData({
        type: 'Feature',
        geometry: { type: 'LineString', coordinates: geom },
        properties: {}
      });
      if (overlay && segs.length > 0) {
        map.setPaintProperty('highlight-route-line', 'line-gradient', buildLineGradient(segs, overlay, distM));
      } else {
        map.setPaintProperty('highlight-route-line', 'line-gradient', null);
        map.setPaintProperty('highlight-route-line', 'line-color', '#38BDF8');
      }
    } else {
      src.setData({ type: 'FeatureCollection', features: [] });
    }
  });
</script>

<div class="relative w-full h-full">
  <div bind:this={container} class="w-full h-full"></div>

  {#if tileError}
    <div class="absolute bottom-4 left-4 z-10
                bg-amber-950/90 border border-amber-700 text-amber-300 text-xs
                px-3 py-2 rounded-lg backdrop-blur flex items-center gap-2">
      <span>{tileError}</span>
      <button onclick={() => tileError = null} class="text-amber-500 hover:text-amber-300">x</button>
    </div>
  {/if}
</div>
