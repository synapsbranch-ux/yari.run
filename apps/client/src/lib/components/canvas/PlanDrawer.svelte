<script lang="ts">
  import { createMutation, useQueryClient } from '@tanstack/svelte-query';
  import Check from '@lucide/svelte/icons/check';
  import Lock from '@lucide/svelte/icons/lock';
  import Rocket from '@lucide/svelte/icons/rocket';
  import X from '@lucide/svelte/icons/x';
  import { getCanvasState } from '#lib/canvas.svelte.js';
  import PlanOverview from '#lib/components/plan/PlanOverview.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import * as Sheet from '#lib/components/ui/sheet/index.js';
  import { data } from '#lib/data/index.js';
  import { keys, planQuery, stagedPlanQuery } from '#lib/data/queries.js';
  import type { HierarchyContext, Plan, PlanChange, Risk } from '#lib/data/types';

  let { ctx }: { ctx: HierarchyContext } = $props();

  const canvas = getCanvasState();
  const client = useQueryClient();

  const view = $derived(canvas.planView);
  const staged = stagedPlanQuery(() => ctx.environment.id);
  const reviewed = planQuery(() => (view?.mode === 'review' ? view.planId : undefined));

  const RANK: Record<Risk, number> = { low: 1, medium: 2, high: 3 };

  function synth(changes: PlanChange[], summary: string): Plan {
    const risk = changes.reduce<Risk>((r, c) => (RANK[c.risk] > RANK[r] ? c.risk : r), 'low');
    return {
      id: 'plan_new',
      organizationId: ctx.organization.id,
      organizationSlug: ctx.organization.slug,
      workspaceSlug: ctx.workspace.slug,
      environmentId: ctx.environment.id,
      projectName: ctx.project.name,
      projectSlug: ctx.project.slug,
      environmentName: ctx.environment.name,
      environmentSlug: ctx.environment.slug,
      environmentProtected: ctx.environment.protected,
      status: 'draft',
      proposedBy: { id: 'you', name: 'You', kind: 'human' },
      summary,
      changes,
      costDeltaCents: changes.reduce((s, c) => s + c.costDeltaCents, 0),
      risk,
      createdAt: new Date().toISOString()
    };
  }

  const plan = $derived.by<Plan | null>(() => {
    if (!view) return null;
    if (view.mode === 'review') return reviewed.data ?? null;
    if (canvas.drafts.length > 0) {
      const n = canvas.drafts.length;
      return synth(
        canvas.draftChanges(),
        `Create ${n} new ${n === 1 ? 'service' : 'services'} from the canvas.`
      );
    }
    return staged.data ?? null;
  });

  const loading = $derived(view?.mode === 'review' ? reviewed.isPending : staged.isPending);

  let sent = $state<{ requiresApproval: boolean; id: string } | null>(null);

  const submit = createMutation(() => ({
    mutationFn: (changes: PlanChange[]) => data.submitPlan(ctx.environment.id, changes),
    onSuccess: ({ plan: created, requiresApproval }) => {
      sent = { requiresApproval, id: created.id };
      client.invalidateQueries({ queryKey: keys.approvals });
    }
  }));

  const decide = createMutation(() => ({
    mutationFn: async (v: { id: string; approve: boolean }) =>
      v.approve ? data.approvePlan(v.id) : data.rejectPlan(v.id),
    onSuccess: (_r, v) => {
      client.invalidateQueries({ queryKey: keys.approvals });
      client.invalidateQueries({ queryKey: keys.plan(v.id) });
      canvas.planView = null;
    }
  }));

  function close(open: boolean) {
    if (!open) {
      canvas.planView = null;
      sent = null;
      submit.reset();
    }
  }
</script>

<Sheet.Root open={view !== null} onOpenChange={close}>
  <Sheet.Content side="right" showCloseButton={false} class="gap-0 sm:max-w-[520px]">
    <header class="flex items-start justify-between gap-3 border-b border-border px-5 py-4">
      <div>
        <Sheet.Title class="text-sm font-semibold">
          {view?.mode === 'review' ? 'Review plan' : 'Deploy'}
        </Sheet.Title>
        <Sheet.Description class="mt-0.5 text-xs text-muted-foreground">
          {view?.mode === 'review'
            ? 'A plan proposed for this environment.'
            : 'Review what yari.run will change before anything runs.'}
        </Sheet.Description>
      </div>
      <Button variant="ghost" size="icon-sm" onclick={() => close(false)} aria-label="Close">
        <X class="size-4" />
      </Button>
    </header>

    <div class="flex-1 overflow-y-auto p-5">
      {#if sent}
        <div class="rounded-lg border border-border bg-card p-5">
          <span
            class="flex size-8 items-center justify-center rounded-full bg-status-online/15 text-status-online"
          >
            <Check class="size-4" />
          </span>
          <p class="mt-3 text-sm font-medium">
            {sent.requiresApproval ? 'Sent for approval' : 'Plan is applying'}
          </p>
          <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
            {sent.requiresApproval
              ? `Plan ${sent.id} needs approval from an admin before yari.run touches ${ctx.environment.name}.`
              : `Plan ${sent.id} is being applied to ${ctx.environment.name}.`}
          </p>
          {#if sent.requiresApproval}
            <Button href="/approvals" variant="outline" size="sm" class="mt-4"
              >Open approvals</Button
            >
          {/if}
        </div>
      {:else if loading}
        <p class="text-xs text-muted-foreground">Loading plan...</p>
      {:else if plan}
        <PlanOverview {plan} />
      {:else}
        <div class="rounded-lg border border-dashed border-border p-8 text-center">
          <p class="text-sm font-medium">Nothing to deploy</p>
          <p class="mx-auto mt-1 max-w-xs text-xs text-muted-foreground">
            Press <kbd class="rounded bg-muted px-1">Ctrl K</kbd> to add a service to the canvas, then
            come back here to review the plan.
          </p>
        </div>
      {/if}
    </div>

    {#if plan && !sent}
      <footer class="space-y-3 border-t border-border px-5 py-4">
        {#if plan.environmentProtected}
          <p class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Lock class="size-3.5 shrink-0" />
            Requires approval from an admin.
          </p>
        {/if}
        {#if submit.error || decide.error}
          <p class="text-xs text-destructive">{(submit.error ?? decide.error)?.message}</p>
        {/if}
        <div class="flex justify-end gap-2">
          {#if view?.mode === 'review'}
            {#if plan.status === 'pending_approval'}
              <Button
                variant="outline"
                size="sm"
                disabled={decide.isPending}
                onclick={() => decide.mutate({ id: plan.id, approve: false })}>Reject</Button
              >
              <Button
                size="sm"
                disabled={decide.isPending}
                onclick={() => decide.mutate({ id: plan.id, approve: true })}>Approve</Button
              >
            {:else}
              <span class="text-xs text-muted-foreground capitalize"
                >{plan.status.replace('_', ' ')}</span
              >
            {/if}
          {:else}
            <Button variant="outline" size="sm" onclick={() => close(false)}>Cancel</Button>
            <Button
              size="sm"
              disabled={submit.isPending}
              onclick={() => submit.mutate(plan.changes)}
            >
              <Rocket class="size-3.5" />
              {plan.environmentProtected ? 'Request approval' : 'Apply plan'}
            </Button>
          {/if}
        </div>
      </footer>
    {/if}
  </Sheet.Content>
</Sheet.Root>
