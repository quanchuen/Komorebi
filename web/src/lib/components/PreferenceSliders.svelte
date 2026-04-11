<!-- web/src/lib/components/PreferenceSliders.svelte -->
<script lang="ts">
  import { plannerPreferences } from '$lib/stores/planner';

  const sliders: { key: keyof { shade: number; greenery: number; wind: number }; label: string; color: string }[] = [
    { key: 'shade', label: 'Shade', color: 'accent-blue-500' },
    { key: 'greenery', label: 'Greenery', color: 'accent-green-500' },
    { key: 'wind', label: 'Wind avoid', color: 'accent-red-400' }
  ];

  function handleInput(key: 'shade' | 'greenery' | 'wind', e: Event) {
    const v = parseFloat((e.target as HTMLInputElement).value);
    plannerPreferences.update((p) => ({ ...p, [key]: v }));
  }
</script>

<div class="space-y-3">
  {#each sliders as s}
    <div class="space-y-1">
      <div class="flex justify-between text-xs text-slate-400">
        <span>{s.label}</span>
        <span>{Math.round($plannerPreferences[s.key] * 100)}%</span>
      </div>
      <input
        type="range"
        min="0"
        max="1"
        step="0.05"
        value={$plannerPreferences[s.key]}
        oninput={(e) => handleInput(s.key, e)}
        class="w-full h-1.5 bg-slate-700 rounded-full appearance-none cursor-pointer {s.color}"
      />
    </div>
  {/each}
</div>
