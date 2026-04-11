<!-- web/src/lib/components/DepartureTimePicker.svelte -->
<script lang="ts">
  import { departureAt } from '$lib/stores/map';

  // Convert ISO string to "YYYY-MM-DDTHH:MM" for datetime-local input
  function toInputValue(iso: string): string {
    return iso.slice(0, 16);
  }

  function fromInputValue(v: string): string {
    return new Date(v).toISOString();
  }

  let inputValue = $state(toInputValue($departureAt));

  function handleChange(e: Event) {
    const v = (e.target as HTMLInputElement).value;
    if (v) {
      inputValue = v;
      departureAt.set(fromInputValue(v));
    }
  }
</script>

<div class="flex flex-col gap-1">
  <label class="text-xs text-slate-400 font-medium uppercase tracking-wide" for="departure-time">
    Depart
  </label>
  <input
    id="departure-time"
    type="datetime-local"
    value={inputValue}
    onchange={handleChange}
    class="bg-slate-800 border border-slate-700 text-slate-100 text-sm rounded-lg px-3 py-2
           focus:outline-none focus:ring-2 focus:ring-sky-500 focus:border-transparent
           [color-scheme:dark]"
  />
</div>
