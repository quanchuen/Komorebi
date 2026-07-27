<!-- web/src/lib/components/ui/Panel.svelte -->
<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    /** Drop the default padding when the panel hosts its own scroll regions. */
    padded?: boolean;
    class?: string;
    children: Snippet;
  }

  let { padded = true, class: extraClass = '', children }: Props = $props();
</script>

<div class="rounded-xl border border-border bg-surface-raised {padded ? 'p-4' : ''} {extraClass}">
  {@render children()}
</div>
