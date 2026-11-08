<script lang="ts">
  import { cn } from '#lib/utils.js';
  import type { ServiceStatus } from '#lib/data/types';

  let {
    status,
    label = false,
    class: className
  }: { status: ServiceStatus; label?: boolean; class?: string } = $props();

  const LABELS: Record<ServiceStatus, string> = {
    online: 'Online',
    sleeping: 'Sleeping',
    deploying: 'Deploying',
    failed: 'Failed',
    draft: 'Draft'
  };
  const COLORS: Record<ServiceStatus, string> = {
    online: 'bg-status-online',
    sleeping: 'bg-status-sleeping',
    deploying: 'bg-status-deploying',
    failed: 'bg-status-failed',
    draft: 'bg-status-draft'
  };
</script>

<span class={cn('inline-flex items-center gap-1.5', className)}>
  <span
    class={cn(
      'inline-block size-1.5 shrink-0 rounded-full',
      COLORS[status],
      status === 'deploying' && 'animate-pulse'
    )}
    role="img"
    aria-label={LABELS[status]}
  ></span>
  {#if label}<span class="text-xs text-muted-foreground">{LABELS[status]}</span>{/if}
</span>
