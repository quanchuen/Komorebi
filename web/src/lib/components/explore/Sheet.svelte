<!-- web/src/lib/components/explore/Sheet.svelte -->
<!-- Mobile bottom sheet (Penpot "Sheet / …"): full width, 20px top corners,
     1px line ring, lg shadow, a 40×4 grabber. The body scrolls inside the
     sheet (data-scroll) so the map behind never moves with it. -->
<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    label: string;
    role?: 'dialog' | 'region';
    /** Tailwind height/max-height classes for the sheet. */
    class?: string;
    children: Snippet;
  }

  let { label, role = 'region', class: extraClass = '', children }: Props = $props();
</script>

<div
  {role}
  aria-label={label}
  aria-modal={role === 'dialog' ? 'true' : undefined}
  class="absolute inset-x-0 bottom-0 z-30 flex flex-col bg-surface-base border border-line
         rounded-t-2xl shadow-lg {extraClass}"
>
  <div class="shrink-0 flex justify-center pt-2 pb-1" aria-hidden="true">
    <span class="block w-10 h-1 rounded-pill bg-fill"></span>
  </div>
  <div class="min-h-0 flex-1 overflow-y-auto" data-scroll="true">
    {@render children()}
  </div>
</div>
