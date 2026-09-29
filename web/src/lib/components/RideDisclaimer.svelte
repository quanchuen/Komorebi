<!-- web/src/lib/components/RideDisclaimer.svelte -->
<!-- Frame F: shown once, when the rider first starts a ride. DRAFT COPY: needs
     legal review (Japanese consumer law limits blanket liability exclusions)
     and a native-speaker review before shipping. -->
<script lang="ts">
  import {
    acceptRideDisclaimer,
    disclaimerLocale,
    dismissRideDisclaimer,
    rideDisclaimerOpen,
    type DisclaimerLocale
  } from '$lib/stores/disclaimer';
  import { dataSourcesOpen } from '$lib/stores/ui';
  import Icon, { type IconName } from './ui/Icon.svelte';

  interface Copy {
    title: string;
    lead: string;
    points: { icon: IconName; title: string; body: string }[];
    accept: string;
    dataSources: string;
    footer: string;
    languageLabel: string;
  }

  const COPY: Record<DisclaimerLocale, Copy> = {
    en: {
      title: 'Before you ride',
      lead: 'Komorebi helps you plan. It can’t see the road you’re on.',
      points: [
        {
          icon: 'info',
          title: 'Estimates, not guarantees',
          body: 'Routes, shade, rain and wind come from open data and forecasts. They can be incomplete, out of date or wrong.'
        },
        {
          icon: 'signpost',
          title: 'The road comes first',
          body: 'Always follow road signs, signals and Japanese traffic law. What you see on the road overrides the app.'
        },
        {
          icon: 'circle-minus',
          title: 'Respect private property',
          body: 'Routes can run past gates, estates or paths that are private or closed. If a sign says no entry, don’t go in — turn back or find another way.'
        },
        {
          icon: 'smartphone',
          title: 'Hands off while riding',
          body: 'Don’t operate your phone while riding. Stop somewhere safe before you touch the screen.'
        },
        {
          icon: 'shield',
          title: 'You ride at your own risk',
          body: 'Komorebi isn’t liable for accidents, injuries or losses from using the app.'
        }
      ],
      accept: 'I understand',
      dataSources: 'Data sources',
      footer: 'Shown before your first ride. Always available in Menu → About.',
      languageLabel: 'Language'
    },
    ja: {
      title: 'ご利用前に',
      lead: 'Komorebiは走行計画をお手伝いしますが、実際の道路状況は把握できません。',
      points: [
        {
          icon: 'info',
          title: '表示は推定です',
          body: 'ルート・日陰・雨・風の情報は、オープンデータと気象予報に基づく推定です。不完全、古い、または誤っている場合があります。'
        },
        {
          icon: 'signpost',
          title: '実際の道路が最優先',
          body: '必ず道路標識・信号・道路交通法に従ってください。アプリの表示よりも、実際の道路状況を優先してください。'
        },
        {
          icon: 'circle-minus',
          title: '私有地に立ち入らない',
          body: 'ルートが私有地や通行止めの道を通る場合があります。「私有地」「立入禁止」などの表示がある場合は進入せず、引き返すか別の道を選んでください。'
        },
        {
          icon: 'smartphone',
          title: '走行中は操作しない',
          body: '走行中にスマートフォンを操作しないでください。操作は安全な場所に停車してから行ってください。'
        },
        {
          icon: 'shield',
          title: 'ご自身の責任でご利用ください',
          body: '本アプリの利用により生じた事故・怪我・損害について、Komorebiは責任を負いかねます。'
        }
      ],
      accept: '理解しました',
      dataSources: 'データ出典',
      footer: '初回走行前に表示されます。メニュー → このアプリについて からいつでも確認できます。',
      languageLabel: '言語'
    }
  };

  const LOCALES: { id: DisclaimerLocale; label: string; lang: string }[] = [
    { id: 'en', label: 'EN', lang: 'en' },
    { id: 'ja', label: '日本語', lang: 'ja' }
  ];

  let dialog: HTMLDialogElement;
  let copy = $derived(COPY[$disclaimerLocale]);

  $effect(() => {
    if (!dialog) return;
    if ($rideDisclaimerOpen && !dialog.open) dialog.showModal();
    else if (!$rideDisclaimerOpen && dialog.open) dialog.close();
  });

  // Esc or a backdrop tap cancels the ride; only "I understand" starts it.
  function onClose() {
    if ($rideDisclaimerOpen) dismissRideDisclaimer();
  }

  function onBackdrop(e: MouseEvent) {
    if (e.target === dialog) dismissRideDisclaimer();
  }
