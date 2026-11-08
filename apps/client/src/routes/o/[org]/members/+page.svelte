<script lang="ts">
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import { Badge } from '#lib/components/ui/badge/index.js';
  import { Button } from '#lib/components/ui/button/index.js';
  import { grantsQuery, membersQuery } from '#lib/data/queries.js';
  import { getOrgContext } from '#lib/org-context.js';

  const ctx = getOrgContext();
  const members = membersQuery(() => ctx.org?.id);
  const grants = grantsQuery(() => ctx.org?.id);

  const KIND: Record<string, string> = {
    human: 'Human',
    agent: 'Agent',
    ci: 'CI',
    api_key: 'API key'
  };
</script>

<svelte:head><title>Members & Teams · yari.run</title></svelte:head>

<PageHeader
  title="Members & Teams"
  description="Principals are humans, API keys, CI runners and agents. Each holds Grants of a Role on a scope."
>
  {#snippet actions()}
    <Button size="sm" variant="outline" disabled>Invite member</Button>
  {/snippet}
</PageHeader>

<div class="overflow-hidden rounded-xl border border-border bg-card">
  <table class="w-full text-left text-[13px]">
    <thead class="border-b border-border text-xs text-muted-foreground">
      <tr>
        <th class="px-4 py-2.5 font-medium">Principal</th>
        <th class="px-4 py-2.5 font-medium">Kind</th>
        <th class="hidden px-4 py-2.5 font-medium sm:table-cell">Team</th>
        <th class="px-4 py-2.5 font-medium">Grants</th>
      </tr>
    </thead>
    <tbody>
      {#each members.data ?? [] as m (m.id)}
        <tr class="border-b border-border last:border-0">
          <td class="px-4 py-3">
            <p class="font-medium">{m.name}</p>
            {#if m.email}<p class="text-xs text-muted-foreground">{m.email}</p>{/if}
          </td>
          <td class="px-4 py-3"
            ><Badge variant="outline" class="rounded-md">{KIND[m.kind] ?? m.kind}</Badge></td
          >
          <td class="hidden px-4 py-3 text-muted-foreground sm:table-cell">{m.team ?? '-'}</td>
          <td class="px-4 py-3">
            <div class="flex flex-wrap gap-1">
              {#each (grants.data ?? []).filter((g) => g.principalId === m.id) as g (g.id)}
                <span class="text-xs text-muted-foreground">
                  <span class="text-foreground">{g.role}</span> on {g.scope.name}
                </span>
              {/each}
            </div>
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan="4" class="px-4 py-8 text-center text-xs text-muted-foreground">
            {members.isPending ? 'Loading...' : 'Member listing is not available from the API yet.'}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
