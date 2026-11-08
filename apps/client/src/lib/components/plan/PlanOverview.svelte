<script lang="ts">
  import Bot from '@lucide/svelte/icons/bot';
  import Lock from '@lucide/svelte/icons/lock';
  import User from '@lucide/svelte/icons/user';
  import Money from '#lib/components/common/Money.svelte';
  import RiskBadge from '#lib/components/common/RiskBadge.svelte';
  import { relativeTime } from '#lib/format.js';
  import PlanChanges from './PlanChanges.svelte';
  import type { Plan } from '#lib/data/types';

  let { plan }: { plan: Plan } = $props();
  const byAgent = $derived(plan.proposedBy.kind === 'agent');
</script>

<div class="space-y-5">
  <div>
    <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
      <span>{plan.projectName}</span>
      <span>/</span>
      <span class="inline-flex items-center gap-1 text-foreground">
        {#if plan.environmentProtected}<Lock class="size-3" />{/if}{plan.environmentName}
      </span>
    </div>
    <p class="mt-1.5 text-sm font-medium">{plan.summary}</p>
    <p class="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
      <span class="inline-flex items-center gap-1.5">
        {#if byAgent}<Bot class="size-3.5" />{:else}<User class="size-3.5" />{/if}
        Proposed by {plan.proposedBy.name}
        <span class="rounded-full border border-border px-1.5 py-px text-[10.5px]">
          {byAgent ? 'agent' : 'human'}
        </span>
      </span>
      <span>·</span>
      <span>{relativeTime(plan.createdAt)}</span>
      {#if byAgent && plan.agentSessionId}
        <span>·</span><span class="font-mono">{plan.agentSessionId}</span>
      {/if}
    </p>
  </div>

  <div class="grid grid-cols-2 gap-3">
    <div class="rounded-lg border border-border bg-card px-3.5 py-3">
      <p class="text-xs text-muted-foreground">Monthly cost delta</p>
      <p class="mt-1 text-lg font-semibold tracking-tight"><Money cents={plan.costDeltaCents} /></p>
    </div>
    <div class="rounded-lg border border-border bg-card px-3.5 py-3">
      <p class="text-xs text-muted-foreground">Risk</p>
      <p class="mt-1.5"><RiskBadge risk={plan.risk} /></p>
    </div>
  </div>

  <div>
    <h3 class="mb-2 text-xs font-medium text-muted-foreground">
      {plan.changes.length}
      {plan.changes.length === 1 ? 'change' : 'changes'}
    </h3>
    <PlanChanges changes={plan.changes} />
  </div>
</div>
