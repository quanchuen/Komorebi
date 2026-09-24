<!-- web/src/lib/components/ReviewList.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { reviews as reviewsApi } from '$lib/api/client';
  import type { Review } from '$lib/api/types';
  import AsyncBoundary from './ui/AsyncBoundary.svelte';

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
  <h3 class="text-sm font-semibold text-text-muted">Reviews</h3>

  <AsyncBoundary
    {loading}
    empty={items.length === 0}
    loadingMessage="Loading reviews…"
    emptyMessage="No reviews yet."
  >
    {#each items as review (review.id)}
      <div class="bg-surface-raised rounded-lg p-3 space-y-1">
        <div class="flex items-center gap-2">
          <span class="text-rating text-xs tracking-wide">{stars(review.rating)}</span>
          <span class="text-xs text-text-subtle"
            >{new Date(review.createdAt).toLocaleDateString()}</span
          >
        </div>
        <p class="text-sm text-text-muted leading-snug">{review.body}</p>
      </div>
    {/each}
  </AsyncBoundary>
</div>
