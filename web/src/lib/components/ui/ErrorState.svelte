<!-- web/src/lib/components/ui/ErrorState.svelte -->
<script lang="ts">
  import Button from './Button.svelte';

  interface Props {
    title?: string;
    message?: string;
    onRetry?: () => void;
  }

  let { title = 'Something went wrong', message = '', onRetry }: Props = $props();
</script>

<div
  class="space-y-2 rounded-lg border border-danger/40 bg-danger-surface p-4 text-center"
  role="alert"
>
  <div class="text-sm font-medium text-danger">{title}</div>
  {#if message}
    <div class="text-xs text-danger/80">{message}</div>
  {/if}
  {#if onRetry}
    <Button variant="danger" size="sm" onclick={onRetry}>Retry</Button>
  {/if}
</div>
