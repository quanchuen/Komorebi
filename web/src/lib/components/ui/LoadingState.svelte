<!-- web/src/lib/components/ui/LoadingState.svelte -->
<script lang="ts">
  interface Props {
    message?: string;
  }

  let { message = 'Loading…' }: Props = $props();
</script>

<div
  class="flex items-center justify-center gap-2 py-4 text-sm text-text-subtle"
  role="status"
  aria-live="polite"
>
  <span
    class="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-line-strong border-t-primary"
    aria-hidden="true"
  ></span>
  {message}
</div>
