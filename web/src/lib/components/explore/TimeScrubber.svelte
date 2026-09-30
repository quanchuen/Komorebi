<!-- web/src/lib/components/explore/TimeScrubber.svelte -->
<!-- Frame C's "TimeScrubber": the time chip expanded. Header — clock,
     "Today, 10:00", the hour's weather, Now, collapse; sunrise / sunset marks;
     rain bars per hour over a 24-hour track with hour ticks and labels every
     two hours (three on mobile). Desktop: 720×112 bottom-left, radius 14.
     Mobile: 366×92 docked above the sheet. The range input is the whole
     track area (44px tall) so it stays an easy drag target. -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import { departureAt, mapCentre } from '$lib/stores/map';
  import { hourly, slotAt, timelineStart } from '$lib/stores/weather';
  import { scrubberOpen, isMobile } from '$lib/stores/ui';
  import { dayTime, hhmm, windPhrase, rainPhrase } from '$lib/utils/format';
  import { sunTimes } from '$lib/utils/sun';

  const STEP_MIN = 10;
  const SPAN_MIN = 24 * 60 - STEP_MIN;
  const start = timelineStart();

  let value = $derived.by(() => {
    const offset = (new Date($departureAt).getTime() - start.getTime()) / 60_000;
    return Math.max(0, Math.min(SPAN_MIN, Math.round(offset / STEP_MIN) * STEP_MIN));
  });
  let selected = $derived(new Date($departureAt));
  let slot = $derived(slotAt($hourly, $departureAt));
  let summary = $derived.by(() => {
    if (!slot) return null;
    const parts: string[] = [];
    if (slot.temperatureC != null) parts.push(`${Math.round(slot.temperatureC)}°`);
    if (slot.windSpeedMs != null && slot.windBearingDeg != null)
      parts.push(windPhrase(slot.windSpeedMs, slot.windBearingDeg));
    if (slot.precipMmh != null) parts.push(rainPhrase(slot.precipMmh));
    return parts.join(' · ');
  });

  function scrub(e: Event) {
    // The input runs to 24:00 so its thumb lands on the hour ticks; the last
    // step is clamped back into the forecast window.
    const v = Math.min(SPAN_MIN, Number((e.currentTarget as HTMLInputElement).value));
    departureAt.set(new Date(start.getTime() + v * 60_000).toISOString());
  }
  function now() {
    const d = new Date();
    d.setMinutes(Math.round(d.getMinutes() / STEP_MIN) * STEP_MIN, 0, 0);
    departureAt.set(d.toISOString());
  }

  // Sunrise / sunset marks inside the 24-hour window, which may span
  // midnight, so both today's and tomorrow's are considered.
  let sunMarks = $derived.by(() => {
    const c = $mapCentre ?? { lat: 35.6895, lon: 139.6917 };
    const pct = (d: Date) => ((d.getTime() - start.getTime()) / 60_000 / (24 * 60)) * 100;
    const marks: { kind: 'rise' | 'set'; at: Date; pct: number }[] = [];
    for (const dayOffset of [0, 1]) {
      const t = sunTimes(new Date(start.getTime() + dayOffset * 86_400_000), c.lat, c.lon);
      if (!t) continue;
      marks.push({ kind: 'rise', at: t.sunrise, pct: pct(t.sunrise) });
      marks.push({ kind: 'set', at: t.sunset, pct: pct(t.sunset) });
    }
    return marks.filter((m) => m.pct >= 2 && m.pct <= 98);
  });

  /** Rain bar height in px: 0 at dry, 21 at 8 mm/h and above. */
  function barHeight(mmh: number | null): number {
    if (!mmh || mmh < 0.05) return 0;
    return Math.max(3, Math.min(21, Math.round((mmh / 8) * 21)));
  }

  let labelEvery = $derived($isMobile ? 3 : 2);
  let hourLabel = (d: Date) => String(d.getHours()).padStart(2, '0');

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') scrubberOpen.set(false);
  }
</script>

<svelte:window onkeydown={$scrubberOpen ? onkeydown : undefined} />

<section
  id="time-scrubber"
  aria-label="Departure time"
  class="bg-surface-base border border-line rounded-xl shadow-lg {$isMobile
    ? 'h-23 px-3'
    : 'h-28 px-4'}"
