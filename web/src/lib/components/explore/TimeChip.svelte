<!-- web/src/lib/components/explore/TimeChip.svelte -->
<!-- The collapsed time control (frames A, D): a 40px pill, bottom-centre on
     desktop — "Now · 18° · light NE wind · dry", or "Leave 10:00 · 18° ·
     tailwind on the way back" once a departure or route is set — with a
     chevron that opens the scrubber. On mobile it is a 28px tint chip inside
     the peek sheet; the button pads out to 44px around it. -->
<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import { departureAt } from '$lib/stores/map';
  import { hourly, slotAt } from '$lib/stores/weather';
  import { scrubberOpen, isMobile } from '$lib/stores/ui';
  import { hhmm, isNow, windPhrase, rainPhrase } from '$lib/utils/format';

  interface Props {
    /** Wind wording for the selected route, e.g. "tailwind on the way back". */
    routeWind?: string | null;
  }
  let { routeWind = null }: Props = $props();

  let label = $derived.by(() => {
    const d = $departureAt;
    const slot = slotAt($hourly, d);
    const when = isNow(d) ? 'Now' : `Leave ${hhmm(new Date(d))}`;
    const parts = [when];
    if (slot?.temperatureC != null) parts.push(`${Math.round(slot.temperatureC)}°`);
    if (routeWind) parts.push(routeWind);
    else if (slot?.windSpeedMs != null && slot.windBearingDeg != null)
      parts.push(
        $isMobile
          ? windPhrase(slot.windSpeedMs, slot.windBearingDeg).replace(/ [NESW]+ /, ' ')
          : windPhrase(slot.windSpeedMs, slot.windBearingDeg)
      );
    if (slot?.precipMmh != null && !routeWind) parts.push(rainPhrase(slot.precipMmh));
    return parts.join(' · ');
  });
</script>

<button
  type="button"
  onclick={() => scrubberOpen.set(true)}
  aria-expanded={$scrubberOpen}
  aria-controls="time-scrubber"
  aria-label="Departure time: {label}"
  class="group/chip inline-flex items-center min-h-11 focus:outline-none {$isMobile ? '' : 'w-max'}"
>
  {#if $isMobile}
    <span
      class="inline-flex items-center gap-1.5 h-7 pl-2.5 pr-3 rounded-pill bg-surface-tint border border-hairline
             text-xs font-medium text-text whitespace-nowrap
             group-focus-visible/chip:ring-2 group-focus-visible/chip:ring-focus"
    >
      <Icon name="clock" class="size-6" />
      <span aria-hidden="true">{label}</span>
    </span>
  {:else}
    <span
      class="inline-flex items-center gap-2 h-10 pl-3.5 pr-3.5 rounded-pill bg-surface-base border border-line shadow-lg
             text-13 font-medium text-text whitespace-nowrap transition-colors group-hover/chip:bg-surface-tint
             group-focus-visible/chip:ring-2 group-focus-visible/chip:ring-focus"
    >
      <Icon name="clock" class="size-6" />
      <span aria-hidden="true">{label}</span>
      <Icon name="chevronUp" class="size-6 text-text-subtle" />
    </span>
  {/if}
</button>
