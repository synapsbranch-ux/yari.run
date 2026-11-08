<script lang="ts">
  import X from '@lucide/svelte/icons/x';
  import KindIcon from '#lib/components/common/KindIcon.svelte';
  import ProviderBadge from '#lib/components/common/ProviderBadge.svelte';
  import StatusDot from '#lib/components/common/StatusDot.svelte';
  import * as Tabs from '#lib/components/ui/tabs/index.js';
  import { Button } from '#lib/components/ui/button/index.js';
  import { kindLabel } from '#lib/kinds.js';
  import type { Edge, HierarchyContext, Service } from '#lib/data/types';
  import AccessTab from './tabs/AccessTab.svelte';
  import DeploymentsTab from './tabs/DeploymentsTab.svelte';
  import LogsTab from './tabs/LogsTab.svelte';
  import MetricsTab from './tabs/MetricsTab.svelte';
  import SettingsTab from './tabs/SettingsTab.svelte';
  import VariablesTab from './tabs/VariablesTab.svelte';

  let {
    service,
    services,
    edges,
    ctx,
    onclose,
    ondiscard
  }: {
    service: Service;
    services: Service[];
    edges: Edge[];
    ctx: HierarchyContext;
    onclose: () => void;
    ondiscard?: () => void;
  } = $props();

  let tab = $state('deployments');
  const draft = $derived(service.status === 'draft');
</script>

<div class="flex h-full min-h-0 flex-col">
  <header class="flex items-start gap-3 border-b border-border px-4 py-3.5">
    <span
      class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"
    >
      <KindIcon kind={service.kind} class="size-[18px]" />
    </span>
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <h2 class="truncate text-sm font-semibold">{service.name}</h2>
        <StatusDot status={service.status} label />
      </div>
      <div class="mt-1 flex items-center gap-2">
        {#if service.provider}<ProviderBadge provider={service.provider} />{/if}
        <span class="truncate text-xs text-muted-foreground"
          >{service.domain ?? kindLabel(service.kind)}</span
        >
      </div>
    </div>
    <Button variant="ghost" size="icon-sm" onclick={onclose} aria-label="Close panel">
      <X class="size-4" />
    </Button>
  </header>

  {#if draft}
    <div class="flex-1 space-y-4 overflow-y-auto p-4">
      <div class="rounded-lg border border-dashed border-status-draft/40 p-4">
        <p class="text-[13px] font-medium">Draft service</p>
        <p class="mt-1 text-xs leading-relaxed text-muted-foreground">
          This service exists only on the canvas. Deploy builds a Plan so you can review the
          changes, the cost and the risk before anything is created at the provider.
        </p>
      </div>
      <Button variant="outline" size="sm" onclick={ondiscard}>Discard draft</Button>
    </div>
  {:else}
    <Tabs.Root bind:value={tab} class="flex min-h-0 flex-1 flex-col gap-0">
      <div class="overflow-x-auto border-b border-border px-2">
        <Tabs.List variant="line" class="h-10 w-full justify-between gap-0 bg-transparent">
          {#each [['deployments', 'Deployments'], ['variables', 'Variables'], ['logs', 'Logs'], ['metrics', 'Metrics'], ['access', 'Access'], ['settings', 'Settings']] as [value, label] (value)}
            <Tabs.Trigger {value} class="px-2 text-[12.5px]">{label}</Tabs.Trigger>
          {/each}
        </Tabs.List>
      </div>
      <div class="min-h-0 flex-1 overflow-y-auto">
        <Tabs.Content value="deployments" class="p-4"><DeploymentsTab {service} /></Tabs.Content>
        <Tabs.Content value="variables" class="p-4">
          <VariablesTab {service} {services} {edges} />
        </Tabs.Content>
        <Tabs.Content value="logs" class="p-4"><LogsTab {service} /></Tabs.Content>
        <Tabs.Content value="metrics" class="p-4"><MetricsTab {service} /></Tabs.Content>
        <Tabs.Content value="access" class="p-4"><AccessTab {service} {ctx} /></Tabs.Content>
        <Tabs.Content value="settings" class="p-4"><SettingsTab {service} {ctx} /></Tabs.Content>
      </div>
    </Tabs.Root>
  {/if}
</div>
