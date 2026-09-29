<!-- web/src/lib/components/ui/DifficultyBadge.svelte -->
<!-- Difficulty carries no colour: 1–4 filled bars plus a label. -->
<script lang="ts">
  import type { Difficulty } from '$lib/api/types';

  interface Props {
    difficulty: Difficulty;
    class?: string;
  }

  let { difficulty, class: extraClass = '' }: Props = $props();

  const LEVELS: Record<Difficulty, { level: number; label: string }> = {
    easy: { level: 1, label: 'Easy' },
    moderate: { level: 2, label: 'Moderate' },
    hard: { level: 3, label: 'Hard' },
    expert: { level: 4, label: 'Expert' }
  };

  let info = $derived(LEVELS[difficulty] ?? { level: 0, label: difficulty });
</script>

<span
  class="inline-flex items-center gap-1.5 rounded-badge border border-hairline bg-surface-tint
         px-1.5 py-0.5 text-2xs font-medium text-text-default {extraClass}"
  data-testid="difficulty-badge"
>
  <span class="flex items-center gap-0.5" aria-hidden="true">
    {#each [1, 2, 3, 4] as bar (bar)}
      <span
        class="h-2.5 w-1 rounded-full {bar <= info.level ? 'bg-meter-filled' : 'bg-meter-empty'}"
      ></span>
    {/each}
  </span>
  <span>{info.label}</span>
  <span class="sr-only">difficulty, level {info.level} of 4</span>
</span>
