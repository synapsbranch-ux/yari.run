<script lang="ts">
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import { serviceDetailQuery } from '#lib/data/queries.js';
  import Sparkline from '../Sparkline.svelte';
  import type { Service } from '#lib/data/types';

  let { service }: { service: Service } = $props();
  const detail = serviceDetailQuery(() => service.id);
</script>

{#if detail.isPending}
  <p class="text-xs text-muted-foreground">Loading...</p>
{:else if !detail.data?.metrics.length}
  <EmptyState title="No metrics yet" />
{:else}
  <p class="mb-3 text-xs text-muted-foreground">Last 24 hours</p>
  <div class="grid grid-cols-2 gap-3">
    {#each detail.data.metrics as m (m.label)}
      {@const last = m.points[m.points.length - 1]}
      <div class="rounded-lg border border-border bg-card p-3">
        <p class="text-xs text-muted-foreground">{m.label}</p>
        <p class="mt-0.5 text-sm font-medium tabular-nums">
          {last}<span class="ml-1 text-xs font-normal text-muted-foreground">{m.unit}</span>
        </p>
        <Sparkline points={m.points} class="mt-2" />
      </div>
    {/each}
  </div>
{/if}
