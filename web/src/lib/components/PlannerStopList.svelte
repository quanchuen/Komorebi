<!-- web/src/lib/components/PlannerStopList.svelte -->
<script lang="ts">
  import { plannerStops, plannerPlan } from '$lib/stores/planner';
  import { plans } from '$lib/api/client';

  async function removeStop(id: string) {
    const plan = $plannerPlan;
    plannerStops.update((stops) => stops.filter((s) => s.id !== id));
    if (plan) {
      const planStop = plan.stops.find((s) => s.resolvedName === id);
      if (planStop) {
        await plans.removeStop(plan.id, planStop.id).catch(() => {});
      }
    }
  }
</script>

<div class="space-y-1">
  {#each $plannerStops as stop, i (stop.id)}
    <div class="flex items-center gap-2 bg-slate-800 rounded-lg px-3 py-2">
      <div class="flex flex-col items-center gap-1 shrink-0">
        <div class="w-3 h-3 rounded-full {i === 0 ? 'bg-green-400' : i === $plannerStops.length - 1 ? 'bg-red-400' : 'bg-sky-400'}"></div>
        {#if i < $plannerStops.length - 1}
          <div class="w-0.5 h-3 bg-slate-700"></div>
        {/if}
      </div>
      <span class="flex-1 text-sm text-slate-300 truncate">{stop.label}</span>
      {#if $plannerStops.length > 2}
        <button
          onclick={() => removeStop(stop.id)}
          class="text-slate-600 hover:text-red-400 text-xs transition-colors"
          aria-label="Remove stop"
        >
          ✕
        </button>
      {/if}
    </div>
  {/each}
</div>
