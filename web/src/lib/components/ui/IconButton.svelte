<!-- web/src/lib/components/ui/IconButton.svelte -->
<!-- Penpot "IconButton": a square control with one icon. `variant` picks the
     Penpot look (raised = white ring + xs shadow, ghost = no chrome, primary =
     near-black). The visible square is `size` px; the button itself is never
     smaller than 44×44 (tests/AGENTS.md), padding transparently around the
     visible square when Penpot draws it smaller. -->
<script lang="ts">
  import type { HTMLButtonAttributes } from 'svelte/elements';
  import Icon from './Icon.svelte';
  import type { IconName } from '$lib/utils/icons';

  interface Props extends HTMLButtonAttributes {
    icon: IconName;
    /** Accessible name; icon-only buttons must have one. */
    label: string;
    variant?: 'raised' | 'ghost' | 'primary';
    /** Visible square in px: 40 (map controls), 36 (panel headers), 32 (popover close). */
    size?: 40 | 36 | 32;
    stroke?: number;
    class?: string;
  }

  let {
    icon,
    label,
    variant = 'ghost',
    size = 36,
    stroke = 1.75,
    type = 'button',
    class: extraClass = '',
    ...rest
  }: Props = $props();

  const looks: Record<string, string> = {
    raised:
      'bg-surface-base border border-line shadow-xs text-text hover:bg-surface-tint hover:border-line-strong',
    ghost: 'bg-transparent border border-transparent text-text hover:bg-surface-tint',
    primary: 'bg-primary border border-primary text-on-primary hover:bg-primary-hover'
  };
  const squares: Record<number, string> = {
    40: 'size-10 rounded-control',
    36: 'size-9 rounded-md',
    32: 'size-8 rounded-md'
  };
</script>

<button
  {type}
  aria-label={label}
  title={label}
  class="group/ib inline-flex items-center justify-center min-w-11 min-h-11 shrink-0 bg-transparent
         focus:outline-none {extraClass}"
  {...rest}
>
  <span
    class="inline-flex items-center justify-center transition-colors
           group-focus-visible/ib:ring-2 group-focus-visible/ib:ring-focus
           {squares[size]} {looks[variant]}"
  >
    <Icon name={icon} {stroke} class="size-6" />
  </span>
</button>
