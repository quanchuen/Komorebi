<!-- web/src/lib/components/Map.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import maplibregl from 'maplibre-gl';
  import 'maplibre-gl/dist/maplibre-gl.css';
  import { mapInstance, mapBounds, activeOverlay } from '$lib/stores/map';
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

  const MARTIN_URL = 'http://localhost:3000';

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
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>'
          },
          'martin-roads': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/osm_roads/{z}/{x}/{y}`],
            minzoom: 8,
            maxzoom: 18
          },
          'martin-pois': {
            type: 'vector',
            tiles: [`${MARTIN_URL}/osm_pois/{z}/{x}/{y}`],
            minzoom: 12,
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
          {
            id: 'basemap',
            type: 'raster',
            source: 'carto-dark',
            paint: { 'raster-opacity': 0.85 }
          },
          {
            id: 'landuse-fill',
            type: 'fill',
            source: 'martin-landuse',
            'source-layer': 'osm_landuse',
            paint: {
              'fill-color': '#166534',
              'fill-opacity': 0.15
            }
          },
          {
            id: 'cycling-roads',
            type: 'line',
            source: 'martin-roads',
            'source-layer': 'osm_roads',
            filter: ['in', 'highway', 'cycleway', 'path', 'track'],
            paint: {
              'line-color': '#22d3ee',
              'line-width': ['interpolate', ['linear'], ['zoom'], 10, 0.5, 16, 3],
              'line-opacity': 0.5
            }
          },
          {
            id: 'curated-routes',
            type: 'line',
            source: 'martin-routes',
            'source-layer': 'routes',
            paint: {
              'line-color': '#10b981',
              'line-width': ['interpolate', ['linear'], ['zoom'], 8, 1.5, 14, 4],
              'line-opacity': 0.7
            }
          },
          {
            id: 'venue-circles',
            type: 'circle',
            source: 'martin-venues',
            'source-layer': 'venues',
            minzoom: 14,
            paint: {
              'circle-radius': 4,
              'circle-color': '#8b5cf6',
              'circle-stroke-color': '#1e1b4b',
              'circle-stroke-width': 1,
              'circle-opacity': 0.7
            }
          }
        ]
      },
      center: initialCenter,
      zoom: initialZoom,
      interactive
    });

    if (showControls) {
      map.addControl(new maplibregl.NavigationControl(), 'top-right');
    }

    // Show tile errors (Martin down, etc.)
    let tileErrorShown = false;
    map.on('error', (e) => {
      if (tileErrorShown) return;
      const msg = e.error?.message ?? '';
      if (msg.includes('Failed to fetch') || msg.includes('localhost:3000')) {
        tileError = 'Tile server not running (localhost:3000). Run: make dev-martin';
        tileErrorShown = true;
      }
    });

    map.on('load', () => {
      // Source + layer for highlighted route line
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
        paint: {
          'line-width': 5,
          'line-color': '#38BDF8', // sky-400 default
          'line-opacity': 0.9
        }
      });

      // Source for planner stops
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

      mapInstance.set(map);
    });

    map.on('moveend', () => {
      const bounds = map.getBounds();
      const b = {
        minLon: bounds.getWest(),
        minLat: bounds.getSouth(),
        maxLon: bounds.getEast(),
        maxLat: bounds.getNorth()
      };
      mapBounds.set(b);
      onmoveend?.({ bounds });
    });

    map.on('click', (e) => {
      onclick?.({ lng: e.lngLat.lng, lat: e.lngLat.lat });
    });
  });

  onDestroy(() => {
    if (map) {
      mapInstance.set(null);
      map.remove();
    }
  });

  // Reactively update highlighted route geometry and condition overlay
  $effect(() => {
    const geom = highlightGeometry;
    const segs = conditionSegments;
    const distM = conditionRouteDistanceM;
    const overlay = $activeOverlay;

    if (map && map.isStyleLoaded() && geom !== null) {
      const source = map.getSource('highlight-route') as maplibregl.GeoJSONSource;
      source.setData({
        type: 'Feature',
        geometry: { type: 'LineString', coordinates: geom },
        properties: {}
      });

      if (overlay && segs.length > 0) {
        const gradient = buildLineGradient(segs, overlay, distM);
        map.setPaintProperty('highlight-route-line', 'line-gradient', gradient);
      } else {
        // Reset to solid color — need to remove line-gradient first
        map.setPaintProperty('highlight-route-line', 'line-gradient', null);
        map.setPaintProperty('highlight-route-line', 'line-color', '#38BDF8');
      }
    } else if (map && map.isStyleLoaded() && geom === null) {
      const source = map.getSource('highlight-route') as maplibregl.GeoJSONSource;
      if (source) {
        source.setData({ type: 'FeatureCollection', features: [] });
      }
    }
  });
</script>

<div class="relative w-full h-full">
  <div bind:this={container} class="w-full h-full"></div>

  {#if tileError}
    <div class="absolute bottom-4 left-4 z-10
                bg-amber-950/90 border border-amber-700 text-amber-300 text-xs
                px-3 py-2 rounded-lg backdrop-blur flex items-center gap-2">
      <span>Tile server offline</span>
      <button onclick={() => tileError = null} class="text-amber-500 hover:text-amber-300">x</button>
    </div>
  {/if}
</div>
