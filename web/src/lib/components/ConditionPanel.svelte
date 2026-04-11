<!-- web/src/lib/components/ConditionPanel.svelte -->
<script lang="ts">
  import type { RouteConditionSegment } from '$lib/api/types';
  import ConditionSparkline from './ConditionSparkline.svelte';

  interface Props {
    segments?: RouteConditionSegment[];
  }

  let { segments = [] }: Props = $props();

  let avgShade = $derived(segments.length > 0
    ? segments.reduce((s, r) => s + r.shade, 0) / segments.length
    : null);
  let avgWind = $derived(segments.length > 0
    ? segments.reduce((s, r) => s + r.windBenefit, 0) / segments.length
    : null);
  let avgPrecip = $derived(segments.length > 0
    ? segments.reduce((s, r) => s + r.precip, 0) / segments.length
    : null);

  function shadeLabel(v: number | null): string {
    if (v === null) return '—';
    if (v > 0.7) return 'Well shaded';
    if (v > 0.4) return 'Partial shade';
    return 'Exposed';
  }

  function windLabel(v: number | null): string {
    if (v === null) return '—';
    if (v > 0.3) return 'Tailwind';
    if (v < -0.3) return 'Headwind';
    return 'Cross / calm';
  }

  function precipLabel(v: number | null): string {
    if (v === null) return '—';
    if (v < 0.1) return 'Dry';
    if (v < 0.4) return 'Light rain';
    return 'Rain expected';
  }
</script>

<div class="space-y-4">
  {#each [
    { label: 'Shade', overlay: 'shade' as const, summary: shadeLabel(avgShade) },
    { label: 'Wind', overlay: 'wind' as const, summary: windLabel(avgWind) },
    { label: 'Rain', overlay: 'rain' as const, summary: precipLabel(avgPrecip) }
  ] as item}
    <div>
      <div class="flex items-center justify-between mb-1">
        <span class="text-sm font-medium text-slate-300">{item.label}</span>
        <span class="text-xs text-slate-400">{item.summary}</span>
      </div>
      <ConditionSparkline {segments} overlay={item.overlay} />
    </div>
  {/each}
</div>
