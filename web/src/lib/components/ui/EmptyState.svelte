<!-- web/src/lib/components/ui/EmptyState.svelte -->
<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    message?: string;
    children?: Snippet;
  }

  let { message = 'Nothing here yet.', children }: Props = $props();
</script>

<div class="py-4 text-center text-sm text-text-subtle">
  {#if children}
    {@render children()}
  {:else}
    {message}
  {/if}
</div>
