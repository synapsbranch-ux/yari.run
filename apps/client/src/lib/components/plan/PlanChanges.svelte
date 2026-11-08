<script lang="ts">
  import KindIcon from '#lib/components/common/KindIcon.svelte';
  import Money from '#lib/components/common/Money.svelte';
  import ProviderBadge from '#lib/components/common/ProviderBadge.svelte';
  import { cn } from '#lib/utils.js';
  import type { PlanChange } from '#lib/data/types';

  let { changes }: { changes: PlanChange[] } = $props();

  const SIGN = { create: '+', update: '~', delete: '-', noop: '=' } as const;
  const SIGN_STYLE = {
    create: 'text-status-online',
    update: 'text-status-deploying',
    delete: 'text-status-failed',
    noop: 'text-muted-foreground'
  } as const;
  const RISK_DOT = {
    low: 'bg-status-online',
    medium: 'bg-status-deploying',
    high: 'bg-status-failed'
  } as const;
</script>

<ul class="divide-y divide-border overflow-hidden rounded-lg border border-border bg-card">
  {#each changes as c (c.service + c.action)}
    <li class="flex items-start gap-3 px-3.5 py-3">
      <span class={cn('mt-px w-3 shrink-0 text-center font-mono text-sm', SIGN_STYLE[c.action])}>
        {SIGN[c.action]}
      </span>
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <KindIcon kind={c.kind} class="size-3.5 text-muted-foreground" />
          <span class="text-[13px] font-medium">{c.service}</span>
          <ProviderBadge provider={c.provider} />
          <span class="text-xs text-muted-foreground capitalize">{c.action}</span>
        </div>
        <p class="mt-1 text-xs leading-relaxed text-muted-foreground">{c.description}</p>
      </div>
      <div class="flex shrink-0 flex-col items-end gap-1">
        <Money cents={c.costDeltaCents} class="text-xs" />
        <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
          <span class={cn('size-1.5 rounded-full', RISK_DOT[c.risk])}></span>{c.risk}
        </span>
      </div>
    </li>
  {/each}
</ul>
