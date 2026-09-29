<!-- web/src/lib/components/TaskInput.svelte -->
<script lang="ts">
  import { plannerTaskInput, plannerPlan } from '$lib/stores/planner';
  import { plans } from '$lib/api/client';

  let adding = $state(false);

  async function addTask() {
    const desc = $plannerTaskInput.trim();
    if (!desc || !$plannerPlan) return;

    const hashMatch = desc.match(/#(\S+)/);
    const hashtag = hashMatch ? `#${hashMatch[1]}` : undefined;

    adding = true;
    try {
      await plans.addTask($plannerPlan.id, desc, hashtag);
      plannerTaskInput.set('');
    } finally {
      adding = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') addTask();
  }

  function handleInput(e: Event) {
    plannerTaskInput.set((e.target as HTMLInputElement).value);
  }
</script>

<div class="flex gap-2">
  <input
    type="text"
    placeholder="Add task: coffee at #cafe"
    value={$plannerTaskInput}
    oninput={handleInput}
    onkeydown={handleKeydown}
    class="flex-1 bg-surface-base border border-line text-text-default text-sm rounded-lg
           px-3 py-2 focus:outline-none focus:ring-2 focus:ring-focus placeholder-text-subtle"
  />
  <button
    onclick={addTask}
    disabled={adding || !$plannerPlan}
    class="bg-primary hover:bg-primary-hover disabled:opacity-40 text-on-primary text-sm font-semibold
           rounded-lg px-3 py-2 transition-colors"
  >
    Add
  </button>
</div>
