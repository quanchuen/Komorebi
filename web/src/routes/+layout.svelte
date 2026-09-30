<!-- web/src/routes/+layout.svelte -->
<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';

  let { children } = $props();

  // Controls are server-rendered before they can act. Flag the document once
  // the client has hydrated so browser tests (and any script) can wait for
  // interactivity instead of guessing.
  onMount(() => {
    document.documentElement.dataset.hydrated = 'true';
  });
</script>

<div class="h-dvh w-screen overflow-hidden">
  {@render children()}
</div>
