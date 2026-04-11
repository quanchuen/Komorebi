<!-- web/src/lib/components/MapOverlayToggle.svelte -->
<script lang="ts">
  import { activeOverlay } from '$lib/stores/map';
  import type { OverlayType } from '$lib/stores/map';

  const overlays: { id: Exclude<OverlayType, null>; label: string; activeClass: string }[] = [
    { id: 'shade', label: 'Shade', activeClass: 'bg-blue-800 text-blue-100' },
    { id: 'wind', label: 'Wind', activeClass: 'bg-green-800 text-green-100' },
    { id: 'rain', label: 'Rain', activeClass: 'bg-purple-800 text-purple-100' }
  ];

  function toggle(id: Exclude<OverlayType, null>) {
    activeOverlay.update((cur) => (cur === id ? null : id));
  }
</script>

<div class="flex gap-2">
  {#each overlays as ov}
    <button
      onclick={() => toggle(ov.id)}
      class="px-3 py-1.5 rounded-full text-xs font-semibold border transition-colors
             {$activeOverlay === ov.id
        ? ov.activeClass + ' border-transparent'
        : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
    >
      {ov.label}
    </button>
  {/each}
</div>
