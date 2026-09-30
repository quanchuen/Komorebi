<!-- web/src/lib/components/AttributionPill.svelte -->
<script lang="ts">
  import { dataSourcesOpen, isMobile } from '$lib/stores/ui';
  import Icon from './ui/Icon.svelte';

  interface Props {
    /** Building-shadow data (Project PLATEAU) is visible. */
    shade?: boolean;
    /** Weather data (Open-Meteo) is visible. */
    weather?: boolean;
  }

  let { shade = false, weather = false }: Props = $props();

  // The pill credits the sources whose data is on screen right now and grows
  // with the active layers; the full list lives in the Data sources sheet.
  // Penpot: 24px tall (22 on mobile), 88% white, radius 6, 11px subtle text
  // and an ⓘ. The button pads out to 44px around it.
  let credits = $derived.by(() => {
    const parts = $isMobile ? ['© OSM', 'CARTO'] : ['© OpenStreetMap contributors', 'CARTO'];
    if (shade) parts.push($isMobile ? 'PLATEAU' : 'PLATEAU (MLIT)');
    if (weather) parts.push('Open-Meteo');
    return parts.join(' · ');
  });
</script>

<button
  type="button"
  onclick={() => dataSourcesOpen.set(true)}
  aria-haspopup="dialog"
  aria-expanded={$dataSourcesOpen}
  aria-label="{credits} · Data sources"
  title="Data sources"
  class="group/attr inline-flex items-center min-h-11 min-w-11 focus:outline-none"
>
  <span
    class="inline-flex items-center gap-1 rounded-sm bg-surface-base/88 backdrop-blur
           text-text-subtle whitespace-nowrap transition-colors
           group-hover/attr:bg-surface-base group-hover/attr:text-text
           group-focus-visible/attr:ring-2 group-focus-visible/attr:ring-focus
           {$isMobile ? 'h-5.5 pl-1.5 pr-0.5 text-3xs' : 'h-6 pl-2 pr-0.5 text-2xs'}"
  >
    <span aria-hidden="true">{credits}</span>
    <Icon name="info" class={$isMobile ? 'size-5' : 'size-6'} />
  </span>
</button>
