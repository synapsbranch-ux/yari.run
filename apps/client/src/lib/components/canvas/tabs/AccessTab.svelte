<script lang="ts">
  import { SvelteMap } from 'svelte/reactivity';
  import Bot from '@lucide/svelte/icons/bot';
  import Cpu from '@lucide/svelte/icons/cpu';
  import ShieldAlert from '@lucide/svelte/icons/shield-alert';
  import User from '@lucide/svelte/icons/user';
  import { grantsQuery, membersQuery } from '#lib/data/queries.js';
  import { cn } from '#lib/utils.js';
  import type { Grant, HierarchyContext, Member, Role, Service } from '#lib/data/types';

  let { service, ctx }: { service: Service; ctx: HierarchyContext } = $props();

  const grants = grantsQuery(() => ctx.organization.id);
  const members = membersQuery(() => ctx.organization.id);

  const RANK: Record<Role, number> = { viewer: 1, developer: 2, admin: 3, owner: 4 };

  /** Levels of the chain that carry down to this service. */
  const chain = $derived([
    { id: ctx.organization.id, type: 'Organization', name: ctx.organization.name },
    { id: ctx.workspace.id, type: 'Workspace', name: ctx.workspace.name },
    { id: ctx.project.id, type: 'Project', name: ctx.project.name },
    { id: ctx.environment.id, type: 'Environment', name: ctx.environment.name }
  ]);

  const rows = $derived.by(() => {
    const byPrincipal = new SvelteMap<string, { member: Member | undefined; grants: Grant[] }>();
    for (const g of grants.data ?? []) {
      if (!chain.some((c) => c.id === g.scope.id)) continue;
      const entry = byPrincipal.get(g.principalId) ?? {
        member: members.data?.find((m) => m.id === g.principalId),
        grants: []
      };
      entry.grants.push(g);
      byPrincipal.set(g.principalId, entry);
    }
    return [...byPrincipal.entries()].map(([id, e]) => {
      const best = e.grants.reduce((a, b) => (RANK[b.role] > RANK[a.role] ? b : a));
      const level = chain.find((c) => c.id === best.scope.id);
      const kind = e.member?.kind ?? 'human';
      return {
        id,
        name: e.member?.name ?? id.slice(0, 8),
        kind,
        role: best.role,
        from: level,
        capabilities: capabilities(kind, best.role, ctx.environment.protected)
      };
    });
  });

  function capabilities(kind: string, role: Role, isProtected: boolean) {
    const rank = RANK[role];
    const apply =
      rank < 2
        ? 'Cannot apply'
        : kind === 'agent' && isProtected
          ? 'Apply needs approval'
          : role === 'developer' && isProtected
            ? 'Apply needs approval'
            : 'Can apply';
    const secrets =
      rank < 2 || kind === 'agent'
        ? 'No secret.read'
        : isProtected && rank < 3
          ? 'No secret.read here'
          : 'secret.read';
    const approve = rank >= 3 && kind !== 'agent' ? 'Can approve' : '';
    return [apply, secrets, approve].filter(Boolean);
  }
</script>

<div class="space-y-5">
  <p class="text-xs leading-relaxed text-muted-foreground">
    Who and what can touch <span class="font-medium text-foreground">{service.name}</span>. Roles
    are granted at any level and carry down; the highest role on the chain wins.
  </p>

  <ol
    class="flex flex-wrap items-center gap-x-1.5 gap-y-1 rounded-lg border border-border bg-card px-3 py-2 text-xs"
  >
    {#each chain as c, i (c.id)}
      {#if i > 0}<span class="text-muted-foreground/50">›</span>{/if}
      <li>
        <span class="text-muted-foreground">{c.type}</span>
        <span class="font-medium">{c.name}</span>
      </li>
    {/each}
  </ol>

  <div
    class="flex items-start gap-2.5 rounded-lg border border-status-failed/30 bg-status-failed/5 p-3 text-xs leading-relaxed"
  >
    <ShieldAlert class="mt-0.5 size-4 shrink-0 text-status-failed" />
    <p>
      <span class="font-medium">Agents never get <code>secret.read</code>.</span>
      <span class="text-muted-foreground">
        They read logs and state, then propose Plans. {ctx.environment.protected
          ? 'On this protected environment a human admin has to approve every Plan an agent proposes.'
          : ''}
      </span>
    </p>
  </div>

  {#if grants.isPending}
    <p class="text-xs text-muted-foreground">Loading...</p>
  {:else if rows.length === 0}
    <p class="rounded-lg border border-dashed border-border p-4 text-xs text-muted-foreground">
      No grants apply to this service yet.
    </p>
  {:else}
    <ul class="space-y-2">
      {#each rows as r (r.id)}
        <li class="rounded-lg border border-border bg-card p-3.5">
          <div class="flex items-center gap-3">
            <span
              class="flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"
            >
              {#if r.kind === 'agent'}<Bot class="size-3.5" />
              {:else if r.kind === 'ci' || r.kind === 'api_key'}<Cpu class="size-3.5" />
              {:else}<User class="size-3.5" />{/if}
            </span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-[13px] font-medium">{r.name}</p>
              <p class="text-xs text-muted-foreground">
                <span class="text-foreground capitalize">{r.role}</span>
                {#if r.from}<span>
                    · inherited from {r.from.type.toLowerCase()} {r.from.name}</span
                  >{/if}
              </p>
            </div>
          </div>
          <div class="mt-2.5 flex flex-wrap gap-1.5">
            {#each r.capabilities as cap (cap)}
              <span
                class={cn(
                  'rounded-md border px-1.5 py-0.5 text-[11px]',
                  cap.startsWith('No') || cap.startsWith('Cannot')
                    ? 'border-border text-muted-foreground'
                    : cap.includes('approval')
                      ? 'border-status-deploying/40 text-status-deploying'
                      : 'border-border text-foreground'
                )}>{cap}</span
              >
            {/each}
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>
