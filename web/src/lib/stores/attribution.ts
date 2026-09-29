// web/src/lib/stores/attribution.ts
//
// Data-source credits. Every source is attributed where its data is visible:
// the attribution pill lists the sources on screen now, and the Data sources
// sheet lists all of them (docs/specs/2026-09-29-ui-rework.md § Attribution).
import { onDestroy, onMount } from 'svelte';
import { derived, writable } from 'svelte/store';
import { visibleLayers } from './map';

export type SourceId = 'osm' | 'carto' | 'plateau' | 'open-meteo' | 'nominatim' | 'valhalla';

export interface DataSource {
  id: SourceId;
  /** Full name for the sheet. */
  name: string;
  /** Short credit for the pill. */
  credit: string;
  usedFor: string;
  licence: string;
  licenceUrl: string;
  /** Static freshness, when it is a fixed property of the dataset. */
  freshness?: string;
}

export const DATA_SOURCES: DataSource[] = [
  {
    id: 'osm',
    name: 'OpenStreetMap contributors',
    credit: '© OpenStreetMap contributors',
    usedFor: 'Roads, paths, places, routing graph',
    licence: 'ODbL 1.0',
    licenceUrl: 'https://www.openstreetmap.org/copyright'
  },
  {
    id: 'carto',
    name: 'CARTO',
    credit: 'CARTO',
    usedFor: 'Basemap tiles',
    licence: 'CARTO basemap terms',
    licenceUrl: 'https://carto.com/attributions',
    freshness: 'Live tiles'
  },
  {
    id: 'plateau',
    name: 'Project PLATEAU · MLIT',
    credit: 'PLATEAU (MLIT)',
    usedFor: '3D buildings → shade',
    licence: 'CC BY 4.0',
    licenceUrl: 'https://www.mlit.go.jp/plateau/site-policy/',
    // PLATEAU_EDITION in pipelines/plateau_shadow/download.py.
    freshness: '2023 edition'
  },
  {
    id: 'open-meteo',
    name: 'Open-Meteo',
    credit: 'Open-Meteo',
    usedFor: 'Rain, wind, temperature',
    licence: 'CC BY 4.0',
    licenceUrl: 'https://open-meteo.com/en/license'
  },
  {
    id: 'nominatim',
    name: 'Nominatim',
    credit: 'Nominatim',
    usedFor: 'Place search',
    licence: 'ODbL (OSM data)',
    licenceUrl: 'https://operations.osmfoundation.org/policies/nominatim/',
    freshness: 'Live'
  },
  {
    id: 'valhalla',
    name: 'Valhalla',
    credit: 'Valhalla',
    usedFor: 'Route calculation (software)',
    licence: 'MIT',
    licenceUrl: 'https://github.com/valhalla/valhalla/blob/master/COPYING'
  }
];

export const OSM_FIX_URL = 'https://www.openstreetmap.org/fixthemap';

/**
 * Dynamic freshness reported by data as it arrives (e.g. the weather grid's
 * valid time). Sources without an entry here or a static value show as not
 * reported; nothing is invented.
 */
export const sourceFreshness = writable<Partial<Record<SourceId, string>>>({});

// Count of mounted components currently showing weather readouts. Weather is
// visible whenever one is on screen, independent of the rain map layer.
const weatherDisplays = writable(0);

/** Call from a component's script when it displays weather data. */
export function creditsWeather() {
  onMount(() => {
    weatherDisplays.update((n) => n + 1);
  });
  onDestroy(() => {
    weatherDisplays.update((n) => Math.max(0, n - 1));
  });
}

/** Sources whose data is visible now, in pill order. */
export const visibleSources = derived(
  [visibleLayers, weatherDisplays],
  ([$layers, $weather]): DataSource[] => {
    const ids: SourceId[] = ['osm', 'carto'];
    if ($layers.has('shadows')) ids.push('plateau');
    if ($layers.has('rain-cells') || $weather > 0) ids.push('open-meteo');
    return ids.map((id) => DATA_SOURCES.find((s) => s.id === id)!);
  }
);
