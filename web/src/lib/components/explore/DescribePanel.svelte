<!-- web/src/lib/components/explore/DescribePanel.svelte -->
<!-- Frame H: Describe a ride. The model only interprets text (ADR 0003):
     it returns preferences, shown as removable chips, and what it could not
     honour under NOT SUPPORTED YET. "Plan this route" applies the chips and
     opens Plan a route; nothing is routed until the rider confirms. The
     PLACES block of the design needs the RouteIntent schema extension and
     is not built yet. -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { routing } from '$lib/api/client';
  import type { RouteIntentResponse, RoutingPreferences } from '$lib/api/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PanelHeader from './PanelHeader.svelte';
  import SectionLabel from './SectionLabel.svelte';
  import Sheet from './Sheet.svelte';
  import { plannerPreferences } from '$lib/stores/planner';
  import { describeAvailable, isMobile } from '$lib/stores/ui';

  interface Props {
    /** Text carried over from the search field. */
    initialText?: string;
    onback: () => void;
    onclose: () => void;
    onplan: () => void;
  }
  let { initialText = '', onback, onclose, onplan }: Props = $props();

  // The field is seeded from the search box once, then owned by the rider.
  // svelte-ignore state_referenced_locally
  let text = $state(initialText);
  let loading = $state(false);
  let result = $state<RouteIntentResponse | null>(null);
  let error = $state<string | null>(null);
  // Chips the rider removed before planning.
  let dropped = $state<Set<string>>(new Set());
  let textarea = $state<HTMLTextAreaElement | null>(null);

  onMount(() => {
    if (text.trim()) void interpret();
    else textarea?.focus();
  });

  type PrefKey = keyof RoutingPreferences;
  const CHIP: Record<PrefKey, (v: number) => string> = {
    shade: (v) => (v > 0.5 ? 'More shade' : 'Shade matters less'),
    greenery: (v) => (v > 0.5 ? 'More greenery' : 'Greenery matters less'),
    wind: (v) => (v > 0.5 ? 'Tailwind preferred' : 'Wind matters less')
  };
  const UNSUPPORTED: Record<string, string> = {
    max_detour_m: "a detour budget isn't available yet, so routes may run longer",
    max_grade_percent: "a climb limit isn't available yet, so routes may still climb"
  };

  let applied = $derived(
    (result?.applied ?? []).filter((k): k is PrefKey => k in CHIP && !dropped.has(k))
  );

  async function interpret(e?: Event) {
    e?.preventDefault();
    const t = text.trim();
    if (!t || loading) return;
    loading = true;
    error = null;
    result = null;
    dropped = new Set();
    try {
      result = await routing.interpretIntent(t, $plannerPreferences);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('503')) {
        describeAvailable.set(false);
        error = "Describing rides isn't available on this server.";
      } else if (msg.includes('Failed to fetch')) {
        error = "Can't reach Komorebi — showing the map only.";
      } else {
        error = "Couldn't interpret that. Try naming what matters: shade, greenery, wind.";
      }
    } finally {
      loading = false;
    }
  }

  function plan() {
    if (!result) return;
    const next = { ...$plannerPreferences };
    for (const k of applied) next[k] = result.preferences[k];
    plannerPreferences.set(next);
    onplan();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) void interpret();
  }
</script>

