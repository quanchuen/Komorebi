<!-- web/src/lib/components/VenueChips.svelte -->
<script lang="ts">
  import { visibleVenueTypes, type VenueType } from '$lib/stores/map';
  import { VENUE_ICON, VENUE_LABEL } from '$lib/utils/mapMarkers';
  import Icon from './ui/Icon.svelte';
  import { isMobile } from '$lib/stores/ui';

  // SHOW ON MAP (frame B). Venues stay off the map until the rider picks a
  // type (opt-in only). Desktop chips: 32px, radius 8, icon + label, xs
  // shadow. Mobile chips: 36px pills, label only. Each button is padded out
  // to a 44px hit box around the visible chip.
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

<div
  class="flex flex-nowrap items-center gap-1.5 overflow-x-auto md:flex-wrap md:overflow-visible -my-1.5"
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
      class="group/chip shrink-0 inline-flex items-center min-h-11 py-1.5 focus:outline-none"
    >
      <span
        class="inline-flex items-center gap-1.5 border transition-colors
               group-focus-visible/chip:ring-2 group-focus-visible/chip:ring-focus
               {$isMobile
          ? 'h-9 px-3.5 rounded-pill text-sm'
          : 'h-8 pl-2.5 pr-2.5 rounded-md shadow-xs text-13'}
               font-medium
               {on
          ? 'bg-primary text-on-primary border-primary'
          : 'bg-surface-base text-text border-line hover:bg-surface-tint'}"
      >
        {#if !$isMobile}
          <Icon name={VENUE_ICON[type]} class="size-6" />
        {/if}
        {VENUE_LABEL[type]}
      </span>
    </button>
  {/each}
</div>
