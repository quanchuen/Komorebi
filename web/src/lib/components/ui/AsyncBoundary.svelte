<!-- web/src/lib/components/ui/AsyncBoundary.svelte -->
<!--
  Wraps an async view so every loading / error / empty / loaded path renders
  through the shared states. This is the structural enforcement of "feedback
  for every action" — views compose this instead of hand-rolling spinners and
  error copy.
-->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import LoadingState from './LoadingState.svelte';
  import EmptyState from './EmptyState.svelte';
  import ErrorState from './ErrorState.svelte';

  interface Props {
    loading?: boolean;
    error?: string | null;
    empty?: boolean;
    loadingMessage?: string;
    emptyMessage?: string;
    errorTitle?: string;
    onRetry?: () => void;
    /** Loaded content. */
    children: Snippet;
    /** Optional custom empty-state content (overrides emptyMessage). */
    emptyContent?: Snippet;
  }

  let {
    loading = false,
    error = null,
    empty = false,
    loadingMessage = undefined,
    emptyMessage = undefined,
    errorTitle = undefined,
    onRetry = undefined,
    children,
    emptyContent
  }: Props = $props();
</script>

{#if error}
  <ErrorState title={errorTitle} message={error} {onRetry} />
{:else if loading}
  <LoadingState message={loadingMessage} />
{:else if empty}
  {#if emptyContent}
    {@render emptyContent()}
  {:else}
    <EmptyState message={emptyMessage} />
  {/if}
{:else}
  {@render children()}
{/if}
