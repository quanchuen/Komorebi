<!-- web/src/lib/components/DataSourcesSheet.svelte -->
<!-- Frame E: every data source, what it is used for, its licence and freshness.
     Opened from the attribution pill and the first-ride disclaimer. -->
<script lang="ts">
  import { DATA_SOURCES, OSM_FIX_URL, sourceFreshness } from '$lib/stores/attribution';
  import { dataSourcesOpen } from '$lib/stores/ui';
  import Icon from './ui/Icon.svelte';

  let dialog: HTMLDialogElement;

  // Swatch for sources that own a map ramp (one hue per meaning).
  const swatch: Partial<Record<string, string>> = {
    plateau: 'bg-shade-mostly',
    'open-meteo': 'bg-rain-light'
  };

  $effect(() => {
    if (!dialog) return;
    if ($dataSourcesOpen && !dialog.open) dialog.showModal();
    else if (!$dataSourcesOpen && dialog.open) dialog.close();
  });

  function close() {
    dataSourcesOpen.set(false);
  }

  function onBackdrop(e: MouseEvent) {
    if (e.target === dialog) close();
  }
</script>

<dialog
  bind:this={dialog}
  onclose={close}
  onclick={onBackdrop}
  aria-labelledby="data-sources-title"
  aria-describedby="data-sources-lead"
  class="sheet m-auto w-full rounded-dialog bg-surface-base p-0 text-text-default shadow-lg
         backdrop:bg-text-strong/30"
>
  <div class="flex max-h-full flex-col">
    <header class="flex items-start gap-3 border-b border-hairline px-6 pb-4 pt-5">
      <div class="min-w-0 flex-1">
        <h2 id="data-sources-title" class="text-lg font-semibold text-text-strong">Data sources</h2>
        <p id="data-sources-lead" class="mt-0.5 text-sm text-text-subtle">
          What powers this map, under which licence, and how fresh it is.
        </p>
      </div>
      <button
        type="button"
        onclick={close}
        aria-label="Close data sources"
        class="-mr-2 -mt-1 flex size-11 shrink-0 items-center justify-center rounded-control
               text-text-subtle hover:bg-surface-tint hover:text-text-default
               focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <Icon name="x" class="size-5" />
      </button>
    </header>

    <div class="min-h-0 overflow-y-auto px-6" data-scroll="true">
      <table class="w-full border-collapse text-left text-sm">
        <thead>
          <tr class="border-b border-hairline text-2xs font-semibold uppercase text-text-subtle">
            <th scope="col" class="py-3 pr-3 font-semibold tracking-wide">Source</th>
            <th scope="col" class="py-3 pr-3 font-semibold tracking-wide">Used for</th>
            <th scope="col" class="py-3 font-semibold tracking-wide">Licence · freshness</th>
          </tr>
        </thead>
        <tbody>
          {#each DATA_SOURCES as source (source.id)}
            {@const fresh = $sourceFreshness[source.id] ?? source.freshness}
            <tr class="border-b border-hairline align-top">
              <th scope="row" class="py-2 pr-3 font-normal">
                <!-- eslint-disable svelte/no-navigation-without-resolve -->
                <a
                  href={source.licenceUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex min-h-11 min-w-11 items-center text-text-default underline
                         underline-offset-2 hover:text-text-strong focus:outline-none
                         focus-visible:ring-2 focus-visible:ring-focus"
                >
                  {source.name}
                </a>
                <!-- eslint-enable svelte/no-navigation-without-resolve -->
              </th>
              <td class="py-3 pr-3 text-text-default">
                <span class="flex items-start gap-1.5 pt-0.5">
                  {#if swatch[source.id]}
                    <span
                      class="mt-1 size-2.5 shrink-0 rounded-sm {swatch[source.id]}"
                      aria-hidden="true"
                    ></span>
                  {/if}
                  {source.usedFor}
                </span>
              </td>
              <td class="py-3">
                <div class="pt-0.5 text-text-default">{source.licence}</div>
                <div class="text-xs text-text-subtle">
                  {#if fresh}{fresh}{:else}<span aria-label="Freshness not reported">—</span>{/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p
        class="my-4 flex items-start gap-2 rounded-control border border-hairline
               bg-surface-elevated px-3 py-2.5 text-xs text-text-default"
      >
        <Icon name="leaf" class="mt-0.5 size-4 shrink-0 text-text-subtle" />
        Shade and greenery scores are computed by Komorebi from the sources above and keep their licences.
      </p>
    </div>

    <footer class="flex flex-wrap items-center justify-between gap-3 px-6 pb-5 pt-1">
      <!-- eslint-disable svelte/no-navigation-without-resolve -->
      <a
        href={OSM_FIX_URL}
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex min-h-11 items-center gap-2 rounded-control border border-line
               bg-surface-base px-3 text-sm text-text-default shadow-xs hover:bg-surface-tint
               focus:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <Icon name="external-link" class="size-4" />
        Fix a map error on OpenStreetMap
      </a>
      <!-- eslint-enable svelte/no-navigation-without-resolve -->
      <button
        type="button"
        onclick={close}
        class="inline-flex min-h-11 min-w-20 items-center justify-center rounded-control
               bg-primary px-5 text-sm font-medium text-on-primary hover:bg-primary-hover
               focus:outline-none focus-visible:ring-2 focus-visible:ring-focus
               focus-visible:ring-offset-2"
      >
        Done
      </button>
    </footer>
  </div>
</dialog>

<style>
  /* A 16px gutter on phones; the UA max-height keeps it inside the viewport. */
  .sheet {
    max-width: min(37.5rem, calc(100vw - 2rem));
    max-height: calc(100dvh - 2rem);
  }
  .sheet[open] {
    display: flex;
    flex-direction: column;
  }
</style>
