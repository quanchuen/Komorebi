<!-- web/src/lib/components/ElevationProfile.svelte -->
<script lang="ts">
  interface Props {
    coordinates?: Array<[number, number, number?]>;
    widthPx?: number;
    heightPx?: number;
  }

  let { coordinates = [], widthPx = 400, heightPx = 80 }: Props = $props();

  interface ElevPoint { x: number; y: number; elevM: number; }

  let points = $derived((() => {
    const withEle = coordinates.filter((c) => c[2] !== undefined) as Array<[number, number, number]>;
    if (withEle.length < 2) return [] as ElevPoint[];
    const elevs = withEle.map((c) => c[2]);
    const minE = Math.min(...elevs);
    const maxE = Math.max(...elevs);
    const range = maxE - minE || 1;
    return withEle.map((c, i) => ({
      x: (i / (withEle.length - 1)) * widthPx,
      y: heightPx - ((c[2] - minE) / range) * (heightPx - 8),
      elevM: c[2]
    }));
  })());

  let pathD = $derived(points.length > 1
    ? points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
    : '');

  let areaD = $derived(pathD.length > 0
    ? `${pathD} L ${widthPx} ${heightPx} L 0 ${heightPx} Z`
    : '');
</script>

{#if points.length > 1}
  <svg
    viewBox="0 0 {widthPx} {heightPx}"
    width="100%"
    height={heightPx}
    class="overflow-visible"
    aria-label="Elevation profile"
    role="img"
  >
    <defs>
      <linearGradient id="elev-grad" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="#38BDF8" stop-opacity="0.5" />
        <stop offset="100%" stop-color="#38BDF8" stop-opacity="0.05" />
      </linearGradient>
    </defs>
    <path d={areaD} fill="url(#elev-grad)" />
    <path d={pathD} fill="none" stroke="#38BDF8" stroke-width="1.5" stroke-linejoin="round" />
  </svg>
{:else}
  <div class="text-slate-500 text-xs">No elevation data</div>
{/if}
