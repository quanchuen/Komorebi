<!-- web/src/lib/components/DataSourcesSheet.svelte -->
<script lang="ts">
  import { dataSourcesOpen, firstRideDisclaimerRequest } from '$lib/stores/ui';

  // Frame E. Every source the app draws from, whatever is on screen now.
  // Freshness ("20 Sep", "2023 edition", "updated 09:00" in the prototype) is
  // deliberately absent until it can be read from real dataset metadata.
  const SOURCES = [
    {
      name: 'OpenStreetMap contributors',
      href: 'https://www.openstreetmap.org/copyright',
      usedFor: 'roads, paths, places, routing graph',
      licence: 'ODbL 1.0'
    },
    {
      name: 'CARTO',
      href: 'https://carto.com/attributions',
      usedFor: 'basemap tiles',
      licence: 'CARTO basemap terms'
    },
    {
      name: 'Project PLATEAU · MLIT',
      href: 'https://www.mlit.go.jp/plateau/',
      usedFor: '3D buildings → shade',
      licence: 'CC BY 4.0'
    },
    {
      name: 'Open-Meteo',
      href: 'https://open-meteo.com/',
      usedFor: 'rain, wind, temperature',
      licence: 'CC BY 4.0'
    },
    {
      name: 'Nominatim',
      href: 'https://nominatim.org/',
      usedFor: 'place search',
      licence: 'ODbL (OSM data)'
    },
    {
      name: 'Valhalla',
      href: 'https://github.com/valhalla/valhalla',
      usedFor: 'route calculation (software)',
      licence: 'MIT'
    }
  ];

  let dialog = $state<HTMLDialogElement | null>(null);

  // A native <dialog> gives the top layer, focus containment and Escape.
  $effect(() => {
    const el = dialog;
    if (!el) return;
    if ($dataSourcesOpen && !el.open) el.showModal();
    else if (!$dataSourcesOpen && el.open) el.close();
  });

  function close() {
    dataSourcesOpen.set(false);
  }

  // Clicking the backdrop (outside the panel) closes the sheet.
  function onBackdropClick(e: MouseEvent) {
    if (e.target === dialog) close();
  }
</script>

<dialog
  bind:this={dialog}
  onclose={close}
  onclick={onBackdropClick}
  aria-labelledby="data-sources-title"
  class="m-0 p-0 w-full max-w-full h-dvh max-h-dvh bg-transparent
         sm:m-auto sm:h-auto sm:w-3xl data-sources-dialog
         backdrop:bg-text/40"
>
  <div
    class="flex flex-col h-full sm:h-auto data-sources-panel bg-surface-base text-text
           sm:rounded-xl shadow-lg"
    role="document"
  >
    <header class="flex items-start justify-between gap-4 px-5 pt-5 pb-3">
      <div>
        <h2 id="data-sources-title" class="text-lg font-semibold text-text-strong">Data sources</h2>
        <p class="text-sm text-text-subtle mt-1">
          Everything Komorebi draws comes from these datasets and tools.
        </p>
      </div>
      <button
        type="button"
        onclick={close}
        aria-label="Close"
        class="shrink-0 inline-flex items-center justify-center size-11 rounded-md
               text-text-subtle hover:bg-surface-tint hover:text-text
               focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <svg
          viewBox="0 0 24 24"
          class="size-5"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg
        >
      </button>
    </header>

    <div class="flex-1 min-h-0 overflow-y-auto px-5 pb-5" data-scroll="true">
      <!-- Desktop: a table. Mobile: one card per source. -->
      <table class="hidden sm:table w-full text-sm border-collapse">
        <thead>
          <tr class="text-left text-2xs uppercase tracking-wider text-text-subtle">
            <th scope="col" class="py-2 pr-4 font-medium border-b border-hairline">Source</th>
            <th scope="col" class="py-2 pr-4 font-medium border-b border-hairline">Used for</th>
            <th scope="col" class="py-2 font-medium border-b border-hairline">Licence</th>
          </tr>
        </thead>
        <tbody>
          {#each SOURCES as s (s.name)}
            <tr class="border-b border-hairline last:border-b-0">
              <th scope="row" class="py-3 pr-4 font-medium text-left align-top">
                <a
                  href={s.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center min-h-11 underline decoration-line-strong underline-offset-2 hover:decoration-text"
                >
                  {s.name}
                </a>
              </th>
              <td class="py-3 pr-4 text-text-subtle align-top">{s.usedFor}</td>
              <td class="py-3 text-text-subtle align-top whitespace-nowrap">{s.licence}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <ul class="sm:hidden space-y-2" aria-label="Sources">
        {#each SOURCES as s (s.name)}
          <li class="rounded-md border border-line bg-surface-elevated p-3">
            <a
              href={s.href}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center min-h-11 font-medium underline decoration-line-strong underline-offset-2"
            >
              {s.name}
            </a>
            <div class="text-sm text-text-subtle">{s.usedFor}</div>
            <div class="text-2xs text-text-subtle mt-1">{s.licence}</div>
          </li>
        {/each}
      </ul>

      <p class="text-sm text-text-subtle mt-4">
        Derived scores (shade, greenery) keep their sources' licences.
      </p>
      <p class="mt-3 flex flex-wrap items-center gap-x-4">
        <!-- Until the Menu → About entry exists, the first-ride disclaimer is
             re-readable from here. -->
        <button
          type="button"
          onclick={() => firstRideDisclaimerRequest.set({})}
          class="inline-flex items-center min-h-11 text-sm font-medium underline decoration-line-strong underline-offset-2 hover:decoration-text"
        >
          Before you ride
        </button>
        <a
          href="https://www.openstreetmap.org/fixthemap"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center min-h-11 text-sm font-medium underline decoration-line-strong underline-offset-2 hover:decoration-text"
        >
          Fix a map error on OpenStreetMap
        </a>
      </p>
    </div>
  </div>
</dialog>

<style>
  /* Sizes the token checker cannot express as utilities: a centred sheet on
     wide screens that never exceeds the viewport. */
  @media (min-width: 40rem) {
    .data-sources-dialog {
      max-width: calc(100% - 2rem);
      max-height: 90dvh;
    }
    .data-sources-panel {
      max-height: 90dvh;
    }
  }
</style>
