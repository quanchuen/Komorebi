<!-- web/src/lib/components/explore/RoutesPeek.svelte -->
<!-- Frame A's "RoutesPeek": "3 curated routes in view" over the first
     names, with a chevron that opens the routes list (frame I). Desktop:
     400×52 bottom-left. Mobile: the resting sheet's title, with the time
     chip below it (frame A mobile). Frame N: an empty viewport says so and
     offers Zoom out. -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import TimeChip from './TimeChip.svelte';
  import Sheet from './Sheet.svelte';
  import { routesInView, routesInViewLoading, routesInViewError } from '$lib/stores/routesInView';
  import { zoomOut } from '$lib/stores/map';
  import { isMobile } from '$lib/stores/ui';

  interface Props {
    onopen: () => void;
    routeWind?: string | null;
  }
  let { onopen, routeWind = null }: Props = $props();

  let count = $derived($routesInView.length);
  let title = $derived(
    $routesInViewError
      ? "Can't reach Komorebi"
      : count === 0
        ? $routesInViewLoading
          ? 'Looking for curated routes…'
          : 'No curated routes here'
        : `${count} curated route${count === 1 ? '' : 's'} in view`
  );
  let names = $derived.by(() => {
    if ($routesInViewError) return 'Showing the map only';
    if (count === 0) return $routesInViewLoading ? '' : 'Zoom out to find some';
    const shown = $routesInView.slice(0, 3).map((r) => r.name);
    return shown.join($isMobile ? ' · ' : ', ') + (count > 3 ? '…' : '');
  });
</script>

{#if $isMobile}
  <Sheet label="Routes in view">
    <div class="px-5 pt-2 pb-4">
      <button
        type="button"
        onclick={count === 0 && !$routesInViewLoading && !$routesInViewError ? zoomOut : onopen}
        class="block w-full text-left min-h-11 rounded-md focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <span class="block text-base font-semibold text-text-strong leading-snug">{title}</span>
        <span class="block text-13 text-text-subtle mt-0.5 truncate">{names}</span>
      </button>
      <div class="mt-1.5 -mb-2">
        <TimeChip {routeWind} />
      </div>
    </div>
  </Sheet>
{:else}
  <button
    type="button"
    onclick={count === 0 && !$routesInViewLoading && !$routesInViewError ? zoomOut : onopen}
    class="flex items-center gap-1 w-100 h-13 pl-4 pr-1.5 rounded-lg bg-surface-base border border-line shadow-lg
           text-left hover:bg-surface-tint transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
  >
    <Icon name="route" class="size-6 text-text" />
    <span class="flex-1 min-w-0 pl-1">
      <span class="block text-sm font-semibold text-text leading-tight">{title}</span>
      <span class="block text-xs text-text-subtle truncate">{names}</span>
    </span>
    {#if count === 0 && !$routesInViewLoading && !$routesInViewError}
      <span class="text-13 font-medium text-text px-2">Zoom out</span>
    {:else}
      <Icon name="chevronUp" class="size-6 text-text-subtle mr-1" />
    {/if}
  </button>
{/if}
