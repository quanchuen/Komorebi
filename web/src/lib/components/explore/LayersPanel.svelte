<!-- web/src/lib/components/explore/LayersPanel.svelte -->
<!-- Frame C: "Map layers". CONDITIONS · FOLLOW THE TIME SLIDER — Shade,
     Rain, Wind, Greenery — each with a swatch, its source and licence, and a
     switch; ALWAYS-ON — Curated routes, Cycling infrastructure. Desktop: a
     340px popover under the Layers button. Mobile: a bottom sheet. Every row
     is the switch (52px tall, so the hit box is the whole row). -->
<script lang="ts">
  import { visibleLayers, toggleLayer, type MapLayer } from '$lib/stores/map';
  import { isMobile, layersOpen } from '$lib/stores/ui';
  import { flags } from '$lib/flags';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import SectionLabel from './SectionLabel.svelte';
  import Sheet from './Sheet.svelte';

  interface Row {
    layer: MapLayer;
    name: string;
    source: string;
    swatch: string;
  }

  // Swatch fills come from the data ramps in app.css (one hue per meaning).
  const conditions: Row[] = [
    {
      layer: 'shadows',
      name: 'Shade',
      source: 'Project PLATEAU (MLIT) · CC BY 4.0',
      swatch: 'swatch-shade'
    },
    {
      layer: 'rain-cells',
      name: 'Rain',
      source: 'Open-Meteo · CC BY 4.0',
      swatch: 'bg-rain-light'
    },
    {
      layer: 'wind',
      name: 'Wind',
      source: 'Open-Meteo · tail vs. headwind on your heading',
      swatch: 'swatch-wind'
    },
    {
      layer: 'greenery',
      name: 'Greenery',
      source: 'Tree cover · derived from OpenStreetMap',
      swatch: 'bg-greenery-mid'
    }
  ];
  const curatedRow: Row = {
    layer: 'curated-routes',
    name: 'Curated routes',
    source: 'Komorebi editors · community reviews',
    swatch: 'bg-route-muted'
  };
  const alwaysOn: Row[] = [
    ...(flags.curatedRoutes ? [curatedRow] : []),
    {
      layer: 'cycling-roads',
      name: 'Cycling infrastructure',
      source: 'OpenStreetMap contributors · ODbL',
      swatch: 'bg-text-placeholder/35'
    }
  ];

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') layersOpen.set(false);
  }
</script>

{#snippet row(r: Row)}
  {@const on = $visibleLayers.has(r.layer)}
  <button
    type="button"
    role="switch"
    aria-checked={on}
    onclick={() => toggleLayer(r.layer)}
    class="layer-row {$isMobile ? 'h-14' : 'h-13'}"
  >
    <span class="size-3.5 rounded-xs shrink-0 {r.swatch}" aria-hidden="true"></span>
    <span class="flex-1 min-w-0 text-left">
      <span
        class="block font-medium {$isMobile ? 'text-15' : 'text-sm'} {on
          ? 'text-text'
          : 'text-text-subtle'}">{r.name}</span
      >
      <span class="block truncate text-text-subtle {$isMobile ? 'text-xs' : 'text-2xs'}"
        >{r.source}</span
      >
    </span>
    <span
      class="switch {$isMobile ? 'switch-lg' : ''} {on ? 'bg-primary' : 'bg-fill'}"
      aria-hidden="true"
    >
      <span class="knob {on ? 'knob-on' : ''}"></span>
    </span>
  </button>
{/snippet}

{#snippet body()}
  <SectionLabel text="Conditions · follow the time slider" class="px-4 pt-3 pb-1" />
  <div class="flex flex-col px-2">
    {#each conditions as r (r.layer)}
      {@render row(r)}
    {/each}
  </div>
  <div class="mx-4 my-1.5 h-px bg-hairline"></div>
  {#if $isMobile}
    <p class="px-4 pt-2 text-xs text-text-subtle">
      Always on: {flags.curatedRoutes ? 'curated routes · ' : ''}cycling infrastructure
    </p>
    <div class="flex flex-col px-2 pt-1 pb-3">
      {#each alwaysOn as r (r.layer)}
        {@render row(r)}
      {/each}
    </div>
  {:else}
    <SectionLabel text="Always-on" class="px-4 pt-2 pb-1" />
    <div class="flex flex-col px-2 pb-2">
      {#each alwaysOn as r (r.layer)}
        {@render row(r)}
      {/each}
    </div>
  {/if}
{/snippet}

<svelte:window onkeydown={$layersOpen ? onkeydown : undefined} />

{#if $layersOpen}
  {#if $isMobile}
    <Sheet label="Map layers" role="dialog" class="sheet-layers">
      <div class="flex items-center justify-between pl-5 pr-2">
        <h2 class="text-17 font-semibold text-text-strong">Map layers</h2>
        <IconButton icon="x" label="Close" size={36} onclick={() => layersOpen.set(false)} />
      </div>
      {@render body()}
    </Sheet>
  {:else}
    <div
      role="dialog"
      aria-label="Map layers"
      class="absolute right-4 top-16 z-30 w-85 bg-surface-base border border-line rounded-lg shadow-lg"
    >
      <div class="flex items-center justify-between h-12 pl-4 pr-1">
        <h2 class="text-15 font-semibold text-text">Map layers</h2>
        <IconButton icon="x" label="Close" size={32} onclick={() => layersOpen.set(false)} />
      </div>
      <div class="h-px bg-hairline"></div>
      {@render body()}
    </div>
  {/if}
{/if}

<style>
  .layer-row {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 0 8px;
    border-radius: var(--radius-md);
    background: transparent;
  }
  .layer-row:hover {
    background: var(--color-surface-tint);
  }
  .layer-row:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }
  /* Penpot "Switch": 32×18 (desktop), 44×26 (mobile); knob 14 / 22. */
  .switch {
    position: relative;
    display: inline-block;
    width: 32px;
    height: 18px;
    border-radius: 999px;
    flex-shrink: 0;
    transition: background-color 0.15s ease;
  }
  .knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 999px;
    background: var(--color-surface-base);
    box-shadow: var(--shadow-xs);
    transition: transform 0.15s ease;
  }
  .knob-on {
    transform: translateX(14px);
  }
  .switch-lg {
    width: 44px;
    height: 26px;
  }
  .switch-lg .knob {
    width: 22px;
    height: 22px;
  }
  .switch-lg .knob-on {
    transform: translateX(18px);
  }
  .swatch-shade {
    background: linear-gradient(
      90deg,
      var(--color-shade-sun),
      var(--color-shade-partial),
      var(--color-shade-mostly),
      var(--color-shade-deep)
    );
  }
  .swatch-wind {
    background: linear-gradient(90deg, var(--color-wind-tail) 50%, var(--color-wind-head) 50%);
  }
  :global(.sheet-layers) {
    max-height: 60dvh;
  }
</style>