{#snippet body()}
  <form class="px-4 pt-1" onsubmit={interpret}>
    <label class="block">
      <span class="sr-only">Describe your ride</span>
      <textarea
        bind:this={textarea}
        bind:value={text}
        rows="4"
        maxlength="500"
        placeholder="From Asakusa-bashi to Kiyosumi Garden, somewhere shady, wind at my back coming home."
        onkeydown={onKey}
        class="textarea"
      ></textarea>
    </label>
    <div class="flex items-center justify-between pt-1.5">
      <span class="text-2xs text-text-placeholder">Ask in English or 日本語</span>
      <button type="submit" disabled={loading || !text.trim()} class="btn-secondary px-3 text-13">
        {loading ? 'Interpreting…' : result ? 'Interpret again' : 'Interpret'}
      </button>
    </div>
  </form>

  {#if error}
    <div
      class="mx-4 mt-3 rounded-md bg-surface-elevated border border-hairline px-3 py-2.5 text-xs text-text-subtle"
    >
      {error}
    </div>
  {/if}

  {#if result}
    <div class="mt-3 h-px bg-hairline"></div>
    <p class="px-4 pt-3 text-13 text-text">{result.intent.summary}</p>

    <SectionLabel text="Preferences" class="px-4 pt-4 pb-1" />
    {#if applied.length === 0}
      <p class="px-4 text-xs text-text-subtle">No routing preferences to apply.</p>
    {:else}
      <div class="flex flex-wrap gap-2 px-4 -my-1.5">
        {#each applied as key (key)}
          <span class="pref-chip">
            {CHIP[key](result.preferences[key])}
            <button
              type="button"
              aria-label="Remove {CHIP[key](result.preferences[key])}"
              onclick={() => (dropped = new Set([...dropped, key]))}
              class="remove"
            >
              <Icon name="x" stroke={1.6} class="size-6 text-text-subtle" />
            </button>
          </span>
        {/each}
      </div>
    {/if}

    {#if result.unsupported.length > 0 || result.intent.unresolved_terms.length > 0}
      <SectionLabel text="Not supported yet" class="px-4 pt-5 pb-1.5" />
      <div class="mx-4 flex flex-col gap-1.5">
        {#each result.unsupported as key (key)}
          <div class="note">
            <Icon name="info" stroke={1.6} class="size-6 text-text-subtle" />
            <span>{UNSUPPORTED[key] ?? `“${key}” isn't supported yet`}</span>
          </div>
        {/each}
        {#if result.intent.unresolved_terms.length > 0}
          <div class="note">
            <Icon name="info" stroke={1.6} class="size-6 text-text-subtle" />
            <span
              >Couldn't interpret: {result.intent.unresolved_terms
                .map((t) => `“${t}”`)
                .join(', ')}.</span
            >
          </div>
        {/if}
      </div>
    {/if}

    <div class="px-4 pt-4 pb-4">
      <button type="button" onclick={plan} class="btn-primary w-50 text-13">Plan this route</button>
    </div>
    <div class="h-px bg-hairline"></div>
    <p class="px-4 py-3 text-2xs text-text-placeholder leading-relaxed">
      Places aren't picked out of the text yet — set them in Plan a route. Nothing is routed until
      you confirm. Interpreted by Claude.
    </p>
  {/if}
{/snippet}

{#if $isMobile}
  <Sheet label="Describe a ride" role="dialog" class="sheet-describe">
    <PanelHeader title="Describe a ride" {onback} {onclose} />
    {@render body()}
  </Sheet>
{:else}
  <div
    class="flex flex-col bg-surface-base border border-line rounded-lg shadow-lg max-h-full overflow-hidden"
  >
    <PanelHeader title="Describe a ride" {onback} {onclose} />
    <div class="min-h-0 overflow-y-auto pb-1" data-scroll="true">
      {@render body()}
    </div>
  </div>
{/if}

<style>
  /* Penpot "Textarea / ride description": 96px, radius 8, 14/1.45; the
     focused ring is 1.5px `focus`. */
  .textarea {
    display: block;
    width: 100%;
    min-height: 96px;
    padding: 10px 12px;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-line);
    background: var(--color-surface-base);
    color: var(--color-text);
    font-size: var(--text-sm);
    line-height: 1.45;
    resize: vertical;
  }
  .textarea::placeholder {
    color: var(--color-text-placeholder);
  }
  .textarea:focus {
    outline: none;
    border-color: var(--color-focus);
    box-shadow: inset 0 0 0 0.5px var(--color-focus);
  }
  /* Penpot "Chip / removable": 30px tint pill; the × is a 44px target. */
  .pref-chip {
    display: inline-flex;
    align-items: center;
    height: 44px;
    padding-left: 12px;
    font-size: var(--text-13);
    font-weight: 500;
    color: var(--color-text-strong);
    position: relative;
  }
  .pref-chip::before {
    content: '';
    position: absolute;
    inset: 7px 8px 7px 0;
    border-radius: 999px;
    background: var(--color-surface-tint);
    border: 1px solid var(--color-hairline);
  }
  .pref-chip > :global(*) {
    position: relative;
  }
  .remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    background: transparent;
  }
  .remove:focus-visible {
    outline: none;
    border-radius: 999px;
    box-shadow: inset 0 0 0 2px var(--color-focus);
  }
  .note {
    display: flex;
    gap: 8px;
    padding: 10px 12px;
    border-radius: var(--radius-md);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-hairline);
    font-size: var(--text-xs);
    line-height: 1.4;
    color: var(--color-text-subtle);
  }
  :global(.sheet-describe) {
    height: 72dvh;
  }
</style>
