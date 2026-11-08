<script lang="ts">
  import { serviceDetailQuery } from '#lib/data/queries.js';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import { relativeTime } from '#lib/format.js';
  import { cn } from '#lib/utils.js';
  import type { Service } from '#lib/data/types';

  let { service }: { service: Service } = $props();
  const detail = serviceDetailQuery(() => service.id);

  const DOT = {
    succeeded: 'bg-status-online',
    failed: 'bg-status-failed',
    in_progress: 'bg-status-deploying animate-pulse',
    rolled_back: 'bg-status-sleeping'
  } as const;
  const LABEL = {
    succeeded: 'Succeeded',
    failed: 'Failed',
    in_progress: 'In progress',
    rolled_back: 'Rolled back'
  } as const;
</script>

{#if detail.isPending}
  <p class="text-xs text-muted-foreground">Loading...</p>
{:else if !detail.data?.deployments.length}
  <EmptyState title="No deployments yet" description="Deployments of this service show up here." />
{:else}
  <ul class="space-y-1">
    {#each detail.data.deployments as d (d.id)}
      <li class="rounded-lg border border-border bg-card px-3.5 py-3">
        <div class="flex items-center justify-between gap-3">
          <p class="truncate text-[13px] font-medium">{d.message}</p>
          <span class="shrink-0 text-xs text-muted-foreground">{relativeTime(d.at)}</span>
        </div>
        <div class="mt-1.5 flex items-center gap-2 text-xs text-muted-foreground">
          <span class={cn('size-1.5 rounded-full', DOT[d.status])}></span>
          <span>{LABEL[d.status]}</span>
          <span>·</span>
          <code class="font-mono">{d.commit}</code>
          <span>·</span>
          <span>{d.by}</span>
          <span class="ml-auto tabular-nums">{d.durationSeconds}s</span>
        </div>
      </li>
    {/each}
  </ul>
{/if}
