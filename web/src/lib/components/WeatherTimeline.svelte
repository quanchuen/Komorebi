<!-- web/src/lib/components/WeatherTimeline.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { departureAt, mapBounds } from '$lib/stores/map';

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

<div class="shrink-0 border-t border-slate-800 bg-slate-900">
  <div class="overflow-hidden">
    <div class="flex items-center justify-between px-4 pt-2 pb-1">
      <span class="text-[10px] text-slate-500 uppercase tracking-wider">Weather timeline</span>
      {#if loading}
        <span class="text-[10px] text-slate-500 animate-pulse">Loading...</span>
      {:else if error}
        <span class="text-[10px] text-amber-400">{error}</span>
      {/if}
    </div>

    <div bind:this={scrollContainer}
         class="flex overflow-x-auto gap-0 px-2 pb-2 scrollbar-thin">
      {#each slots as slot (slot.hour)}
        <button
          onclick={() => selectHour(slot)}
          class="flex flex-col items-center shrink-0 w-14 py-1 rounded-lg transition-colors
                 {slot.isSelected
            ? 'bg-sky-600/20 border border-sky-500/40'
            : 'hover:bg-slate-800/50 border border-transparent'}"
        >
          <span class="text-[10px] font-medium {slot.isSelected ? 'text-sky-300' : 'text-slate-400'}">
            {slot.hour}
          </span>

          <div class="w-6 h-5 flex items-end justify-center my-0.5">
            {#if slot.precip > 0}
              <div class="w-4 rounded-t-sm"
                style="height: {Math.max(2, precipHeight(slot.precip))}%; background: {precipColor(slot.precip)};"></div>
            {:else}
              <div class="w-4 h-px bg-slate-700"></div>
            {/if}
          </div>

          <div class="text-[10px] h-4 flex items-center justify-center" title="Wind {slot.windSpeed}m/s">
            {#if slot.windSpeed > 0.5}
              <span style="transform: {windArrow(slot.windDir)}; display: inline-block;"
                    class="{slot.windSpeed > 5 ? 'text-amber-400' : 'text-slate-400'}">↑</span>
            {:else}
              <span class="text-slate-600">·</span>
            {/if}
          </div>

          <span class="text-[9px] {slot.temp > 30 ? 'text-red-400' : slot.temp < 10 ? 'text-blue-400' : 'text-slate-500'}">
            {slot.temp > 0 ? slot.temp : '--'}°
          </span>
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
  .scrollbar-thin::-webkit-scrollbar { height: 4px; }
  .scrollbar-thin::-webkit-scrollbar-track { background: transparent; }
  .scrollbar-thin::-webkit-scrollbar-thumb { background: #334155; border-radius: 2px; }
</style>