>
  <div class="flex items-center {$isMobile ? 'h-11' : 'h-12'}">
    {#if !$isMobile}
      <Icon name="clock" class="size-6 text-text mr-0.5" />
    {/if}
    <span class="font-semibold text-text-strong {$isMobile ? 'text-15' : 'text-sm ml-0.5'}"
      >{$isMobile
        ? dayTime(selected).replace('Today, ', '').replace('Tomorrow, ', 'Tmrw ')
        : dayTime(selected)}</span
    >
    {#if summary}
      <span class="ml-4 text-xs text-text-subtle truncate"
        >{$isMobile ? `· ${summary}` : summary}</span
      >
    {/if}
    <span class="flex-1"></span>
    <button
      type="button"
      onclick={now}
      class="group/now inline-flex items-center min-h-11 px-1 focus:outline-none"
    >
      <span
        class="inline-flex items-center rounded-md bg-surface-base border border-line shadow-xs px-3.5 font-medium text-text
               transition-colors group-hover/now:bg-surface-tint group-focus-visible/now:ring-2 group-focus-visible/now:ring-focus
               {$isMobile ? 'h-7.5 text-13' : 'h-7 text-xs'}">Now</span
      >
    </button>
    {#if !$isMobile}
      <IconButton
        icon="chevronDown"
        label="Collapse time slider"
        size={32}
        onclick={() => scrubberOpen.set(false)}
      />
    {/if}
  </div>

  <!-- The track area is padded by half a thumb on each side so the input,
       spanning the whole area with max = 24 h, puts the thumb's centre on
       the hour ticks and never overflows its box. -->
  <div class="relative {$isMobile ? 'h-12' : 'h-15'}">
    <div class="absolute inset-y-0 left-2 right-2" aria-hidden="true">
      <!-- sunrise / sunset -->
      {#if !$isMobile}
        {#each sunMarks as m (m.kind + m.at.getTime())}
          <span
            class="absolute top-0 text-3xs text-text-placeholder -translate-x-1/2 whitespace-nowrap"
            style="left: {m.pct}%">{m.kind === 'rise' ? `☀︎ ${hhmm(m.at)}` : `${hhmm(m.at)} ☾`}</span
          >
        {/each}
      {/if}

      <!-- rain bars and ticks, one per hour, centred on the hour -->
      {#each $hourly as h, i (h.time.getTime())}
        {@const left = (i / 24) * 100}
        {@const bar = barHeight(h.precipMmh)}
        {#if bar > 0}
          <span
            class="absolute w-2.5 rounded-xs -translate-x-1/2 {(h.precipMmh ?? 0) >= 2
              ? 'bg-rain-moderate'
              : 'bg-rain-light'}"
            style="left: {left}%; height: {bar}px; bottom: {$isMobile ? 24 : 26}px"
            aria-hidden="true"
          ></span>
        {/if}
        <span
          class="absolute w-px bg-line -translate-x-1/2 {$isMobile ? 'h-2' : 'h-1'}"
          style="left: {left}%; bottom: {$isMobile ? 16 : 20}px"
          aria-hidden="true"
        ></span>
        {#if i % labelEvery === 0}
          <span
            class="absolute bottom-0 -translate-x-1/2 text-2xs text-text-subtle tabular-nums"
            style="left: {left}%"
            aria-hidden="true">{hourLabel(h.time)}</span
          >
        {/if}
      {/each}
      {#if $hourly.length === 0}
        {#each Array.from({ length: 24 }, (_, i) => i) as i (i)}
          {@const left = (i / 24) * 100}
          <span
            class="absolute w-px h-1 bg-line -translate-x-1/2"
            style="left: {left}%; bottom: 20px"
            aria-hidden="true"
          ></span>
          {#if i % labelEvery === 0}
            <span
              class="absolute bottom-0 -translate-x-1/2 text-2xs text-text-subtle tabular-nums"
              style="left: {left}%"
              aria-hidden="true">{hourLabel(new Date(start.getTime() + i * 3_600_000))}</span
            >
          {/if}
        {/each}
      {/if}

      <!-- track: the past part is near-black -->
      <span
        class="absolute left-0 right-0 h-0.5 rounded-pill bg-fill"
        style="bottom: {$isMobile ? 19 : 25}px"
      ></span>
      <span
        class="absolute left-0 h-0.5 rounded-pill bg-primary"
        style="bottom: {$isMobile ? 19 : 25}px; width: {(value / (24 * 60)) * 100}%"
      ></span>
    </div>

    <input
      type="range"
      min="0"
      max={24 * 60}
      step={STEP_MIN}
      {value}
      oninput={scrub}
      aria-label="Departure time"
      aria-valuetext={dayTime(selected)}
      class="scrubber absolute inset-x-0 {$isMobile ? 'scrubber-mobile' : ''}"
      style="bottom: {$isMobile ? -2 : 4}px"
    />
  </div>
</section>

<style>
  /* The input spans the track; only its 16px knob is visible (white, 2px
     near-black ring). The 44px height keeps the whole strip draggable. */
  .scrubber {
    -webkit-appearance: none;
    appearance: none;
    height: 44px;
    background: transparent;
    cursor: ew-resize;
    margin: 0;
  }
  .scrubber::-webkit-slider-runnable-track {
    height: 2px;
    background: transparent;
  }
  .scrubber::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 16px;
    height: 16px;
    margin-top: -7px;
    border-radius: 50%;
    background: var(--color-surface-base);
    border: 2px solid var(--color-primary);
    box-shadow: var(--shadow-xs);
  }
  .scrubber::-moz-range-track {
    height: 2px;
    background: transparent;
  }
  .scrubber::-moz-range-thumb {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--color-surface-base);
    border: 2px solid var(--color-primary);
    box-shadow: var(--shadow-xs);
  }
  .scrubber:focus-visible {
    outline: none;
  }
  .scrubber:focus-visible::-webkit-slider-thumb {
    box-shadow:
      0 0 0 3px var(--color-surface-base),
      0 0 0 5px var(--color-focus);
  }
  /* Mobile draws a 2×40 playhead instead of a knob. */
  .scrubber-mobile::-webkit-slider-thumb {
    width: 16px;
    height: 40px;
    margin-top: -21px;
    border-radius: 0;
    background: linear-gradient(
      to right,
      transparent 7px,
      var(--color-primary) 7px 9px,
      transparent 9px
    );
    border: none;
    box-shadow: none;
  }
  .scrubber-mobile::-moz-range-thumb {
    width: 16px;
    height: 40px;
    border-radius: 0;
    background: linear-gradient(
      to right,
      transparent 7px,
      var(--color-primary) 7px 9px,
      transparent 9px
    );
    border: none;
    box-shadow: none;
  }
</style>
