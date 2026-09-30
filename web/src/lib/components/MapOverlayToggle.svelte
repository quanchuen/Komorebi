<!-- web/src/lib/components/MapOverlayToggle.svelte -->
<script lang="ts">
  import { activeOverlay } from '$lib/stores/map';
  import type { OverlayType } from '$lib/stores/map';

  const overlays: { id: Exclude<OverlayType, null>; label: string; activeClass: string }[] = [
    { id: 'shade', label: 'Shade', activeClass: 'bg-shade text-on-primary' },
    { id: 'wind', label: 'Wind', activeClass: 'bg-wind text-on-primary' },
    { id: 'rain', label: 'Rain', activeClass: 'bg-rain text-on-primary' }
  ];

  function toggle(id: Exclude<OverlayType, null>) {
    activeOverlay.update((cur) => (cur === id ? null : id));
  }
</script>

<div class="flex gap-2">
  {#each overlays as ov}
    <button
      onclick={() => toggle(ov.id)}
      aria-pressed={$activeOverlay === ov.id}
      class="px-3 py-1.5 rounded-full text-xs font-semibold border transition-colors
             {$activeOverlay === ov.id
        ? ov.activeClass + ' border-transparent'
        : 'bg-surface-base text-text-subtle border-line hover:bg-surface-tint'}"
    >
      {ov.label}
    </button>
  {/each}
</div>
