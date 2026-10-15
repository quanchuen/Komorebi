<!-- web/src/lib/components/explore/SearchPanel.svelte -->
<!-- Frame B: one panel under the search pill. Empty field: Plan a route,
     Describe a ride, SHOW ON MAP, RECENT. With text: PLACES (geocoder
     matches, see $lib/geocode) replace RECENT (Penpot mobile B). -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import VenueChips from '$lib/components/VenueChips.svelte';
  import SectionLabel from './SectionLabel.svelte';
  import { recentPlaces, savedRoutes, rememberPlace } from '$lib/stores/recents';
  import { featuredRoutes } from '$lib/stores/routesInView';
  import DifficultyBadge from '$lib/components/DifficultyBadge.svelte';
  import { km, climb } from '$lib/utils/format';
  import type { PlaceHit } from '$lib/geocode';

  interface Props {
    query: string;
    places: PlaceHit[];
    placesLoading?: boolean;
    describeAvailable?: boolean;
    onplan: () => void;
    ondescribe: () => void;
    onplace: (place: PlaceHit) => void;
    onsavedroute: (id: string) => void;
    onfeatured: (id: string) => void;
  }

  let {
    query,
    places,
    placesLoading = false,
    describeAvailable = true,
    onplan,
    ondescribe,
    onplace,
    onsavedroute,
    onfeatured
  }: Props = $props();

  let typing = $derived(query.trim().length > 0);

  // RECENT mixes recently searched places and saved routes, newest first.
  let recent = $derived(
    [...$recentPlaces, ...$savedRoutes].sort((a, b) => b.at - a.at).slice(0, 5)
  );

  function pick(place: PlaceHit) {
    rememberPlace(place);
    onplace(place);
  }
</script>

<div class="flex flex-col py-2">
  <button type="button" onclick={onplan} class="entry-row">
    <span class="icon-well"><Icon name="route" class="size-6" /></span>
    <span class="flex-1 min-w-0 text-left">
      <span class="block text-sm font-semibold text-text">Plan a route</span>
      <span class="block text-xs text-text-subtle truncate"
        >From your location to anywhere · A → B, multi-stop</span
      >
    </span>
    <Icon name="chevronRight" class="size-6 text-text-placeholder" />
  </button>
  <button
    type="button"
    onclick={ondescribe}
    disabled={!describeAvailable}
    aria-disabled={!describeAvailable}
    class="entry-row disabled:pointer-events-none"
  >
    <span class="icon-well"><Icon name="sparkle" class="size-6" /></span>
    <span class="flex-1 min-w-0 text-left">
      <span
        class="block text-sm font-semibold {describeAvailable
          ? 'text-text'
          : 'text-text-placeholder'}">Describe a ride</span
      >
      <span
        class="block text-xs truncate {describeAvailable
          ? 'text-text-subtle'
          : 'text-text-placeholder'}"
      >
        {describeAvailable
          ? '“Shady 30 km loop, back before noon”'
          : "Describing rides isn't available on this server"}
      </span>
    </span>
    {#if describeAvailable}
      <Icon name="chevronRight" class="size-6 text-text-placeholder" />
    {/if}
  </button>

  {#if !typing && $featuredRoutes.length > 0}
    <!-- House picks (routes.route.featured), decided 2026-10-01: listed here,
         never pinned in the viewport list and not shown on the resting map. -->
    <div class="mx-4 mt-2 h-px bg-hairline"></div>
    <SectionLabel text="Featured routes" class="px-4 pt-3.5 pb-1" />
    <ul class="flex flex-col px-2">
      {#each $featuredRoutes.slice(0, 5) as route (route.id)}
        <li>
          <button type="button" onclick={() => onfeatured(route.id)} class="recent-row">
            <Icon name="route" class="size-6 text-text-subtle" />
            <span class="flex-1 min-w-0 text-left">
              <span class="block text-sm font-medium text-text truncate">{route.name}</span>
              <span class="block text-xs text-text-subtle truncate"
                >{km(route.distanceM)} · {climb(route.elevationGainM)}</span
              >
            </span>
            <DifficultyBadge difficulty={route.difficulty} />
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  {#if typing}
    <div class="mx-4 mt-2 h-px bg-hairline"></div>
    <SectionLabel text="Places" class="px-4 pt-3.5 pb-1.5" />
    {#if places.length === 0}
      <div class="px-4 py-2 text-xs text-text-subtle">
        {placesLoading ? 'Searching…' : 'No places match yet — keep typing.'}
      </div>
    {:else}
      <ul class="flex flex-col px-2">
        {#each places as place (place.name + place.lat)}
          <li>
            <button type="button" onclick={() => pick(place)} class="recent-row">
              <Icon name="pin" class="size-6 text-text-subtle" />
              <span class="flex-1 min-w-0 text-left">
                <span class="block text-sm font-medium text-text truncate">{place.name}</span>
                <span class="block text-xs text-text-subtle truncate">{place.detail}</span>
              </span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}

  <div class="mx-4 mt-2 h-px bg-hairline"></div>
  <SectionLabel text="Show on map" class="px-4 pt-3.5 pb-2.5" />
  <div class="px-4 py-1.5">
    <VenueChips />
  </div>
  <p class="px-4 pt-2 text-xs text-text-subtle">Venues stay off the map until you pick a type.</p>

  {#if !typing && recent.length > 0}
    <div class="mx-4 mt-3 h-px bg-hairline"></div>
    <SectionLabel text="Recent" class="px-4 pt-3.5 pb-1" />
    <ul class="flex flex-col px-2">
      {#each recent as item (item.kind === 'place' ? 'p' + item.name : 'r' + item.id)}
        <li>
          {#if item.kind === 'place'}
            <button type="button" onclick={() => pick(item)} class="recent-row">
              <Icon name="history" class="size-6 text-text-subtle" />
              <span class="flex-1 min-w-0 text-left">
                <span class="block text-sm font-medium text-text truncate">{item.name}</span>
                <span class="block text-xs text-text-subtle truncate">{item.detail}</span>
              </span>
            </button>
          {:else}
            <button type="button" onclick={() => onsavedroute(item.id)} class="recent-row">
              <Icon name="bookmark" class="size-6 text-text-subtle" />
              <span class="flex-1 min-w-0 text-left">
                <span class="block text-sm font-medium text-text truncate">{item.name}</span>
                <span class="block text-xs text-text-subtle"
                  >Saved route · {km(item.distanceM, 0)}</span
                >
              </span>
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  /* Penpot "Row / Plan a route": 56px, 36px icon well, chevron; radius 8. */
  .entry-row {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 56px;
    margin: 0 8px;
    padding: 0 12px 0 8px;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--color-text);
  }
  .entry-row:hover:not(:disabled) {
    background: var(--color-surface-tint);
  }
  .entry-row:focus-visible,
  .recent-row:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }
  .icon-well {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: var(--radius-md);
    background: var(--color-surface-recessed);
    flex-shrink: 0;
  }
  /* Penpot "Recent / …": 44px rows. */
  .recent-row {
    display: flex;
    width: 100%;
    align-items: center;
    gap: 8px;
    min-height: 44px;
    padding: 2px 8px 2px 4px;
    border-radius: var(--radius-md);
    background: transparent;
  }
  .recent-row:hover {
    background: var(--color-surface-tint);
  }
</style>
