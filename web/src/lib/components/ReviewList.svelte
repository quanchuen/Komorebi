<!-- web/src/lib/components/ReviewList.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { reviews as reviewsApi } from '$lib/api/client';
  import type { Review } from '$lib/api/types';

  interface Props {
    routeId: string;
  }

  let { routeId }: Props = $props();

  let items = $state<Review[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      const res = await reviewsApi.list(routeId);
      items = res.reviews;
    } catch {
      // no reviews or API unavailable
    } finally {
      loading = false;
    }
  });

  function stars(n: number): string {
    return '★'.repeat(n) + '☆'.repeat(5 - n);
  }
</script>

<div class="space-y-3">
  <h3 class="text-sm font-semibold text-slate-300">Reviews</h3>

  {#if loading}
    <div class="text-slate-500 text-sm">Loading reviews…</div>
  {:else if items.length === 0}
    <div class="text-slate-500 text-sm">No reviews yet.</div>
  {:else}
    {#each items as review (review.id)}
      <div class="bg-slate-800 rounded-lg p-3 space-y-1">
        <div class="flex items-center gap-2">
          <span class="text-yellow-400 text-xs tracking-wide">{stars(review.rating)}</span>
          <span class="text-xs text-slate-500">{new Date(review.createdAt).toLocaleDateString()}</span>
        </div>
        <p class="text-sm text-slate-300 leading-snug">{review.body}</p>
      </div>
    {/each}
  {/if}
</div>
