<!-- web/src/lib/components/DifficultyBadge.svelte -->
<script lang="ts">
  import type { Difficulty } from '$lib/api/types';

  // Difficulty carries no colour (spec "Difficulty — no colour"): a neutral
  // badge with 1–4 filled bars and a label, so every hue stays free for
  // map data. Filled bar = text.default, empty bar = line-strong.
  const LEVELS: Record<Difficulty, { label: string; bars: 1 | 2 | 3 | 4 }> = {
    easy: { label: 'Easy', bars: 1 },
    moderate: { label: 'Moderate', bars: 2 },
    hard: { label: 'Hard', bars: 3 },
    expert: { label: 'Expert', bars: 4 }
  };

  interface Props {
    difficulty: Difficulty;
    class?: string;
  }

  let { difficulty, class: extraClass = '' }: Props = $props();

  let level = $derived(LEVELS[difficulty] ?? LEVELS.easy);
</script>

<span
  role="img"
  aria-label="Difficulty: {level.label}, {level.bars} of 4"
  class="inline-flex items-center gap-1.5 rounded-sm border border-hairline bg-surface-tint
         px-2 py-0.5 text-2xs font-medium text-text {extraClass}"
>
  <span class="flex items-end gap-px" aria-hidden="true">
    {#each [1, 2, 3, 4] as bar (bar)}
      <span
        class="w-1 rounded-xs {bar <= level.bars ? 'bg-text' : 'bg-line-strong'}"
        style="height: {4 + bar * 2}px"
      ></span>
    {/each}
  </span>
  {level.label}
</span>
