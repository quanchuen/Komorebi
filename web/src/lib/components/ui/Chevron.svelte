<!-- web/src/lib/components/ui/Chevron.svelte -->
<!-- Fold/unfold indicator. Points the way the panel will move when pressed. -->
<script lang="ts">
  interface Props {
    direction?: 'up' | 'down' | 'left' | 'right';
    class?: string;
  }

  let { direction = 'down', class: extraClass = '' }: Props = $props();

  const rotation: Record<NonNullable<Props['direction']>, string> = {
    down: '',
    up: 'rotate-180',
    left: 'rotate-90',
    right: '-rotate-90'
  };
</script>

<svg
  class="w-4 h-4 shrink-0 transition-transform {rotation[direction]} {extraClass}"
  viewBox="0 0 16 16"
  fill="none"
  stroke="currentColor"
  stroke-width="1.75"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d="M4 6l4 4 4-4" />
</svg>
