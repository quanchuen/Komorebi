<!-- web/src/lib/components/VenueChips.svelte -->
<script lang="ts">
  import { visibleVenueTypes, type VenueType } from '$lib/stores/map';
  import { iconSvg, VENUE_ICON, VENUE_LABEL } from '$lib/utils/mapMarkers';

  // Venues stay off the map until the rider picks a type (opt-in only).
  const types: VenueType[] = ['konbini', 'cafe', 'water', 'repair'];

  function toggle(type: VenueType) {
    visibleVenueTypes.update((set) => {
      const next = new Set(set);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
  }
</script>

<!-- One scrolling row on narrow screens; the pill and zoom stack keep the corners. -->
<div
  class="flex flex-nowrap gap-2 overflow-x-auto pb-1 md:flex-wrap md:overflow-visible md:pb-0"
  role="group"
  aria-label="Show on map"
  data-scroll="true"
>
  {#each types as type (type)}
    {@const on = $visibleVenueTypes.has(type)}
    <button
      type="button"
      onclick={() => toggle(type)}
      aria-pressed={on}
      class="shrink-0 inline-flex items-center gap-1.5 min-h-11 min-w-11 px-3 rounded-full border text-xs font-medium
             transition-colors shadow-xs
             {on
        ? 'bg-primary text-on-primary border-primary'
        : 'bg-surface-base text-text border-line hover:bg-surface-tint'}"
    >
      <span class="size-4 shrink-0" aria-hidden="true">
        <!-- eslint-disable-next-line svelte/no-at-html-tags -- static SVG from mapMarkers -->
        {@html iconSvg(VENUE_ICON[type], 2)}
      </span>
      {VENUE_LABEL[type]}
    </button>
  {/each}
</div>
