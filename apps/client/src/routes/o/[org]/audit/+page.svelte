<script lang="ts">
  import Bot from '@lucide/svelte/icons/bot';
  import User from '@lucide/svelte/icons/user';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import { auditQuery } from '#lib/data/queries.js';
  import { relativeTime } from '#lib/format.js';
  import { getOrgContext } from '#lib/org-context.js';

  const ctx = getOrgContext();
  const events = auditQuery(() => ctx.org?.id);
</script>

<svelte:head><title>Audit log · yari.run</title></svelte:head>

<PageHeader
  title="Audit log"
  description="Every change made by a human or an agent, in order. Agents appear with the plan or session they acted for."
/>

{#if events.isPending}
  <p class="text-sm text-muted-foreground">Loading...</p>
{:else if events.isError}
  <p class="text-sm text-destructive">Could not load the audit log: {events.error.message}</p>
{:else if events.data.length === 0}
  <EmptyState title="No events yet" />
{:else}
  <div class="overflow-hidden rounded-xl border border-border bg-card">
    <ul>
      {#each events.data as e (e.id)}
        <li class="flex items-start gap-3 border-b border-border px-4 py-3 last:border-0">
          <span
            class="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"
          >
            {#if e.principal.kind === 'agent'}<Bot class="size-3.5" />{:else}<User
                class="size-3.5"
              />{/if}
          </span>
          <div class="min-w-0 flex-1">
            <p class="text-[13px]">
              <span class="font-medium">{e.principal.name}</span>
              <code class="mx-1 rounded bg-muted px-1.5 py-0.5 text-[11.5px]">{e.action}</code>
              <span class="text-muted-foreground">{e.target}</span>
            </p>
            <p class="mt-0.5 text-xs text-muted-foreground">
              {e.scope.name}{e.agentSessionId ? ` · agent session ${e.agentSessionId}` : ''}
            </p>
          </div>
          <time class="shrink-0 text-xs text-muted-foreground" datetime={e.occurredAt}>
            {relativeTime(e.occurredAt)}
          </time>
        </li>
      {/each}
    </ul>
  </div>
{/if}
