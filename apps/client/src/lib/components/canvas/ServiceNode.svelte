<script lang="ts">
  import { Handle, Position, type NodeProps, type Node } from '@xyflow/svelte';
  import KindIcon from '#lib/components/common/KindIcon.svelte';
  import ProviderBadge from '#lib/components/common/ProviderBadge.svelte';
  import StatusDot from '#lib/components/common/StatusDot.svelte';
  import { kindLabel } from '#lib/kinds.js';
  import { cn } from '#lib/utils.js';
  import type { Service } from '#lib/data/types';

  type ServiceNodeType = Node<{ service: Service }, 'service'>;

  let { data, selected }: NodeProps<ServiceNodeType> = $props();
  const service = $derived(data.service);
  const draft = $derived(service.status === 'draft');
</script>

<Handle type="target" position={Position.Left} />
<div
  class={cn(
    'w-[244px] rounded-xl border bg-card p-3.5 shadow-sm transition-colors',
    draft ? 'border-dashed border-status-draft/60' : 'border-border',
    selected && 'border-primary ring-3 ring-primary/25'
  )}
>
  <div class="flex items-start gap-3">
    <span
      class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"
    >
      <KindIcon kind={service.kind} class="size-4" />
    </span>
    <div class="min-w-0 flex-1">
      <div class="flex items-center justify-between gap-2">
        <p class="truncate text-[13px] leading-5 font-medium">{service.name}</p>
        <StatusDot status={service.status} />
      </div>
      <p class="truncate text-xs leading-4 text-muted-foreground">
        {service.domain ?? kindLabel(service.kind)}
      </p>
    </div>
  </div>
  <div class="mt-3 flex items-center justify-between">
    {#if service.provider}
      <ProviderBadge provider={service.provider} />
    {:else}
      <span class="text-[10.5px] text-muted-foreground">Provider: yari.run chooses</span>
    {/if}
    <span class="text-[11px] text-muted-foreground">
      {draft ? 'Draft' : service.domain ? kindLabel(service.kind) : ''}
    </span>
  </div>
</div>
<Handle type="source" position={Position.Right} />
