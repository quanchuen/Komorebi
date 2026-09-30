<!-- web/src/lib/components/explore/MenuPopover.svelte -->
<!-- Under the search pill's ☰. Frame M (Your riding) is not built yet, so the
     menu carries the two entries the disclaimer and the sheet promise:
     About (Before you ride) and Data sources. -->
<script lang="ts">
  import { menuOpen, dataSourcesOpen, firstRideDisclaimerRequest } from '$lib/stores/ui';
  import Icon from '$lib/components/ui/Icon.svelte';

  function about() {
    menuOpen.set(false);
    firstRideDisclaimerRequest.set({});
  }
  function sources() {
    menuOpen.set(false);
    dataSourcesOpen.set(true);
  }
  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') menuOpen.set(false);
  }
</script>

<svelte:window onkeydown={$menuOpen ? onkeydown : undefined} />

{#if $menuOpen}
  <div
    role="menu"
    aria-label="Menu"
    class="absolute z-40 w-56 p-2 bg-surface-base border border-line rounded-lg shadow-lg"
  >
    <button
      type="button"
      role="menuitem"
      onclick={about}
      class="w-full flex items-center gap-3 h-11 px-2 rounded-md text-sm text-text hover:bg-surface-tint
             focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
    >
      <Icon name="info" class="size-6 text-text-subtle" />
      <span class="flex-1 text-left">About · Before you ride</span>
    </button>
    <button
      type="button"
      role="menuitem"
      onclick={sources}
      class="w-full flex items-center gap-3 h-11 px-2 rounded-md text-sm text-text hover:bg-surface-tint
             focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
    >
      <Icon name="layers" class="size-6 text-text-subtle" />
      <span class="flex-1 text-left">Data sources</span>
    </button>
  </div>
{/if}
