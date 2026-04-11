<!-- web/src/lib/components/MapLayerControl.svelte -->
<!-- Unified control for map layers (data) and condition overlays (color) -->
<script lang="ts">
  import { activeOverlay, visibleLayers } from '$lib/stores/map';
  import type { OverlayType, MapLayer } from '$lib/stores/map';

  let expanded = $state(false);

  const conditionOverlays: { id: Exclude<OverlayType, null>; label: string; icon: string }[] = [
    { id: 'shade', label: 'Shade', icon: '☀' },
    { id: 'wind', label: 'Wind', icon: '💨' },
    { id: 'rain', label: 'Rain', icon: '🌧' }
  ];

  const dataLayers: { id: MapLayer; label: string; icon: string }[] = [
    { id: 'venues', label: 'Venues', icon: '📍' },
    { id: 'cycling-roads', label: 'Cycle paths', icon: '🚲' },
    { id: 'landuse', label: 'Green areas', icon: '🌳' }
  ];

  function toggleOverlay(id: Exclude<OverlayType, null>) {
    activeOverlay.update((cur) => (cur === id ? null : id));
  }

  function toggleLayer(id: MapLayer) {
    visibleLayers.update((set) => {
      const next = new Set(set);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  }
</script>

<div class="relative">
  <button
    onclick={() => expanded = !expanded}
    class="text-[10px] text-slate-400 hover:text-slate-200 bg-slate-800 hover:bg-slate-700
           border border-slate-700 rounded-lg px-3 py-1.5 transition-colors"
  >
    Layers
  </button>

  {#if expanded}
    <div class="absolute top-full right-0 mt-1 w-44 z-50
                bg-slate-800 border border-slate-700 rounded-xl shadow-xl
                overflow-hidden">

      <!-- Condition overlays (route coloring) -->
      <div class="px-3 pt-2.5 pb-1">
        <div class="text-[9px] text-slate-500 uppercase tracking-wider mb-1.5">Route overlay</div>
        <div class="flex gap-1.5">
          {#each conditionOverlays as ov}
            <button
              onclick={() => toggleOverlay(ov.id)}
              class="flex-1 text-center py-1 rounded text-[10px] transition-colors
                     {$activeOverlay === ov.id
                ? 'bg-sky-600/30 text-sky-300 border border-sky-500/40'
                : 'text-slate-400 hover:text-slate-200 border border-transparent hover:bg-slate-700'}"
            >
              <span class="block text-sm">{ov.icon}</span>
              {ov.label}
            </button>
          {/each}
        </div>
      </div>

      <div class="border-t border-slate-700 mx-2"></div>

      <!-- Data layers (show/hide) -->
      <div class="px-3 pt-2 pb-2.5">
        <div class="text-[9px] text-slate-500 uppercase tracking-wider mb-1.5">Map layers</div>
        {#each dataLayers as layer}
          <button
            onclick={() => toggleLayer(layer.id)}
            class="w-full flex items-center gap-2 px-2 py-1.5 rounded text-[11px] transition-colors
                   {$visibleLayers.has(layer.id)
              ? 'text-slate-100 bg-slate-700/50'
              : 'text-slate-500 hover:text-slate-300 hover:bg-slate-700/30'}"
          >
            <span class="w-4 text-center text-xs">{layer.icon}</span>
            <span class="flex-1 text-left">{layer.label}</span>
            {#if $visibleLayers.has(layer.id)}
              <span class="text-sky-400 text-[10px]">on</span>
            {/if}
          </button>
        {/each}
      </div>
    </div>
  {/if}
</div>

<!-- Close on click outside -->
{#if expanded}
  <button
    class="fixed inset-0 z-40 cursor-default"
    onclick={() => expanded = false}
    aria-label="Close layers"
    tabindex="-1"
  ></button>
{/if}
