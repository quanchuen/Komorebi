<!-- web/src/lib/components/WeatherTimeline.svelte -->
<!-- Bottom timeline scrubber showing hourly weather for the map center -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { departureAt, mapBounds, activeOverlay } from '$lib/stores/map';

  interface HourSlot {
    hour: string;       // "14:00"
    time: Date;
    temp: number;       // celsius
    windSpeed: number;  // m/s
    windDir: number;    // degrees
    precip: number;     // mm/h
    isSelected: boolean;
  }

  let slots = $state<HourSlot[]>([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let scrollContainer: HTMLDivElement;

  // Generate 24 hour slots from current time
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

    const centerLat = (bounds.minLat + bounds.maxLat) / 2;
    const centerLon = (bounds.minLon + bounds.maxLon) / 2;

    loading = true;
    error = null;

    try {
      // Fetch weather for center point at each hour
      const baseSlots = generateSlots();
      const updated = await Promise.all(
        baseSlots.map(async (slot) => {
          try {
            const res = await fetch(
              `/api/v1/weather/point?lat=${centerLat}&lon=${centerLon}&at=${slot.time.toISOString()}`
            );
            if (res.ok) {
              const data = await res.json();
              return {
                ...slot,
                temp: Math.round(data.temperature_c ?? 0),
                windSpeed: Math.round((data.wind_speed_ms ?? 0) * 10) / 10,
                windDir: data.wind_bearing_deg ?? 0,
                precip: data.precip_intensity_mmh ?? 0
              };
            }
          } catch { /* weather unavailable for this hour */ }
          return slot;
        })
      );
      slots = updated;
    } catch (e) {
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

  // Refetch when map moves
  let fetchTimeout: ReturnType<typeof setTimeout>;
  $effect(() => {
    const _ = $mapBounds;
    clearTimeout(fetchTimeout);
    fetchTimeout = setTimeout(fetchWeather, 1500); // debounce
  });

  function selectHour(slot: HourSlot) {
    departureAt.set(slot.time.toISOString());
    slots = slots.map((s) => ({ ...s, isSelected: s.time.getTime() === slot.time.getTime() }));
  }

  function precipHeight(mmh: number): number {
    return Math.min(100, Math.max(0, mmh * 50)); // 2mm/h = full bar
  }

  function windArrow(deg: number): string {
    // Rotate arrow to show wind FROM direction
    return `rotate(${deg + 180}deg)`;
  }

  function precipColor(mmh: number): string {
    if (mmh <= 0) return 'transparent';
    if (mmh < 0.5) return '#818cf8'; // indigo-400
    if (mmh < 2) return '#6366f1'; // indigo-500
    return '#4338ca'; // indigo-700
  }
</script>

<div class="absolute bottom-0 left-0 right-0 z-10 pointer-events-auto">
  <div class="mx-4 mb-4 bg-slate-900/90 backdrop-blur-lg border border-slate-700/50
              rounded-2xl shadow-2xl overflow-hidden">

    <!-- Header row -->
    <div class="flex items-center justify-between px-4 pt-2 pb-1">
      <span class="text-[10px] text-slate-500 uppercase tracking-wider">Weather timeline</span>
      {#if loading}
        <span class="text-[10px] text-slate-500 animate-pulse">Loading...</span>
      {:else if error}
        <span class="text-[10px] text-amber-400">{error}</span>
      {/if}
    </div>

    <!-- Scrollable timeline -->
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
          <!-- Hour label -->
          <span class="text-[10px] font-medium {slot.isSelected ? 'text-sky-300' : 'text-slate-400'}">
            {slot.hour}
          </span>

          <!-- Precip bar -->
          <div class="w-6 h-5 flex items-end justify-center my-0.5">
            {#if slot.precip > 0}
              <div
                class="w-4 rounded-t-sm"
                style="height: {Math.max(2, precipHeight(slot.precip))}%; background: {precipColor(slot.precip)};"
              ></div>
            {:else}
              <div class="w-4 h-px bg-slate-700"></div>
            {/if}
          </div>

          <!-- Wind arrow -->
          <div class="text-[10px] h-4 flex items-center justify-center"
               title="Wind {slot.windSpeed}m/s">
            {#if slot.windSpeed > 0.5}
              <span style="transform: {windArrow(slot.windDir)}; display: inline-block;"
                    class="{slot.windSpeed > 5 ? 'text-amber-400' : 'text-slate-400'}">
                ↑
              </span>
            {:else}
              <span class="text-slate-600">·</span>
            {/if}
          </div>

          <!-- Temp -->
          <span class="text-[9px] {slot.temp > 30 ? 'text-red-400' : slot.temp < 10 ? 'text-blue-400' : 'text-slate-500'}">
            {slot.temp > 0 ? slot.temp : '--'}°
          </span>
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
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
