<script lang="ts">
  import type { Snippet } from 'svelte';
  import { page } from '$app/state';
  import OrgShell from '#lib/components/shell/OrgShell.svelte';
  import { organizationsQuery } from '#lib/data/queries.js';
  import { setOrgContext } from '#lib/org-context.js';

  let { children }: { children: Snippet } = $props();

  const slug = $derived(page.params.org ?? '');
  const orgs = organizationsQuery();
  const org = $derived(orgs.data?.find((o) => o.slug === slug));

  setOrgContext({
    get org() {
      return org;
    },
    get slug() {
      return slug;
    }
  });
</script>

<OrgShell orgSlug={slug}>
  {#if orgs.isError}
    <p class="text-sm text-destructive">Could not load organizations: {orgs.error.message}</p>
  {:else if orgs.data && !org}
    <p class="text-sm text-muted-foreground">Organization "{slug}" was not found.</p>
  {:else}
    {@render children()}
  {/if}
</OrgShell>
