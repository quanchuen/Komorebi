<!-- web/src/lib/components/WeatherTimeline.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import {
    departureAt,
    mapBounds,
    timelineLayer,
    visibleLayers,
    activeOverlay,
    type TimelineLayer
  } from '$lib/stores/map';
  import { weatherTimelineCollapsed as collapsed } from '$lib/stores/ui';
  import FoldGrabber from './ui/FoldGrabber.svelte';
  import { creditsWeather } from '$lib/stores/attribution';

  // The strip shows forecast weather; credit it in the attribution pill.
  creditsWeather();

  function toggleCollapsed() {
    collapsed.update((c) => !c);
  }

  // The relocated layer control: an exclusive environment lens. Weather shows
  // the rain radar + rain route coloring, Shade the building-shadow layer,
  // Sun the sun-exposure route coloring — all scrubbed by departure time.
  const lenses: { id: TimelineLayer; label: string }[] = [
    { id: 'weather', label: 'Weather' },
    { id: 'shade', label: 'Shade' },
    { id: 'sun', label: 'Sun' }
  ];

  function selectLens(id: TimelineLayer) {
    timelineLayer.set(id);
    visibleLayers.update((set) => {
      const next = new Set(set);
      next.delete('rain-cells');
      next.delete('shadows');
      if (id === 'weather') next.add('rain-cells');
      if (id === 'shade') next.add('shadows');
      return next;
    });
    activeOverlay.set(id === 'weather' ? 'rain' : id === 'sun' ? 'shade' : null);
  }

  const lensLabel = $derived(lenses.find((l) => l.id === $timelineLayer)?.label ?? 'Weather');

  interface HourSlot {
    hour: string;
    time: Date;
    temp: number;
    windSpeed: number;
    windDir: number;
    precip: number;
    isSelected: boolean;
  }

  let slots = $state<HourSlot[]>([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let scrollContainer: HTMLDivElement;
  let lastFetchKey = '';

  // Scrubber: drag left/right across the next 24h; the map's shadow and rain
  // layers follow departureAt live.
  const SCRUB_STEP_MIN = 10;
  const SCRUB_SPAN_MIN = 24 * 60 - SCRUB_STEP_MIN;

  function currentHourFloor(): Date {
    const d = new Date();
    d.setMinutes(0, 0, 0);
    return d;
  }
  const timelineStart = currentHourFloor();

  const scrubValue = $derived.by(() => {
    const offsetMin = (new Date($departureAt).getTime() - timelineStart.getTime()) / 60000;
    const snapped = Math.round(offsetMin / SCRUB_STEP_MIN) * SCRUB_STEP_MIN;
    return Math.max(0, Math.min(SCRUB_SPAN_MIN, snapped));
  });

  const scrubLabel = $derived.by(() => {
    const t = new Date($departureAt);
    const hm = t.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
    return t.getDate() === new Date().getDate()
      ? hm
      : `${t.toLocaleDateString('en-US', { weekday: 'short' })} ${hm}`;
  });

  function handleScrub(e: Event) {
    const v = Number((e.currentTarget as HTMLInputElement).value);
    const t = new Date(timelineStart.getTime() + v * 60000);
    departureAt.set(t.toISOString());
    slots = slots.map((s) => ({
      ...s,
      isSelected: s.time.getHours() === t.getHours() && s.time.getDate() === t.getDate()
    }));
  }

  function generateSlots(): HourSlot[] {
    const now = new Date();
    now.setMinutes(0, 0, 0);
    const selected = new Date($departureAt);

    return Array.from({ length: 24 }, (_, i) => {
      const t = new Date(now.getTime() + i * 3600000);
      return {
        hour: t.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false }),
        time: t,
        temp: 0,
        windSpeed: 0,
        windDir: 0,
        precip: 0,
        isSelected: t.getHours() === selected.getHours() && t.getDate() === selected.getDate()
      };
    });
  }

  async function fetchWeather() {
    const bounds = $mapBounds;
    if (!bounds) return;

    const centerLat = ((bounds.minLat + bounds.maxLat) / 2).toFixed(2);
    const centerLon = ((bounds.minLon + bounds.maxLon) / 2).toFixed(2);
    const fetchKey = `${centerLat},${centerLon}`;

    // Skip if same location (user just panned slightly)
    if (fetchKey === lastFetchKey) return;
    lastFetchKey = fetchKey;

    loading = true;
    error = null;

    try {
      // Single fetch for current hour — use it for the whole timeline
      // (weather doesn't change much across 24h at the same location for display purposes)
      const now = new Date();
      now.setMinutes(0, 0, 0);
      const res = await fetch(
        `/api/v1/weather/point?lat=${centerLat}&lon=${centerLon}&at=${now.toISOString()}`
      );

      const baseSlots = generateSlots();

      if (res.ok) {
        const data = await res.json();
        // Apply current weather to all slots as baseline, fetch a few key hours
        const baseTemp = Math.round(data.temperature_c ?? 0);
        const baseWind = Math.round((data.wind_speed_ms ?? 0) * 10) / 10;
        const baseWindDir = data.wind_bearing_deg ?? 0;
        const basePrecip = data.precip_intensity_mmh ?? 0;

        // Fetch a few spread-out hours for variation (0, 6, 12, 18)
        const keyHours = [0, 6, 12, 18].map((offset) => {
          const t = new Date(now.getTime() + offset * 3600000);
          return { offset, time: t };
        });

        const keyData = await Promise.all(
          keyHours.map(async (kh) => {
            try {
              const r = await fetch(
                `/api/v1/weather/point?lat=${centerLat}&lon=${centerLon}&at=${kh.time.toISOString()}`
              );
              if (r.ok) return { offset: kh.offset, data: await r.json() };
            } catch {}
            return null;
          })
        );

        // Build a lookup of known hours
        const hourMap = new Map<number, any>();
        hourMap.set(0, data);
        for (const kd of keyData) {
          if (kd) hourMap.set(kd.offset, kd.data);
        }

        slots = baseSlots.map((slot, i) => {
          // Find nearest known hour
          const known = hourMap.get(i) ?? hourMap.get(Math.floor(i / 6) * 6) ?? data;
          return {
            ...slot,
            temp: Math.round(known.temperature_c ?? baseTemp),
            windSpeed: Math.round((known.wind_speed_ms ?? baseWind) * 10) / 10,
            windDir: known.wind_bearing_deg ?? baseWindDir,
            precip: known.precip_intensity_mmh ?? basePrecip
          };
        });
      } else {
        slots = baseSlots;
        error = 'Weather unavailable';
      }
    } catch {
      error = 'Weather unavailable';
      slots = generateSlots();
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    slots = generateSlots();
    fetchWeather();
  });

  // Refetch when map moves (heavily debounced)
  let fetchTimeout: ReturnType<typeof setTimeout>;
  $effect(() => {
    const _ = $mapBounds;
    clearTimeout(fetchTimeout);
    fetchTimeout = setTimeout(fetchWeather, 3000);
  });

  function selectHour(slot: HourSlot) {
    departureAt.set(slot.time.toISOString());
    slots = slots.map((s) => ({ ...s, isSelected: s.time.getTime() === slot.time.getTime() }));
  }

  function precipHeight(mmh: number): number {
    return Math.min(100, Math.max(0, mmh * 50));
  }

  function windArrow(deg: number): string {
    return `rotate(${deg + 180}deg)`;
  }

  function precipColor(mmh: number): string {
    if (mmh <= 0) return 'transparent';
    if (mmh < 0.5) return '#818cf8';
    if (mmh < 2) return '#6366f1';
    return '#4338ca';
  }
