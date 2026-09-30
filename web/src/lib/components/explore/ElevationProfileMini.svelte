<!-- web/src/lib/components/explore/ElevationProfileMini.svelte -->
<!-- The route card's elevation strip (frame D): a 6% text-coloured area
     under a 1.5px chart line on a 1px baseline, with the highest point
     labelled when a waypoint sits near it. -->
<script lang="ts">
  import { metresBetween } from '$lib/utils/mapMarkers';

  interface Props {
    coordinates: Array<[number, number] | [number, number, number]>;
    waypoints?: { name: string; lon: number; lat: number }[];
  }
  let { coordinates, waypoints = [] }: Props = $props();

  const W = 368;
  const H = 56;
  const TOP = 19;

  let chart = $derived.by(() => {
    let dist = 0;
    const samples: { d: number; e: number; lon: number; lat: number }[] = [];
    coordinates.forEach((c, i) => {
      if (i > 0)
        dist += metresBetween([coordinates[i - 1][0], coordinates[i - 1][1]], [c[0], c[1]]);
      if (c[2] !== undefined) samples.push({ d: dist, e: c[2], lon: c[0], lat: c[1] });
    });
    if (samples.length < 2 || dist <= 0) return null;
    const es = samples.map((s) => s.e);
    const min = Math.min(...es);
    const range = Math.max(...es) - min || 1;
    const pts = samples.map((s) => ({
      x: (s.d / dist) * W,
      y: H - 1 - ((s.e - min) / range) * (H - 1 - TOP)
    }));
    const line = pts
      .map((p, i) => `${i ? 'L' : 'M'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`)
      .join(' ');
    const peakIndex = es.indexOf(Math.max(...es));
    const peak = samples[peakIndex];
    let label: { x: number; text: string } | null = null;
    for (const wp of waypoints) {
      if (metresBetween([wp.lon, wp.lat], [peak.lon, peak.lat]) < 400) {
        label = { x: pts[peakIndex].x, text: wp.name };
        break;
      }
    }
    return { line, area: `${line} L ${W} ${H - 1} L 0 ${H - 1} Z`, label };
  });
</script>

{#if chart}
  <div class="relative">
    {#if chart.label}
      <span
        class="absolute top-0 text-3xs text-text-subtle whitespace-nowrap -translate-x-1/2"
        style="left: {Math.max(8, Math.min(92, (chart.label.x / W) * 100))}%"
        >{chart.label.text}</span
      >
    {/if}
    <svg viewBox="0 0 {W} {H}" class="w-full h-14" role="img" aria-label="Elevation profile">
      <path d={chart.area} class="fill-text/6" />
      <path
        d={chart.line}
        class="stroke-chart-line"
        fill="none"
        stroke-width="1.5"
        stroke-linejoin="round"
      />
      <line x1="0" y1={H - 0.5} x2={W} y2={H - 0.5} class="stroke-line" stroke-width="1" />
    </svg>
  </div>
{/if}
