<!-- web/src/lib/components/explore/SearchPill.svelte -->
<!-- Penpot "Search" (frames A/B/D): desktop 400×44, radius 10 — wordmark ·
     separator · search icon · field · ☰. Mobile 44 tall, radius 12 — search
     icon · field · a 32px circle for the menu. Focusing the field opens the
     search panel (frame B); the ring darkens to `focus`. -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import { explorePanel, menuOpen, isMobile } from '$lib/stores/ui';

  interface Props {
    value: string;
    placeholder?: string;
    /** The rider clicked or tapped the field. */
    onclick?: () => void;
    oninput?: (value: string) => void;
    onsubmit?: () => void;
    oncancel?: () => void;
  }

  let {
    value = $bindable(''),
    placeholder = 'Search places, or describe a ride',
    onclick,
    oninput,
    onsubmit,
    oncancel
  }: Props = $props();

  let input = $state<HTMLInputElement | null>(null);
  export function focus() {
    input?.focus();
  }
  export function blur() {
    input?.blur();
  }

  let focused = $derived($explorePanel === 'search');
</script>

<div class="flex items-center gap-2 w-full">
  <form
    class="flex-1 flex items-center h-11 bg-surface-base border shadow-lg transition-colors
           {focused ? 'border-focus pill-focused' : 'border-line'}
           {$isMobile ? 'rounded-lg' : 'rounded-control'}"
    role="search"
    onsubmit={(e) => {
      e.preventDefault();
      onsubmit?.();
    }}
  >
    {#if !$isMobile}
      <span class="pl-3.5 pr-3 text-sm font-bold tracking-tight text-text-strong select-none"
        >komorebi</span
      >
      <span class="w-px h-5 bg-line" aria-hidden="true"></span>
    {/if}
    <label class="flex-1 min-w-0 flex items-center gap-0.5 h-full cursor-text pl-2.5">
      <Icon name="search" class="size-6 text-text-subtle" />
      <span class="sr-only">Search places, or describe a ride</span>
      <input
        bind:this={input}
        bind:value
        type="search"
        autocomplete="off"
        enterkeyhint="search"
        {placeholder}
        onclick={() => onclick?.()}
        oninput={() => oninput?.(value)}
        class="flex-1 min-w-0 h-11 -my-px bg-transparent border-0 outline-none pl-0.5 pr-2
               text-15 md:text-sm text-text
               placeholder:text-text-placeholder
               [&::-webkit-search-cancel-button]:appearance-none"
      />
    </label>
    {#if $isMobile && focused}
      <!-- The keyboard is up; Cancel is the way back (Penpot mobile B). -->
      <button
        type="button"
        onclick={() => oncancel?.()}
        class="shrink-0 h-11 px-3 text-15 font-medium text-text-strong">Cancel</button
      >
    {:else}
      <button
        type="button"
        aria-label="Menu"
        aria-haspopup="menu"
        aria-expanded={$menuOpen}
        onclick={() => menuOpen.update((v) => !v)}
        class="group/menu shrink-0 inline-flex items-center justify-center size-11 focus:outline-none"
      >
        {#if $isMobile}
          <span
            class="inline-flex items-center justify-center size-8 rounded-pill bg-surface-recessed
                   text-text group-focus-visible/menu:ring-2 group-focus-visible/menu:ring-focus"
          >
            <Icon name="menu" class="size-5" />
          </span>
        {:else}
          <Icon
            name="menu"
            class="size-6 text-text rounded-sm group-focus-visible/menu:ring-2 group-focus-visible/menu:ring-focus"
          />
        {/if}
      </button>
    {/if}
  </form>
</div>

<style>
  /* Penpot draws the focused pill with a 1.5px focus-coloured ring. */
  .pill-focused {
    border-width: 1.5px;
  }
</style>