</script>

<div class="shrink-0 border-t border-border bg-surface">
  <div class="overflow-hidden">
    <!-- Dedicated fold strip along the top edge — the edge the panel grows
         from. The grabber bar sits dead-center; its chevron points the way
         the panel will move when pressed. -->
    <button
      onclick={toggleCollapsed}
      aria-expanded={!$collapsed}
      aria-controls="weather-timeline-hours"
      aria-label={$collapsed ? 'Show weather timeline' : 'Hide weather timeline'}
      class="group w-full h-6 flex items-center justify-center bg-surface-overlay/40
             border-b border-border/50 hover:bg-surface-overlay/60 transition-colors"
    >
      <FoldGrabber direction={$collapsed ? 'up' : 'down'} size="sm" />
    </button>

    {#if $collapsed}
      <div
        class="h-9 flex items-center gap-1.5 px-4 text-3xs text-text-subtle uppercase tracking-wider"
      >
        {lensLabel}
        <span class="normal-case tracking-normal text-accent font-medium tabular-nums">
          · {scrubLabel}
        </span>
      </div>
    {:else}
      <!-- Header row: the relocated layer switch shares the line with the
           departure scrubber. -->
      <div class="h-10 flex items-center gap-3 px-4">
        <div class="flex items-center gap-1 shrink-0" role="group" aria-label="Timeline layer">
          {#each lenses as lens (lens.id)}
            <button
              onclick={() => selectLens(lens.id)}
              aria-pressed={$timelineLayer === lens.id}
              class="text-3xs uppercase tracking-wider px-2 py-1 rounded-lg border transition-colors
                     {$timelineLayer === lens.id
                ? 'bg-accent/15 border-accent/40 text-accent-strong font-medium'
                : 'border-transparent text-text-muted hover:text-text hover:bg-surface-overlay/50'}"
            >
              {lens.label}
            </button>
          {/each}
        </div>
        {#if loading}
          <span class="text-3xs text-text-subtle animate-pulse shrink-0">Loading...</span>
        {:else if error}
          <span class="text-3xs text-danger shrink-0">{error}</span>
        {/if}
        <input
          type="range"
          min="0"
          max={SCRUB_SPAN_MIN}
          step={SCRUB_STEP_MIN}
          value={scrubValue}
          oninput={handleScrub}
          aria-label="Departure time"
          aria-valuetext={scrubLabel}
          class="time-scrubber w-full min-w-0"
        />
        <span class="text-2xs text-accent font-medium tabular-nums shrink-0 w-16 text-right">
          {scrubLabel}
        </span>
      </div>
    {/if}

    <div
      id="weather-timeline-hours"
      bind:this={scrollContainer}
      class="flex overflow-x-auto gap-0 px-2 pb-2 scrollbar-thin {$collapsed ? 'hidden' : ''}"
    >
      {#each slots as slot (slot.hour)}
        <button
          onclick={() => selectHour(slot)}
          class="flex flex-col items-center shrink-0 w-14 py-1 rounded-lg transition-colors
                 {slot.isSelected
            ? 'bg-accent/20 border border-accent/40'
            : 'hover:bg-surface-raised/50 border border-transparent'}"
        >
          <span class="text-3xs font-medium {slot.isSelected ? 'text-accent' : 'text-text-muted'}">
            {slot.hour}
          </span>

          <div class="w-6 h-5 flex items-end justify-center my-0.5">
            {#if slot.precip > 0}
              <div
                class="w-4 rounded-t-sm"
                style="height: {Math.max(2, precipHeight(slot.precip))}%; background: {precipColor(
                  slot.precip
                )};"
              ></div>
            {:else}
              <div class="w-4 h-px bg-surface-overlay"></div>
            {/if}
          </div>

          <div
            class="text-3xs h-4 flex items-center justify-center"
            title="Wind {slot.windSpeed}m/s"
          >
            {#if slot.windSpeed > 0.5}
              <span
                style="transform: {windArrow(slot.windDir)}; display: inline-block;"
                class={slot.windSpeed > 5 ? 'text-warning-strong' : 'text-text-muted'}>↑</span
              >
            {:else}
              <span class="text-text-subtle">·</span>
            {/if}
          </div>

          <span
            class="text-3xs {slot.temp > 30
              ? 'text-hot'
              : slot.temp < 10
                ? 'text-cold'
                : 'text-text-subtle'}"
          >
            {slot.temp > 0 ? slot.temp : '--'}°
          </span>
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
  /* The 44px tall input keeps the whole strip an easy drag target while the
     visible track stays a thin line. */
  .time-scrubber {
    -webkit-appearance: none;
    appearance: none;
    height: 44px;
    background: transparent;
    cursor: ew-resize;
  }
  .time-scrubber::-webkit-slider-runnable-track {
    height: 4px;
    border-radius: 2px;
    background: var(--color-surface-overlay, #cbd5e1);
  }
  .time-scrubber::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 16px;
    height: 16px;
    margin-top: -6px;
    border-radius: 50%;
    background: var(--color-accent, #0284c7);
    border: 2px solid var(--color-surface, #ffffff);
    box-shadow: 0 1px 3px rgb(0 0 0 / 0.35);
  }
  .time-scrubber::-moz-range-track {
    height: 4px;
    border-radius: 2px;
    background: var(--color-surface-overlay, #cbd5e1);
  }
  .time-scrubber::-moz-range-thumb {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--color-accent, #0284c7);
    border: 2px solid var(--color-surface, #ffffff);
    box-shadow: 0 1px 3px rgb(0 0 0 / 0.35);
  }

  .scrollbar-thin::-webkit-scrollbar {
    height: 4px;
  }
  .scrollbar-thin::-webkit-scrollbar-track {
    background: transparent;
  }
  .scrollbar-thin::-webkit-scrollbar-thumb {
    background: #334155;
    border-radius: 2px;
  }
</style>
