<script lang="ts">
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import { Input } from '#lib/components/ui/input/index.js';
  import { getOrgContext } from '#lib/org-context.js';

  const ctx = getOrgContext();
</script>

<svelte:head><title>Settings · yari.run</title></svelte:head>

<PageHeader title="Settings" description="Organization-wide settings." />

<div class="max-w-xl space-y-5 rounded-xl border border-border bg-card p-5">
  <div class="space-y-1.5">
    <label for="org-name" class="text-xs font-medium">Organization name</label>
    <Input id="org-name" value={ctx.org?.name ?? ''} disabled class="h-8 text-[13px]" />
  </div>
  <div class="space-y-1.5">
    <label for="org-slug" class="text-xs font-medium">Slug</label>
    <Input id="org-slug" value={ctx.org?.slug ?? ''} disabled class="h-8 font-mono text-[13px]" />
    <p class="text-xs text-muted-foreground">Used in URLs, for example /o/{ctx.slug}.</p>
  </div>
  <Button size="sm" variant="outline" disabled>Save changes</Button>
</div>
