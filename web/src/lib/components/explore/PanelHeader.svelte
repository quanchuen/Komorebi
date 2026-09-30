<!-- web/src/lib/components/explore/PanelHeader.svelte -->
<!-- Flow-panel header (frames G, H, I): optional Back, a 17px title, an
     optional count badge, Close on the right. -->
<script lang="ts">
  import IconButton from '$lib/components/ui/IconButton.svelte';

  interface Props {
    title: string;
    count?: number | null;
    onback?: () => void;
    onclose: () => void;
  }

  let { title, count = null, onback, onclose }: Props = $props();
</script>

<div class="flex items-center gap-1 pl-2 pr-2 pt-2 pb-1 min-h-13">
  {#if onback}
    <IconButton icon="chevronLeft" label="Back" size={36} stroke={1.6} onclick={onback} />
  {/if}
  <h2 class="text-17 font-semibold text-text-strong {onback ? '' : 'pl-2'}">{title}</h2>
  {#if count !== null}
    <span
      class="ml-2 inline-flex items-center justify-center h-5.5 min-w-7 px-2 rounded-pill
             bg-surface-recessed text-xs font-semibold text-text-strong"
    >
      {count}
    </span>
  {/if}
  <span class="flex-1"></span>
  <IconButton icon="x" label="Close" size={36} stroke={1.6} onclick={onclose} />
</div>
