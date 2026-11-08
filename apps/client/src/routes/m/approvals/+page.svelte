<script lang="ts">
  import Bot from '@lucide/svelte/icons/bot';
  import Lock from '@lucide/svelte/icons/lock';
  import User from '@lucide/svelte/icons/user';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import Money from '#lib/components/common/Money.svelte';
  import RiskBadge from '#lib/components/common/RiskBadge.svelte';
  import Screen from '#lib/components/mobile/Screen.svelte';
  import PlanOverview from '#lib/components/plan/PlanOverview.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import * as Sheet from '#lib/components/ui/sheet/index.js';
  import { approvalMutations, approvalsQuery } from '#lib/data/queries.js';
  import { relativeTime } from '#lib/format.js';
  import type { Plan } from '#lib/data/types';

  const approvals = approvalsQuery();
  const { approve, reject } = approvalMutations();

  let openId = $state<string | null>(null);
  let confirming = $state<'approve' | 'reject' | null>(null);

  const open = $derived(approvals.data?.find((p) => p.id === openId));
  const busy = $derived(approve.isPending || reject.isPending);

  function close() {
    openId = null;
    confirming = null;
  }

  function confirm(plan: Plan) {
    const m = confirming === 'approve' ? approve : reject;
    m.mutate(plan.id, { onSuccess: close });
  }
</script>

<svelte:head><title>Approvals · yari.run</title></svelte:head>

<Screen title="Approvals" subtitle="Plans waiting for you">
  {#if approvals.isPending}
    <p class="text-sm text-muted-foreground">Loading...</p>
  {:else if (approvals.data?.length ?? 0) === 0}
    <EmptyState title="All clear" description="Nothing is waiting for approval." />
  {:else}
    <ul class="space-y-2">
      {#each approvals.data ?? [] as p (p.id)}
        <li>
          <button
            type="button"
            onclick={() => (openId = p.id)}
            class="w-full rounded-xl border border-border bg-card p-3.5 text-left"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="inline-flex items-center gap-1 text-xs text-muted-foreground">
                {p.projectName} / {#if p.environmentProtected}<Lock
                    class="size-3"
                  />{/if}{p.environmentName}
              </span>
              <span class="text-[11px] text-muted-foreground">{relativeTime(p.createdAt)}</span>
            </div>
            <p class="mt-1.5 text-[13px] font-medium">{p.summary}</p>
            <div class="mt-2.5 flex items-center justify-between text-xs">
              <span class="inline-flex items-center gap-1.5 text-muted-foreground">
                {#if p.proposedBy.kind === 'agent'}<Bot class="size-3.5" />{:else}<User
                    class="size-3.5"
                  />{/if}
                {p.proposedBy.name}
              </span>
              <span class="flex items-center gap-3"
                ><Money cents={p.costDeltaCents} /><RiskBadge risk={p.risk} /></span
              >
            </div>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</Screen>

<Sheet.Root open={!!open} onOpenChange={(v) => !v && close()}>
  <Sheet.Content side="bottom" showCloseButton={false} class="max-h-[88dvh] gap-0 rounded-t-2xl">
    {#if open}
      <div class="mx-auto mt-2 h-1 w-9 rounded-full bg-border" aria-hidden="true"></div>
      <Sheet.Title class="sr-only">Review plan</Sheet.Title>
      <Sheet.Description class="sr-only">Approve or reject this plan</Sheet.Description>
      <div class="min-h-0 flex-1 overflow-y-auto p-4">
        {#if confirming}
          <div class="py-4 text-center">
            <p class="text-base font-semibold">
              {confirming === 'approve' ? 'Approve this plan?' : 'Reject this plan?'}
            </p>
            <p class="mx-auto mt-2 max-w-xs text-xs leading-relaxed text-muted-foreground">
              {#if confirming === 'approve'}
                yari.run will apply {open.changes.length}
                {open.changes.length === 1 ? 'change' : 'changes'}
                to {open.projectName} / {open.environmentName}.
              {:else}
                The plan is cancelled and nothing changes in {open.environmentName}.
              {/if}
            </p>
          </div>
        {:else}
          <PlanOverview plan={open} />
        {/if}
      </div>
      <div class="pb-safe-4 grid grid-cols-2 gap-2 border-t border-border px-4 pt-4">
        {#if confirming}
          <Button variant="outline" onclick={() => (confirming = null)} disabled={busy}>Back</Button
          >
          <Button
            variant={confirming === 'approve' ? 'default' : 'destructive'}
            onclick={() => confirm(open)}
            disabled={busy}
          >
            {confirming === 'approve' ? 'Confirm approval' : 'Confirm rejection'}
          </Button>
        {:else}
          <Button variant="outline" onclick={() => (confirming = 'reject')}>Reject</Button>
          <Button onclick={() => (confirming = 'approve')}>Approve</Button>
        {/if}
      </div>
    {/if}
  </Sheet.Content>
</Sheet.Root>
