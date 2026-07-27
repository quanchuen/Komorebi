<!-- web/src/lib/components/FilterChips.svelte -->
<script lang="ts">
  import { discoveryFilters } from '$lib/stores/discovery';
  import type { Difficulty } from '$lib/api/types';

  const difficulties: { value: Difficulty; label: string }[] = [
    { value: 'easy', label: 'Easy' },
    { value: 'moderate', label: 'Moderate' },
    { value: 'hard', label: 'Hard' },
    { value: 'expert', label: 'Expert' }
  ];

  function toggleDifficulty(d: Difficulty) {
    discoveryFilters.update((f) => ({ ...f, difficulty: f.difficulty === d ? null : d }));
  }

  function toggleShade() {
    discoveryFilters.update((f) => ({ ...f, shade: !f.shade }));
  }

  function toggleGreenery() {
    discoveryFilters.update((f) => ({ ...f, greenery: !f.greenery }));
  }
</script>

<div class="flex flex-wrap gap-2">
  {#each difficulties as d}
    <button
      onclick={() => toggleDifficulty(d.value)}
      aria-pressed={$discoveryFilters.difficulty === d.value}
      class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
             {$discoveryFilters.difficulty === d.value
        ? 'bg-accent text-white border-transparent'
        : 'bg-surface-raised text-text-muted border-border hover:bg-surface-overlay'}"
    >
      {d.label}
    </button>
  {/each}

  <button
    onclick={toggleShade}
    aria-pressed={$discoveryFilters.shade}
    class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
           {$discoveryFilters.shade
      ? 'bg-blue-800 text-blue-100 border-transparent'
      : 'bg-surface-raised text-text-muted border-border hover:bg-surface-overlay'}"
  >
    Shade
  </button>

  <button
    onclick={toggleGreenery}
    aria-pressed={$discoveryFilters.greenery}
    class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
           {$discoveryFilters.greenery
      ? 'bg-green-800 text-green-100 border-transparent'
      : 'bg-surface-raised text-text-muted border-border hover:bg-surface-overlay'}"
  >
    Greenery
  </button>
</div>
