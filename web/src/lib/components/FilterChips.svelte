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
      class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
             {$discoveryFilters.difficulty === d.value
        ? 'bg-sky-600 text-white border-transparent'
        : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
    >
      {d.label}
    </button>
  {/each}

  <button
    onclick={toggleShade}
    class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
           {$discoveryFilters.shade
      ? 'bg-blue-800 text-blue-100 border-transparent'
      : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
  >
    Shade
  </button>

  <button
    onclick={toggleGreenery}
    class="px-3 py-1 rounded-full text-xs font-semibold border transition-colors
           {$discoveryFilters.greenery
      ? 'bg-green-800 text-green-100 border-transparent'
      : 'bg-slate-800 text-slate-400 border-slate-700 hover:bg-slate-700'}"
  >
    Greenery
  </button>
</div>