</script>

<dialog
  bind:this={dialog}
  onclose={onClose}
  onclick={onBackdrop}
  lang={$disclaimerLocale}
  aria-labelledby="ride-disclaimer-title"
  aria-describedby="ride-disclaimer-lead"
  class="disclaimer m-auto w-full rounded-dialog bg-surface-base p-0 text-text-default shadow-lg
         backdrop:bg-text-strong/30"
>
  <div class="overflow-y-auto px-7 pb-6 pt-7" data-scroll="true">
    <div class="flex items-center justify-between gap-3">
      <span
        class="flex size-9 items-center justify-center rounded-control bg-primary text-on-primary"
        aria-hidden="true"
      >
        <Icon name="bike" class="size-5" />
      </span>
      <div
        role="group"
        aria-label={copy.languageLabel}
        class="flex rounded-control bg-surface-recessed p-0.5"
      >
        {#each LOCALES as locale (locale.id)}
          <button
            type="button"
            lang={locale.lang}
            aria-pressed={$disclaimerLocale === locale.id}
            onclick={() => disclaimerLocale.set(locale.id)}
            class="min-h-11 min-w-14 rounded-control px-3 text-xs font-medium
                   focus:outline-none focus-visible:ring-2 focus-visible:ring-focus
                   {$disclaimerLocale === locale.id
              ? 'bg-surface-base text-text-default shadow-xs'
              : 'text-text-subtle hover:text-text-default'}"
          >
            {locale.label}
          </button>
        {/each}
      </div>
    </div>

    <h2 id="ride-disclaimer-title" class="mt-5 text-xl font-semibold text-text-strong">
      {copy.title}
    </h2>
    <p id="ride-disclaimer-lead" class="mt-1 text-sm text-text-subtle">{copy.lead}</p>

    <ol class="mt-5 space-y-4">
      {#each copy.points as point (point.title)}
        <li class="flex gap-3">
          <span
            class="flex size-8 shrink-0 items-center justify-center rounded-control border
                   border-line bg-surface-base text-text-default"
            aria-hidden="true"
          >
            <Icon name={point.icon} class="size-4" />
          </span>
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-text-strong">{point.title}</h3>
            <p class="mt-0.5 text-sm text-text-subtle">{point.body}</p>
          </div>
        </li>
      {/each}
    </ol>

    <button
      type="button"
      onclick={acceptRideDisclaimer}
      class="mt-6 flex min-h-11 w-full items-center justify-center rounded-control bg-primary
             text-sm font-semibold text-on-primary hover:bg-primary-hover focus:outline-none
             focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2"
    >
      {copy.accept}
    </button>

    <!-- "Terms of use" joins this row once the Terms page exists. -->
    <div class="mt-1 flex justify-center">
      <button
        type="button"
        onclick={() => dataSourcesOpen.set(true)}
        aria-haspopup="dialog"
        class="min-h-11 px-2 text-xs text-text-default underline underline-offset-2
               hover:text-text-strong focus:outline-none focus-visible:ring-2
               focus-visible:ring-focus"
      >
        {copy.dataSources}
      </button>
    </div>

    <p class="mt-2 border-t border-hairline pt-4 text-center text-xs text-text-subtle">
      {copy.footer}
    </p>
  </div>
</dialog>

<style>
  .disclaimer {
    max-width: min(32.5rem, calc(100vw - 2rem));
    max-height: calc(100dvh - 2rem);
  }
  .disclaimer[open] {
    display: flex;
    flex-direction: column;
  }
</style>
