<!-- web/src/lib/components/ui/Button.svelte -->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLButtonAttributes } from 'svelte/elements';

  interface Props extends HTMLButtonAttributes {
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
    size?: 'sm' | 'md';
    /** When set, renders a toggle button with the correct aria-pressed state. */
    pressed?: boolean;
    class?: string;
    children: Snippet;
  }

  let {
    variant = 'secondary',
    size = 'md',
    pressed = undefined,
    type = 'button',
    class: extraClass = '',
    children,
    ...rest
  }: Props = $props();

  const variants: Record<string, string> = {
    primary: 'bg-accent text-surface border-transparent hover:bg-accent-strong',
    secondary:
      'bg-surface-raised text-text border-border hover:bg-surface-overlay hover:border-border-strong',
    ghost:
      'bg-transparent text-text-muted border-transparent hover:bg-surface-overlay hover:text-text',
    danger: 'bg-danger-surface text-danger border-danger/40 hover:bg-danger/20'
  };

  const sizes: Record<string, string> = {
    sm: 'text-xs gap-1 rounded-md px-2.5 py-1',
    md: 'text-sm gap-1.5 rounded-lg px-3.5 py-2'
  };
</script>

<button
  {type}
  aria-pressed={pressed}
  class="inline-flex items-center justify-center border font-medium transition-colors
         focus:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1 focus-visible:ring-offset-surface
         disabled:pointer-events-none disabled:opacity-50
         {variants[variant]} {sizes[size]} {extraClass}"
  {...rest}
>
  {@render children()}
</button>
