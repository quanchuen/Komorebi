<!-- web/src/lib/components/explore/SectionLabel.svelte -->
<!-- "SHOW ON MAP", "RECENT", "CONDITIONS · FOLLOW THE TIME SLIDER": 11px,
     semibold, 0.6px tracking, text.subtle. -->
<script lang="ts">
  interface Props {
    text: string;
    class?: string;
  }
  let { text, class: extraClass = '' }: Props = $props();
</script>

<div class="text-2xs font-semibold tracking-wider uppercase text-text-subtle {extraClass}">
  {text}
</div>
