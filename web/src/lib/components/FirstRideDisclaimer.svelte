<!-- web/src/lib/components/FirstRideDisclaimer.svelte -->
<script lang="ts">
  import { browser } from '$app/environment';
  import {
    dataSourcesOpen,
    firstRideDisclaimerRequest as request,
    firstRideDisclaimerSeen as seen
  } from '$lib/stores/ui';

  // Frame F. Shown once, before the first "Ride this"; reachable again from
  // the Data sources sheet until the Menu → About entry exists.
  //
  // DRAFT COPY — pending legal review (Japanese consumer law limits blanket
  // liability exclusions) and a native-speaker review of the Japanese. Keep
  // the wording in step with docs/specs/2026-09-29-ui-rework.md.
  type Lang = 'en' | 'ja';

  const COPY: Record<
    Lang,
    {
      switchLabel: string;
      title: string;
      lead: string;
      points: { title: string; body: string }[];
      accept: string;
      terms: string;
      sources: string;
      footer: string;
      draft: string;
    }
  > = {
    en: {
      switchLabel: 'Language',
      title: 'Before you ride',
      lead: "Komorebi helps you plan. It can't see the road you're on.",
      points: [
        {
          title: 'Estimates, not guarantees',
          body: 'Routes, shade, rain and wind come from open data and forecasts. They can be incomplete, out of date or wrong.'
        },
        {
          title: 'The road comes first',
          body: 'Always follow road signs, signals and Japanese traffic law. What you see on the road overrides the app.'
        },
        {
          title: 'Respect private property',
          body: "Routes can run past gates, estates or paths that are private or closed. If a sign says no entry, don't go in — turn back or find another way."
        },
        {
          title: 'Hands off while riding',
          body: "Don't operate your phone while riding. Stop somewhere safe before you touch the screen."
        },
        {
          title: 'You ride at your own risk',
          body: "Komorebi isn't liable for accidents, injuries or losses from using the app."
        }
      ],
      accept: 'I understand',
      terms: 'Terms of use',
      sources: 'Data sources',
      footer: 'Shown before your first ride. Always available in Menu → About.',
      draft: 'Draft wording — pending legal review.'
    },
    ja: {
      switchLabel: '言語',
      title: 'ご利用前に',
      lead: 'Komorebiは走行計画をお手伝いしますが、実際の道路状況は把握できません。',
      points: [
        {
          title: '表示は推定です',
          body: 'ルート・日陰・雨・風の情報は、オープンデータと気象予報に基づく推定です。不完全、古い、または誤っている場合があります。'
        },
        {
          title: '実際の道路が最優先',
          body: '必ず道路標識・信号・道路交通法に従ってください。アプリの表示よりも、実際の道路状況を優先してください。'
        },
        {
          title: '私有地に立ち入らない',
          body: 'ルートが私有地や通行止めの道を通る場合があります。「私有地」「立入禁止」などの表示がある場合は進入せず、引き返すか別の道を選んでください。'
        },
        {
          title: '走行中は操作しない',
          body: '走行中にスマートフォンを操作しないでください。操作は安全な場所に停車してから行ってください。'
        },
        {
          title: 'ご自身の責任でご利用ください',
          body: '本アプリの利用により生じた事故・怪我・損害について、Komorebiは責任を負いかねます。'
        }
      ],
      accept: '理解しました',
      terms: '利用規約',
      sources: 'データ出典',
      footer: '初回走行前に表示されます。メニュー → このアプリについて からいつでも確認できます。',
      draft: '文言はドラフトです — 法務レビュー待ち。'
    }
  };

  function deviceLang(): Lang {
    return browser && navigator.language.toLowerCase().startsWith('ja') ? 'ja' : 'en';
  }

  let lang = $state<Lang>('en');
  let dialog = $state<HTMLDialogElement | null>(null);
  let copy = $derived(COPY[lang]);

  // Open on request (device locale each time), close when the request clears.
  $effect(() => {
    const el = dialog;
    const req = $request;
    if (!el) return;
    if (req && !el.open) {
      lang = deviceLang();
      el.showModal();
    } else if (!req && el.open) {
      el.close();
    }
  });

  function accept() {
    const req = $request;
    seen.set(true);
    request.set(null);
    req?.onAccept?.();
  }

  // Closing by Escape or backdrop dismisses without accepting: the ride does
  // not start and the disclaimer will show again next time.
  function dismiss() {
    request.set(null);
  }

  function onBackdropClick(e: MouseEvent) {
    if (e.target === dialog) dismiss();
  }
