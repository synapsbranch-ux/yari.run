<script lang="ts">
  import { Button } from '#lib/components/ui/button/index.js';
  import { Input } from '#lib/components/ui/input/index.js';
  import ProviderBadge from '#lib/components/common/ProviderBadge.svelte';
  import { kindLabel } from '#lib/kinds.js';
  import type { HierarchyContext, Service } from '#lib/data/types';

  let { service, ctx }: { service: Service; ctx: HierarchyContext } = $props();
</script>

<div class="space-y-5">
  <div class="space-y-1.5">
    <label for="svc-name" class="text-xs font-medium">Name</label>
    <Input id="svc-name" value={service.name} disabled class="h-8 text-[13px]" />
  </div>
  <dl class="grid grid-cols-[110px_1fr] gap-y-2.5 text-[13px]">
    <dt class="text-muted-foreground">Kind</dt>
    <dd>{kindLabel(service.kind)}</dd>
    <dt class="text-muted-foreground">Provider</dt>
    <dd><ProviderBadge provider={service.provider} /></dd>
    {#if service.region}
      <dt class="text-muted-foreground">Region</dt>
      <dd class="font-mono text-xs">{service.region}</dd>
    {/if}
    {#if service.domain}
      <dt class="text-muted-foreground">Domain</dt>
      <dd class="font-mono text-xs">{service.domain}</dd>
    {/if}
    <dt class="text-muted-foreground">Environment</dt>
    <dd>{ctx.environment.name}</dd>
  </dl>

  <div class="rounded-lg border border-border p-4">
    <p class="text-[13px] font-medium">Delete service</p>
    <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
      Deleting adds a change to a Plan. Nothing is removed until the Plan is
      {ctx.environment.protected ? 'approved by an admin and' : ''} applied.
    </p>
    <Button variant="destructive" size="sm" class="mt-3" disabled>Add delete to Plan</Button>
  </div>
</div>
