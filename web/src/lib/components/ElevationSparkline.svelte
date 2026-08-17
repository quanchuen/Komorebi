<script lang="ts">
  export interface ElevationSample {
    distanceM: number;
    elevationM: number;
  }

  interface Props {
    samples?: ElevationSample[];
  }

  let { samples = [] }: Props = $props();

  const width = 240;
  const profileBottom = 36;
  const effortBaseline = 49;
  const effortAmplitude = 8;
  const height = 59;

  let chart = $derived.by(() => {
    if (samples.length < 2) return null;

    const maxDistance = samples.at(-1)?.distanceM ?? 0;
    if (maxDistance <= 0) return null;
    const elevations = samples.map((sample) => sample.elevationM);
    const minElevation = Math.min(...elevations);
    const elevationRange = Math.max(...elevations) - minElevation || 1;
    const x = (distanceM: number) => (distanceM / maxDistance) * width;

    const elevationPoints = samples.map((sample) => ({
      x: x(sample.distanceM),
      y: profileBottom - 2 - ((sample.elevationM - minElevation) / elevationRange) * 30
    }));

    const rawGrades = samples.map((sample, index) => {
      if (index === 0) return 0;
      const distance = sample.distanceM - samples[index - 1].distanceM;
      return distance > 0
        ? ((sample.elevationM - samples[index - 1].elevationM) / distance) * 100
        : 0;
    });
    // A short moving average makes the derivative readable instead of
    // amplifying metre-scale DEM noise.
    const grades = rawGrades.map((_, index) => {
      const from = Math.max(0, index - 2);
      const to = Math.min(rawGrades.length, index + 3);
      return rawGrades.slice(from, to).reduce((sum, grade) => sum + grade, 0) / (to - from);
    });
    const effortPoints = samples.map((sample, index) => ({
      x: x(sample.distanceM),
      y: effortBaseline - (Math.max(-15, Math.min(15, grades[index])) / 15) * effortAmplitude
    }));
    const path = (points: Array<{ x: number; y: number }>) =>
      points
        .map(
          (point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(1)} ${point.y.toFixed(1)}`
        )
        .join(' ');

    const elevationPath = path(elevationPoints);
    return {
      elevationPath,
      areaPath: `${elevationPath} L ${width} ${profileBottom} L 0 ${profileBottom} Z`,
      effortPath: path(effortPoints),
      distanceLabel:
        maxDistance >= 1000
          ? `${(maxDistance / 1000).toFixed(1)} km`
          : `${Math.round(maxDistance)} m`
    };
  });
</script>

{#if chart}
  <div>
    <svg
      viewBox="0 0 {width} {height}"
      class="h-14 w-full"
      role="img"
      aria-label="Elevation area profile and grade effort trace over distance"
    >
      <path d={chart.areaPath} class="fill-accent/20" />
      <path
        d={chart.elevationPath}
        class="stroke-accent"
        fill="none"
        stroke-width="1.5"
        stroke-linejoin="round"
      />
      <line
        x1="0"
        y1={effortBaseline}
        x2={width}
        y2={effortBaseline}
        class="stroke-border"
        stroke-width="0.75"
      />
      <path
        d={chart.effortPath}
        class="stroke-warning"
        fill="none"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
    <div class="flex justify-between text-3xs text-text-subtle -mt-1">
      <span>0</span>
      <span>Effort = grade</span>
      <span>{chart.distanceLabel}</span>
    </div>
  </div>
{/if}
