<script lang="ts">
  import { cn } from '#lib/utils.js';
  import type { Risk } from '#lib/data/types';

  let { risk, class: className }: { risk: Risk; class?: string } = $props();

  const STYLE: Record<Risk, string> = {
    low: 'text-status-online',
    medium: 'text-status-deploying',
    high: 'text-status-failed'
  };
  const DOT: Record<Risk, string> = {
    low: 'bg-status-online',
    medium: 'bg-status-deploying',
    high: 'bg-status-failed'
  };
</script>

<span class={cn('inline-flex items-center gap-1.5 text-xs font-medium', STYLE[risk], className)}>
  <span class={cn('size-1.5 rounded-full', DOT[risk])}></span>
  {risk[0].toUpperCase() + risk.slice(1)} risk
</span>
