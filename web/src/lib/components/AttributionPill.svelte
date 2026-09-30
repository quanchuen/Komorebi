<!-- web/src/lib/components/AttributionPill.svelte -->
<script lang="ts">
  import { dataSourcesOpen } from '$lib/stores/ui';

  interface Props {
    /** Building-shadow data (Project PLATEAU) is visible. */
    shade?: boolean;
    /** Weather data (Open-Meteo) is visible. */
    weather?: boolean;
  }

  let { shade = false, weather = false }: Props = $props();

  // The pill credits the sources whose data is on screen right now and grows
  // with the active layers; the full list lives in the Data sources sheet.
  let credits = $derived.by(() => {
    const parts = ['© OpenStreetMap contributors', 'CARTO'];
    if (shade) parts.push('PLATEAU (MLIT)');
    if (weather) parts.push('Open-Meteo');
    return parts.join(' · ');
  });
</script>

<button
  type="button"
  onclick={() => dataSourcesOpen.set(true)}
  aria-haspopup="dialog"
  aria-expanded={$dataSourcesOpen}
  title="Data sources"
  class="inline-flex items-center min-h-11 px-3 rounded-full bg-surface-base/90 backdrop-blur
         border border-line shadow-xs text-2xs text-text-subtle whitespace-nowrap
         hover:bg-surface-base hover:text-text transition-colors
         focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
>
  {credits}
</button>
