<!-- web/src/lib/components/ui/FoldGrabber.svelte -->
<!-- Fold-handle affordance ("V3" in the design canvas): a rigid grabber bar,
     dead-center of its strip. While the surrounding `group` control is hovered
     or keyboard-focused, an accent chevron fades in on the fold side — the
     direction the panel will move when pressed — sliding 4px into place. The
     chevron is absolutely positioned so the bar never shifts. -->
<script lang="ts">
  interface Props {
    direction?: 'up' | 'down' | 'left' | 'right';
    size?: 'sm' | 'lg';
    class?: string;
  }

  let { direction = 'down', size = 'lg', class: extraClass = '' }: Props = $props();

  const vertical = $derived(direction === 'left' || direction === 'right');
</script>

<span class="v3 {size} {direction} {vertical ? 'v' : 'h'} {extraClass}" aria-hidden="true">
  <span class="bar"></span>
  <span class="cv">
    {#if direction === 'up'}
      {#if size === 'lg'}
        <svg
          width="12"
          height="8"
          viewBox="0 0 12 8"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"><path d="M2 6l4-4 4 4" /></svg
        >
      {:else}
        <svg
          width="10"
          height="6"
          viewBox="0 0 10 6"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"><path d="M1.5 4.5L5 1l3.5 3.5" /></svg
        >
      {/if}
    {:else if direction === 'down'}
      {#if size === 'lg'}
        <svg
          width="12"
          height="8"
          viewBox="0 0 12 8"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"><path d="M2 2l4 4 4-4" /></svg
        >
      {:else}
        <svg
          width="10"
          height="6"
          viewBox="0 0 10 6"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"><path d="M1.5 1L5 4.5 8.5 1" /></svg
        >
      {/if}
    {:else if direction === 'right'}
      <svg
        width="8"
        height="12"
        viewBox="0 0 8 12"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"><path d="M2 2l4 4-4 4" /></svg
      >
    {:else}
      <svg
        width="8"
        height="12"
        viewBox="0 0 8 12"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"><path d="M6 2L2 6l4 4" /></svg
      >
    {/if}
  </span>
</span>

<style>
  .v3 {
    position: relative;
    display: inline-flex;
    flex-shrink: 0;
  }
  .bar {
    display: block;
    border-radius: 9999px;
    background: var(--color-border-strong);
  }
  .h.lg .bar {
    width: 40px;
    height: 4px;
  }
  .h.sm .bar {
    width: 24px;
    height: 3px;
  }
  .v .bar {
    width: 4px;
    height: 24px;
  }

  .cv {
    position: absolute;
    display: flex;
    color: var(--color-accent-strong);
    opacity: 0;
    transition:
      opacity 0.2s ease,
      transform 0.2s ease;
  }
  .up .cv {
    bottom: calc(100% + 2px);
    left: 50%;
    transform: translate(-50%, 4px);
  }
  .down .cv {
    top: calc(100% + 2px);
    left: 50%;
    transform: translate(-50%, -4px);
  }
  .right .cv {
    left: calc(100% + 2px);
    top: 50%;
    transform: translate(-4px, -50%);
  }
  .left .cv {
    right: calc(100% + 2px);
    top: 50%;
    transform: translate(4px, -50%);
  }

  :global(.group:where(:hover, :focus-visible)) .up.v3 .cv,
  :global(.group:where(:hover, :focus-visible)) .down.v3 .cv {
    opacity: 1;
    transform: translate(-50%, 0);
  }
  :global(.group:where(:hover, :focus-visible)) .right.v3 .cv,
  :global(.group:where(:hover, :focus-visible)) .left.v3 .cv {
    opacity: 1;
    transform: translate(0, -50%);
  }

  @media (prefers-reduced-motion: reduce) {
    .cv {
      transition: none;
    }
  }
</style>
