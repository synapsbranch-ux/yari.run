<script lang="ts">
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import { serviceDetailQuery } from '#lib/data/queries.js';
  import { clockTime } from '#lib/format.js';
  import { cn } from '#lib/utils.js';
  import type { Service } from '#lib/data/types';

  let { service }: { service: Service } = $props();
  const detail = serviceDetailQuery(() => service.id);

  const LEVEL = {
    info: 'text-muted-foreground',
    warn: 'text-status-deploying',
    error: 'text-status-failed'
  } as const;
</script>

{#if detail.isPending}
  <p class="text-xs text-muted-foreground">Loading...</p>
{:else if !detail.data?.logs.length}
  <EmptyState title="No logs yet" description="Log lines of this service show up here." />
{:else}
  <div
    class="overflow-x-auto rounded-lg border border-border bg-surface p-3 font-mono text-[11.5px] leading-5"
  >
    {#each detail.data.logs as l, i (i)}
      <div class="flex gap-3 whitespace-pre">
        <time class="shrink-0 text-muted-foreground/70" datetime={l.at}>{clockTime(l.at)}</time>
        <span class={cn(LEVEL[l.level])}>{l.message}</span>
      </div>
    {/each}
  </div>
{/if}
