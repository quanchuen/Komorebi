<!-- web/src/routes/+layout.svelte -->
<script lang="ts">
  import '../app.css';
  import DataSourcesSheet from '$lib/components/DataSourcesSheet.svelte';
  import RideDisclaimer from '$lib/components/RideDisclaimer.svelte';

  let { children } = $props();
</script>

<div class="h-dvh w-screen overflow-hidden">
  {@render children()}
</div>

<RideDisclaimer />
<!-- After the disclaimer so the sheet stacks above it when opened from there. -->
<DataSourcesSheet />