</script>

<dialog
  bind:this={dialog}
  onclose={dismiss}
  onclick={onBackdropClick}
  aria-labelledby="first-ride-title"
  {lang}
  class="m-0 p-0 w-full max-w-full h-dvh max-h-dvh bg-transparent sm:m-auto sm:h-auto sm:w-lg
         first-ride-dialog backdrop:bg-text/40"
>
  <div
    class="flex flex-col h-full sm:h-auto first-ride-panel bg-surface-base text-text sm:rounded-xl shadow-lg"
  >
    <div class="flex items-center justify-end px-5 pt-4">
      <div
        class="inline-flex rounded-md bg-surface-recessed p-0.5"
        role="group"
        aria-label={copy.switchLabel}
      >
        {#each [['en', 'EN'], ['ja', '日本語']] as [code, label] (code)}
          <button
            type="button"
            onclick={() => (lang = code as Lang)}
            aria-pressed={lang === code}
            lang={code}
            class="min-h-11 min-w-11 px-3 rounded-sm text-xs font-medium transition-colors
                   {lang === code
              ? 'bg-surface-base text-text shadow-xs'
              : 'text-text-subtle hover:text-text'}"
          >
            {label}
          </button>
        {/each}
      </div>
    </div>

    <div class="flex-1 min-h-0 overflow-y-auto px-5 pb-5" data-scroll="true">
      <h2 id="first-ride-title" class="text-xl font-semibold text-text-strong mt-2">
        {copy.title}
      </h2>
      <p class="text-base text-text-subtle mt-1">{copy.lead}</p>

      <ol class="mt-5 space-y-4">
        {#each copy.points as point, i (i)}
          <li class="flex gap-3">
            <span
              class="shrink-0 size-6 rounded-full bg-surface-recessed text-text text-2xs font-semibold
                     inline-flex items-center justify-center tabular-nums"
              aria-hidden="true">{i + 1}</span
            >
            <div>
              <div class="text-sm font-semibold text-text-strong">{point.title}</div>
              <p class="text-sm text-text-subtle mt-0.5">{point.body}</p>
            </div>
          </li>
        {/each}
      </ol>

      <button
        type="button"
        onclick={accept}
        class="mt-6 w-full min-h-11 rounded-md bg-primary text-on-primary text-sm font-semibold
               hover:bg-primary-hover transition-colors
               focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        {copy.accept}
      </button>

      <div class="mt-3 flex items-center justify-center gap-1 text-sm">
        <!-- The Terms of use page does not exist yet (open item in the spec). -->
        <span
          class="inline-flex items-center min-h-11 px-1 text-text-placeholder"
          aria-disabled="true"
        >
          {copy.terms}
        </span>
        <span class="text-text-placeholder" aria-hidden="true">·</span>
        <button
          type="button"
          onclick={() => dataSourcesOpen.set(true)}
          class="inline-flex items-center min-h-11 px-1 underline decoration-line-strong underline-offset-2 hover:decoration-text"
        >
          {copy.sources}
        </button>
      </div>

      <p class="mt-2 text-center text-2xs text-text-subtle">{copy.footer}</p>
      <p class="mt-1 text-center text-2xs text-text-placeholder">{copy.draft}</p>
    </div>
  </div>
</dialog>

<style>
  @media (min-width: 40rem) {
    .first-ride-dialog {
      max-width: calc(100% - 2rem);
      max-height: 90dvh;
    }
    .first-ride-panel {
      max-height: 90dvh;
    }
  }
</style>
