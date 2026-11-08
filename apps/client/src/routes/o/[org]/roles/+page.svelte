<script lang="ts">
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import { rolesQuery } from '#lib/data/queries.js';

  const roles = rolesQuery();
</script>

<svelte:head><title>Roles · yari.run</title></svelte:head>

<PageHeader
  title="Roles"
  description="Built-in roles are cumulative. Grants carry down Organization, Workspace, Project and Environment, and the highest role on the chain wins."
/>

<div class="space-y-3">
  {#each roles.data ?? [] as r (r.role)}
    <article class="rounded-xl border border-border bg-card p-4">
      <h2 class="text-sm font-medium capitalize">{r.role}</h2>
      <p class="mt-1 text-xs text-muted-foreground">{r.summary}</p>
      <div class="mt-3 flex flex-wrap gap-1.5">
        {#each r.permissions as perm (perm)}
          <code class="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground"
            >{perm}</code
          >
        {/each}
      </div>
    </article>
  {/each}
</div>

<div class="mt-6 space-y-1.5 rounded-xl border border-dashed border-border p-4 text-xs">
  <p class="font-medium text-foreground">Rules applied on top of roles</p>
  <ul class="list-disc space-y-1 pl-4 text-muted-foreground">
    <li>On protected environments, developers need approval before a plan applies.</li>
    <li>On protected environments, only admins and owners can read secrets.</li>
    <li>
      Agents never get <code>secret.read</code>, cannot approve plans and cannot start agents.
    </li>
  </ul>
</div>
