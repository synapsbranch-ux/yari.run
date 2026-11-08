<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import Screen from '#lib/components/mobile/Screen.svelte';
  import { sessionsQuery } from '#lib/data/queries.js';
  import { relativeTime, usdMicros } from '#lib/format.js';
  import { cn } from '#lib/utils.js';

  const sessions = sessionsQuery();

  const DOT: Record<string, string> = {
    running: 'bg-status-deploying animate-pulse',
    queued: 'bg-status-deploying',
    waiting_approval: 'bg-status-draft',
    succeeded: 'bg-status-online',
    failed: 'bg-status-failed',
    stopped: 'bg-status-sleeping'
  };
</script>

<svelte:head><title>Agent · yari.run</title></svelte:head>

<Screen title="Agent" subtitle="Sessions of your DevOps agent">
  {#if sessions.isPending}
    <p class="text-sm text-muted-foreground">Loading...</p>
  {:else if (sessions.data?.length ?? 0) === 0}
    <EmptyState
      title="No sessions yet"
      description="Start a session from the canvas on a larger screen."
    />
  {:else}
    <ul class="space-y-2">
      {#each sessions.data ?? [] as s (s.id)}
        <li>
          <a
            href="/m/agent/{s.id}"
            class="flex items-center gap-3 rounded-xl border border-border bg-card p-3.5"
          >
            <div class="min-w-0 flex-1">
              <p class="line-clamp-2 text-[13px] font-medium">{s.goal}</p>
              <p class="mt-1.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                <span class={cn('size-1.5 rounded-full', DOT[s.status])}></span>
                <span class="capitalize">{s.status.replace('_', ' ')}</span>
                <span>·</span><span>{s.environmentLabel}</span>
              </p>
              <p class="mt-0.5 text-[11px] text-muted-foreground">
                {s.model} · {usdMicros(s.spendUsdMicros)} · {relativeTime(s.startedAt)}
              </p>
            </div>
            <ChevronRight class="size-4 shrink-0 text-muted-foreground" />
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</Screen>
