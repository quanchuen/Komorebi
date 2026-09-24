<!-- web/src/lib/components/MapOverlayToggle.svelte -->
<script lang="ts">
  import { activeOverlay } from '$lib/stores/map';
  import type { OverlayType } from '$lib/stores/map';

  const overlays: { id: Exclude<OverlayType, null>; label: string; activeClass: string }[] = [
    { id: 'shade', label: 'Shade', activeClass: 'bg-shade text-on-accent' },
    { id: 'wind', label: 'Wind', activeClass: 'bg-wind text-on-accent' },
    { id: 'rain', label: 'Rain', activeClass: 'bg-rain text-on-accent' }
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
        : 'bg-surface-raised text-text-muted border-border hover:bg-surface-overlay'}"
    >
      {ov.label}
    </button>
  {/each}
</div>
