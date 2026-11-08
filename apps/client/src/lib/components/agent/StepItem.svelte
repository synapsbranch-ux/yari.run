<script lang="ts">
  import Bot from '@lucide/svelte/icons/bot';
  import Eye from '@lucide/svelte/icons/eye';
  import ShieldAlert from '@lucide/svelte/icons/shield-alert';
  import Terminal from '@lucide/svelte/icons/terminal';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import { clockTime } from '#lib/format.js';
  import { cn } from '#lib/utils.js';
  import type { AgentStep } from '#lib/data/types';

  let { step, onreviewplan }: { step: AgentStep; onreviewplan?: (planId: string) => void } =
    $props();

  const LOCATION = { sandbox: 'Sandbox', yari: 'yari', none: '' } as const;
</script>

{#if step.kind === 'status'}
  <div class="flex items-center gap-3 py-1 text-[11.5px] text-muted-foreground">
    <span class="h-px flex-1 bg-border"></span>
    <span class="max-w-[85%] text-center">{step.summary}</span>
    <span class="h-px flex-1 bg-border"></span>
  </div>
{:else if step.kind === 'message'}
  <div class={cn('flex', step.author === 'user' && 'justify-end')}>
    <div
      class={cn(
        'max-w-[92%] rounded-xl px-3.5 py-2.5 text-[13px] leading-relaxed',
        step.author === 'user'
          ? 'rounded-br-sm bg-primary/15 text-foreground'
          : 'rounded-bl-sm bg-muted'
      )}
    >
      {#if step.author !== 'user'}
        <p class="mb-1 flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
          <Bot class="size-3" /> yari agent
        </p>
      {/if}
      {step.summary}
    </div>
  </div>
{:else if step.kind === 'action'}
  <div class="rounded-lg border border-border bg-card px-3 py-2.5">
    <div class="flex items-center gap-2">
      <Terminal class="size-3.5 shrink-0 text-muted-foreground" />
      <code class="min-w-0 flex-1 truncate font-mono text-xs">{step.tool}</code>
      {#if LOCATION[step.location]}
        <span
          class={cn(
            'rounded-full border px-1.5 py-px text-[10.5px] leading-4',
            step.location === 'sandbox'
              ? 'border-border text-muted-foreground'
              : 'border-primary/40 text-primary'
          )}
          title={step.location === 'sandbox'
            ? 'Ran inside the isolated Sandbox'
            : 'Ran through the yari.run API, so it is permission checked and audited'}
        >
          {LOCATION[step.location]}
        </span>
      {/if}
      <time class="shrink-0 text-[10.5px] text-muted-foreground/70" datetime={step.at}
        >{clockTime(step.at)}</time
      >
    </div>
    <p class="mt-1.5 text-xs leading-relaxed text-muted-foreground">{step.summary}</p>
    {#if step.planId && onreviewplan}
      <button
        type="button"
        class="mt-2 text-xs font-medium text-primary hover:underline"
        onclick={() => onreviewplan?.(step.planId!)}
      >
        Review plan {step.planId}
      </button>
    {/if}
  </div>
{:else if step.kind === 'observation'}
  <div class="ml-2 flex gap-2 border-l-2 border-border pl-3">
    <Eye class="mt-0.5 size-3.5 shrink-0 text-muted-foreground/70" />
    <p class="text-xs leading-relaxed text-muted-foreground">{step.summary}</p>
  </div>
{:else if step.kind === 'guardrail'}
  <div
    class="flex gap-2.5 rounded-lg border border-status-deploying/40 bg-status-deploying/5 px-3 py-2.5"
  >
    <ShieldAlert class="mt-0.5 size-4 shrink-0 text-status-deploying" />
    <div class="min-w-0">
      <p class="text-[12px] font-medium">
        Guardrail refused <code class="font-mono">{step.tool}</code>
      </p>
      <p class="mt-0.5 text-xs leading-relaxed text-muted-foreground">{step.summary}</p>
    </div>
  </div>
{:else}
  <div
    class="flex gap-2.5 rounded-lg border border-status-failed/40 bg-status-failed/5 px-3 py-2.5"
  >
    <TriangleAlert class="mt-0.5 size-4 shrink-0 text-status-failed" />
    <p class="text-xs leading-relaxed">{step.summary}</p>
  </div>
{/if}
