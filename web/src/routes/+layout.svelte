<!-- web/src/routes/+layout.svelte -->
<script lang="ts">
  import '../app.css';

  let { children } = $props();
</script>

<div class="h-dvh w-screen overflow-hidden">
  {@render children()}
</div>
