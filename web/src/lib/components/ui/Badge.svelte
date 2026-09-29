<!-- web/src/lib/components/ui/Badge.svelte -->
<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    tone?: 'neutral' | 'accent';
    class?: string;
    children: Snippet;
  }

  let { tone = 'neutral', class: extraClass = '', children }: Props = $props();

  const tones: Record<string, string> = {
    neutral: 'bg-surface-overlay text-text-muted',
    accent: 'bg-accent/10 text-accent'
  };
</script>

<span
  class="inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-medium capitalize {tones[
    tone
  ]} {extraClass}"
>
  {@render children()}
</span>
