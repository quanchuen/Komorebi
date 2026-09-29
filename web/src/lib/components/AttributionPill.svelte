<!-- web/src/lib/components/AttributionPill.svelte -->
<!-- Credits the sources whose data is visible now; grows with active layers.
     Opens the Data sources sheet. Mounted into MapLibre's bottom-right control
     stack by Map.svelte so it sits under the zoom controls. -->
<script lang="ts">
  import { visibleSources } from '$lib/stores/attribution';
  import { dataSourcesOpen } from '$lib/stores/ui';
  import Icon from './ui/Icon.svelte';

  let credits = $derived($visibleSources.map((s) => s.credit).join(' · '));
</script>

<!-- The button is the 44px hit box; the pill inside is the visual. -->
<button
  type="button"
  onclick={() => dataSourcesOpen.set(true)}
  aria-haspopup="dialog"
  aria-label="Data sources: {credits}"
  data-testid="attribution-pill"
  class="attribution group flex min-h-11 min-w-11 items-center justify-end
         focus:outline-none"
>
  <span
    class="flex items-center gap-1.5 rounded-control border border-line bg-surface-base/90
           py-1 pl-2 pr-1.5 text-left text-2xs text-text-subtle shadow-xs backdrop-blur
           group-hover:text-text-default group-focus-visible:ring-2 group-focus-visible:ring-focus"
  >
    <span>{credits}</span>
    <Icon name="info" class="size-4 shrink-0" />
  </span>
</button>

<style>
  /* Wraps rather than truncates: credits must stay readable on phones. */
  .attribution {
    max-width: min(28rem, calc(100vw - 1.5rem));
  }
</style>
