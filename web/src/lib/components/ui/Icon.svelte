<!-- web/src/lib/components/ui/Icon.svelte -->
<!-- One 24-unit stroked icon from lib/utils/icons.ts, sized by the `size`
     class (default 24px) and coloured by the surrounding text token. -->
<script lang="ts">
  import { iconSvg, type IconName } from '$lib/utils/icons';

  interface Props {
    name: IconName;
    /** Penpot stroke: 1.75 on chrome, 1.6 inside flow panels, 2.2 on markers. */
    stroke?: number;
    class?: string;
  }

  let { name, stroke = 1.75, class: extraClass = 'size-6' }: Props = $props();
</script>

<span class="inline-flex shrink-0 [&>svg]:size-full {extraClass}" aria-hidden="true">
  <!-- eslint-disable-next-line svelte/no-at-html-tags -- static SVG from lib/utils/icons, no user input -->
  {@html iconSvg(name, stroke)}
</span>
