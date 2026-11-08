<script lang="ts">
  import Bot from '@lucide/svelte/icons/bot';
  import User from '@lucide/svelte/icons/user';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import Screen from '#lib/components/mobile/Screen.svelte';
  import { auditQuery, organizationsQuery } from '#lib/data/queries.js';
  import { relativeTime } from '#lib/format.js';

  const orgs = organizationsQuery();
  const events = auditQuery(() => orgs.data?.[0]?.id);
</script>

<svelte:head><title>Activity · yari.run</title></svelte:head>

<Screen title="Activity" subtitle="What humans and agents changed">
  {#if events.isPending}
    <p class="text-sm text-muted-foreground">Loading...</p>
  {:else if (events.data?.length ?? 0) === 0}
    <EmptyState title="No activity yet" />
  {:else}
    <ul class="divide-y divide-border overflow-hidden rounded-xl border border-border bg-card">
      {#each events.data ?? [] as e (e.id)}
        <li class="flex items-start gap-3 px-3.5 py-3">
          <span
            class="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"
          >
            {#if e.principal.kind === 'agent'}<Bot class="size-3.5" />{:else}<User
                class="size-3.5"
              />{/if}
          </span>
          <div class="min-w-0 flex-1">
            <p class="text-[13px] leading-snug">
              <span class="font-medium">{e.principal.name}</span>
              <span class="text-muted-foreground"> {e.action}</span>
            </p>
            <p class="mt-0.5 truncate text-xs text-muted-foreground">{e.target} · {e.scope.name}</p>
          </div>
          <time class="shrink-0 text-[11px] text-muted-foreground" datetime={e.occurredAt}
            >{relativeTime(e.occurredAt)}</time
          >
        </li>
      {/each}
    </ul>
  {/if}
</Screen>
