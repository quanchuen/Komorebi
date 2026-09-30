<!-- web/src/lib/components/explore/Legend.svelte -->
<!-- Frame C: the legend above the scrubber while shade or rain is showing —
     a 40×10 ramp bar and a label per active layer. -->
<script lang="ts">
  import { visibleLayers } from '$lib/stores/map';
</script>

{#if $visibleLayers.has('shadows') || $visibleLayers.has('rain-cells')}
  <div
    class="inline-flex items-center gap-3 h-8 px-2.5 rounded-md bg-surface-base border border-line shadow-xs text-xs text-text"
    role="img"
    aria-label="Legend: {[
      $visibleLayers.has('shadows') ? 'sun to shade' : '',
      $visibleLayers.has('rain-cells') ? 'rain, drizzle to heavy' : ''
    ]
      .filter(Boolean)
      .join('; ')}"
  >
    {#if $visibleLayers.has('shadows')}
      <span class="inline-flex items-center gap-1.5">
        <span class="w-10 h-2.5 rounded-xs ramp-shade"></span>
        Sun → shade
      </span>
    {/if}
    {#if $visibleLayers.has('rain-cells')}
      <span class="inline-flex items-center gap-1.5">
        <span class="w-10 h-2.5 rounded-xs ramp-rain"></span>
        Rain, drizzle → heavy
      </span>
    {/if}
  </div>
{/if}

<style>
  .ramp-shade {
    background: linear-gradient(
      90deg,
      var(--color-shade-sun),
      var(--color-shade-partial),
      var(--color-shade-mostly),
      var(--color-shade-deep)
    );
  }
  .ramp-rain {
    background: linear-gradient(
      90deg,
      var(--color-rain-drizzle),
      var(--color-rain-light),
      var(--color-rain-moderate),
      var(--color-rain-heavy)
    );
  }
</style>
