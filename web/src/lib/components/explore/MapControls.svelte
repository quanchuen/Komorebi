<!-- web/src/lib/components/explore/MapControls.svelte -->
<!-- Penpot "IconButton / Layers" and "Zoom+Locate". Desktop: Layers alone
     top-right (40px, radius 10); the 40×120 zoom + locate stack bottom-right
     above the attribution. Mobile: Layers and Locate as two 40px buttons
     under the search pill; zoom is a gesture. Each visible 40px control sits
     inside a 44px hit box (the stack's rows are 44px each). -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import { zoomIn, zoomOut, locate, locateStatus } from '$lib/stores/map';
  import { layersOpen, isMobile } from '$lib/stores/ui';

  let locateLabel = $derived(
    $locateStatus === 'denied'
      ? 'Show my location (permission denied)'
      : $locateStatus === 'unavailable'
        ? 'Show my location (unavailable)'
        : 'Show my location'
  );
</script>

{#if $isMobile}
  <div class="absolute right-2.5 top-26.5 z-20 flex flex-col gap-1">
    <IconButton
      icon="layers"
      label="Map layers"
      size={40}
      variant={$layersOpen ? 'primary' : 'raised'}
      aria-expanded={$layersOpen}
      aria-haspopup="dialog"
      onclick={() => layersOpen.update((v) => !v)}
    />
    <IconButton icon="locate" label={locateLabel} size={40} variant="raised" onclick={locate} />
  </div>
{:else}
  <div class="absolute right-3.5 top-3.5 z-20">
    <IconButton
      icon="layers"
      label="Map layers"
      size={40}
      variant={$layersOpen ? 'primary' : 'raised'}
      aria-expanded={$layersOpen}
      aria-haspopup="dialog"
      onclick={() => layersOpen.update((v) => !v)}
    />
  </div>
  <div
    class="absolute right-3.5 bottom-14 z-20 w-11 flex flex-col"
    role="group"
    aria-label="Zoom and locate"
  >
    <!-- The visible 40×120 stack is a decoration behind three 44×44 buttons,
         so no button overflows its parent. -->
    <span
      class="absolute inset-y-0 left-0.5 right-0.5 rounded-control bg-surface-base border border-line shadow-xs"
      aria-hidden="true"
    ></span>
    <button type="button" aria-label="Zoom in" onclick={zoomIn} class="stack-btn">
      <span class="stack-face rounded-t-control"><Icon name="plus" class="size-6" /></span>
    </button>
    <span class="relative mx-2.5 h-px bg-hairline" aria-hidden="true"></span>
    <button type="button" aria-label="Zoom out" onclick={zoomOut} class="stack-btn">
      <span class="stack-face"><Icon name="minus" class="size-6" /></span>
    </button>
    <span class="relative mx-2.5 h-px bg-hairline" aria-hidden="true"></span>
    <button type="button" aria-label={locateLabel} onclick={locate} class="stack-btn">
      <span class="stack-face rounded-b-control"><Icon name="locate" class="size-6" /></span>
    </button>
  </div>
{/if}

<style>
  /* Each row is a 44×44 button; the 40px face inside it takes the hover. */
  .stack-btn {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    padding: 0;
    background: transparent;
    color: var(--color-text);
  }
  .stack-face {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
  }
  .stack-btn:hover .stack-face {
    background: var(--color-surface-tint);
  }
  .stack-btn:focus-visible {
    outline: none;
  }
  .stack-btn:focus-visible .stack-face {
    box-shadow: inset 0 0 0 2px var(--color-focus);
  }
</style>
