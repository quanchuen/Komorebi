<!-- web/src/lib/components/ConditionSparkline.svelte -->
<script lang="ts">
  import type { RouteConditionSegment } from '$lib/api/types';
  import { conditionColor } from '$lib/utils/conditionColors';

  interface Props {
    segments?: RouteConditionSegment[];
    overlay?: 'shade' | 'wind' | 'rain';
  }

  let { segments = [], overlay = 'shade' }: Props = $props();

  const BAR_COUNT = 20;

  function getValue(seg: RouteConditionSegment): number {
    if (overlay === 'shade') return seg.shade;
    if (overlay === 'wind') return (seg.wind_benefit + 1) / 2; // normalize -1..1 → 0..1
    return seg.precip;
  }

  function sampleSegments(segs: RouteConditionSegment[], n: number): number[] {
    if (segs.length === 0) return Array(n).fill(0);
    return Array.from({ length: n }, (_, i) => {
      const idx = Math.floor((i / n) * segs.length);
      return getValue(segs[idx]);
    });
  }

  // Check if all values are effectively zero (no data)
  function isFlat(vals: number[]): boolean {
    const threshold = overlay === 'wind' ? 0.45 : 0.02; // wind is centered at 0.5
    const center = overlay === 'wind' ? 0.5 : 0;
    return vals.every((v) => Math.abs(v - center) < threshold);
  }

  let values = $derived(sampleSegments(segments, BAR_COUNT));
  let noData = $derived(segments.length === 0 || isFlat(values));
</script>

<div class="flex items-end gap-px h-4" aria-hidden="true">
  {#each values as v}
    {#if noData}
      <!-- Hollow/flat line for no data -->
      <div
        class="flex-1 rounded-sm border border-slate-700"
        style="height: 25%; background: transparent;"
      ></div>
    {:else}
      <div
        class="flex-1 rounded-sm"
        style="height: {Math.max(20, v * 100)}%; background: {conditionColor(overlay, overlay === 'wind' ? v * 2 - 1 : v)};"
      ></div>
    {/if}
  {/each}
</div>
