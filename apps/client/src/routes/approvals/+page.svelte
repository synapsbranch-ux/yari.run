<script lang="ts">
  import Bot from '@lucide/svelte/icons/bot';
  import Check from '@lucide/svelte/icons/check';
  import Lock from '@lucide/svelte/icons/lock';
  import User from '@lucide/svelte/icons/user';
  import X from '@lucide/svelte/icons/x';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import Money from '#lib/components/common/Money.svelte';
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import RiskBadge from '#lib/components/common/RiskBadge.svelte';
  import PlanOverview from '#lib/components/plan/PlanOverview.svelte';
  import OrgShell from '#lib/components/shell/OrgShell.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import { Skeleton } from '#lib/components/ui/skeleton/index.js';
  import { approvalMutations, approvalsQuery, organizationsQuery } from '#lib/data/queries.js';
  import { relativeTime } from '#lib/format.js';
  import { cn } from '#lib/utils.js';

  const orgs = organizationsQuery();
  const approvals = approvalsQuery();
  const { approve, reject } = approvalMutations();

  const orgSlug = $derived(orgs.data?.[0]?.slug ?? 'acme');
  let selectedId = $state<string | null>(null);

  const plans = $derived(approvals.data ?? []);
  const selected = $derived(plans.find((p) => p.id === selectedId) ?? plans[0]);
  const busy = $derived(approve.isPending || reject.isPending);
  const error = $derived(approve.error ?? reject.error);
</script>

<svelte:head><title>Approvals · yari.run</title></svelte:head>

<OrgShell {orgSlug}>
  <PageHeader
    title="Approvals"
    description="Plans waiting for a second pair of eyes. Agents propose, humans approve."
  />

  {#if approvals.isPending}
    <div class="grid gap-4 lg:grid-cols-[320px_1fr]">
      <Skeleton class="h-72 rounded-xl" /><Skeleton class="h-72 rounded-xl" />
    </div>
  {:else if approvals.isError}
    <p class="text-sm text-destructive">Could not load approvals: {approvals.error.message}</p>
  {:else if plans.length === 0}
    <EmptyState title="Nothing to approve" description="Plans that need approval show up here." />
  {:else}
    <div class="grid items-start gap-5 lg:grid-cols-[320px_1fr]">
      <ul class="space-y-2">
        {#each plans as p (p.id)}
          <li>
            <button
              type="button"
              onclick={() => (selectedId = p.id)}
              class={cn(
                'w-full rounded-xl border border-border bg-card p-3.5 text-left transition-colors hover:border-foreground/20',
                selected?.id === p.id && 'border-primary/50 bg-accent/40'
              )}
            >
              <div class="flex items-center justify-between gap-2">
                <span class="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  {p.projectName} / {#if p.environmentProtected}<Lock
                      class="size-3"
                    />{/if}{p.environmentName}
                </span>
                <span class="text-[11px] text-muted-foreground">{relativeTime(p.createdAt)}</span>
              </div>
              <p class="mt-1.5 line-clamp-2 text-[13px] font-medium">{p.summary}</p>
              <div class="mt-2.5 flex items-center justify-between gap-2">
                <span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                  {#if p.proposedBy.kind === 'agent'}<Bot class="size-3.5" />{:else}<User
                      class="size-3.5"
                    />{/if}
                  {p.proposedBy.name}
                </span>
                <span class="flex items-center gap-3 text-xs">
                  <Money cents={p.costDeltaCents} />
                  <RiskBadge risk={p.risk} />
                </span>
              </div>
            </button>
          </li>
        {/each}
      </ul>

      {#if selected}
        <section class="rounded-xl border border-border bg-surface">
          <div class="p-5">
            <PlanOverview plan={selected} />
          </div>
          <div class="flex items-center justify-between gap-3 border-t border-border px-5 py-3.5">
            <p class="text-xs text-muted-foreground">
              {#if error}<span class="text-destructive">{error.message}</span>
              {:else if selected.environmentProtected}Requires approval from an admin.
              {:else}Approving lets yari.run apply this plan.{/if}
            </p>
            <div class="flex shrink-0 gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={busy}
                onclick={() => reject.mutate(selected.id)}
              >
                <X class="size-3.5" /> Reject
              </Button>
              <Button size="sm" disabled={busy} onclick={() => approve.mutate(selected.id)}>
                <Check class="size-3.5" /> Approve
              </Button>
            </div>
          </div>
        </section>
      {/if}
    </div>
  {/if}
</OrgShell>
