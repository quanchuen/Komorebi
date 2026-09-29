<!-- web/src/lib/components/ui/Icon.svelte -->
<!-- Line icons (Lucide, ISC licence) on a 24×24 grid, stroked in currentColor. -->
<script lang="ts" module>
  const PATHS = {
    bike: [
      'M5.5 14a3.5 3.5 0 1 0 0 7 3.5 3.5 0 1 0 0-7z',
      'M18.5 14a3.5 3.5 0 1 0 0 7 3.5 3.5 0 1 0 0-7z',
      'M15 4a1 1 0 1 0 0 2 1 1 0 1 0 0-2z',
      'M12 17.5V14l-3-3 4-3 2 3h2'
    ],
    info: ['M12 2a10 10 0 1 0 0 20 10 10 0 1 0 0-20z', 'M12 16v-4', 'M12 8h.01'],
    signpost: [
      'M12 13v8',
      'M12 3v3',
      'M18 6a2 2 0 0 1 1.387.56l2.307 2.22a1 1 0 0 1 0 1.44l-2.307 2.22A2 2 0 0 1 18 13H6a2 2 0 0 1-1.387-.56l-2.306-2.22a1 1 0 0 1 0-1.44l2.306-2.22A2 2 0 0 1 6 6z'
    ],
    'circle-minus': ['M12 2a10 10 0 1 0 0 20 10 10 0 1 0 0-20z', 'M8 12h8'],
    smartphone: [
      'M7 2h10a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2z',
      'M12 18h.01'
    ],
    shield: [
      'M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z'
    ],
    x: ['M18 6 6 18', 'M6 6l12 12'],
    'external-link': [
      'M15 3h6v6',
      'M10 14 21 3',
      'M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6'
    ],
    leaf: [
      'M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19 2c1 2 2 4.18 2 8 0 5.5-4.78 10-10 10Z',
      'M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12'
    ]
  } as const;

  export type IconName = keyof typeof PATHS;
</script>

<script lang="ts">
  interface Props {
    name: IconName;
    class?: string;
  }

  let { name, class: extraClass = 'size-4' }: Props = $props();
</script>

<svg
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.75"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  class={extraClass}
>
  {#each PATHS[name] as d (d)}
    <path {d} />
  {/each}
</svg>
