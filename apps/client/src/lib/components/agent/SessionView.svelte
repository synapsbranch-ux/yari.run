<script lang="ts">
  import { useQueryClient } from '@tanstack/svelte-query';
  import Send from '@lucide/svelte/icons/send';
  import Square from '@lucide/svelte/icons/square';
  import { Button } from '#lib/components/ui/button/index.js';
  import { Input } from '#lib/components/ui/input/index.js';
  import { data } from '#lib/data/index.js';
  import { keys, sessionQuery, stopSessionMutation } from '#lib/data/queries.js';
  import { usdMicros } from '#lib/format.js';
  import StepItem from './StepItem.svelte';
  import type { AgentStep, SessionStatus } from '#lib/data/types';

  let {
    sessionId,
    readonly = false,
    onreviewplan
  }: {
    sessionId: string;
    /** Read-only plus Stop, as on the phone. */
    readonly?: boolean;
    onreviewplan?: (planId: string) => void;
  } = $props();

  const client = useQueryClient();

  let steps = $state<AgentStep[]>([]);
  let liveStatus = $state<SessionStatus | null>(null);
  let streamError = $state<string | null>(null);
  let message = $state('');
  let sendError = $state<string | null>(null);
  let scroller = $state<HTMLElement | null>(null);

  const ACTIVE: SessionStatus[] = ['queued', 'running', 'waiting_approval'];
  const active = $derived(liveStatus !== null && ACTIVE.includes(liveStatus));

  const session = sessionQuery(
    () => sessionId,
    () => liveStatus === 'running' || liveStatus === 'queued'
  );
  const stop = stopSessionMutation();

  // Follow the step stream of the selected session.
  $effect(() => {
    const id = sessionId;
    steps = [];
    liveStatus = null;
    streamError = null;
    const controller = new AbortController();
    void data.watchAgentSession(
      id,
      {
        onStep: (step) => {
          if (steps.some((s) => s.seq === step.seq)) return;
          steps = [...steps, step].sort((a, b) => a.seq - b.seq);
        },
        onStatus: (status) => {
          liveStatus = status;
          client.invalidateQueries({ queryKey: keys.session(id) });
        },
        onError: (err) => {
          streamError = err instanceof Error ? err.message : 'The step stream was interrupted.';
        }
      },
      controller.signal
    );
    return () => controller.abort();
  });

  // Keep the newest step in view.
  $effect(() => {
    void steps.length;
    if (scroller) scroller.scrollTo({ top: scroller.scrollHeight, behavior: 'smooth' });
  });

  const s = $derived(session.data);
  const status = $derived(liveStatus ?? s?.status ?? 'queued');

  async function send(e: SubmitEvent) {
    e.preventDefault();
    const text = message.trim();
    if (!text) return;
    message = '';
    sendError = null;
    try {
      await data.sendAgentMessage(sessionId, text);
    } catch (err) {
      sendError = err instanceof Error ? err.message : 'Could not send the message.';
    }
  }

  const NETWORK = { none: 'no internet', allowlist: 'allowlisted network' } as const;
</script>

<div class="flex h-full min-h-0 flex-col">
  {#if s}
    <div class="space-y-1 border-b border-border bg-surface px-4 py-2.5 text-xs">
      <p class="flex flex-wrap items-center gap-x-1.5 text-muted-foreground">
        <span class="font-mono text-foreground">{s.model}</span>
        <span>·</span>
        <span class="tabular-nums"
          >{usdMicros(s.spendUsdMicros)} of {usdMicros(s.budgetUsdMicros)}</span
        >
        <span>·</span>
        <span class="capitalize">{status.replace('_', ' ')}</span>
      </p>
      <p class="text-muted-foreground">
        Sandbox: {s.sandbox.status} · {s.sandbox.cpu} CPU · {s.sandbox.memoryGb} GB · {NETWORK[
          s.sandbox.network
        ]}
      </p>
    </div>
  {/if}

  <div bind:this={scroller} class="min-h-0 flex-1 space-y-3 overflow-y-auto p-4" aria-live="polite">
    {#if s}
      <p class="text-[11.5px] text-muted-foreground">Goal</p>
      <p class="-mt-2 text-[13px] leading-relaxed font-medium">{s.goal}</p>
    {/if}
    {#each steps as step (step.seq)}
      <StepItem {step} {onreviewplan} />
    {/each}
    {#if steps.length === 0 && !streamError}
      <p class="text-xs text-muted-foreground">Waiting for the first step...</p>
    {/if}
    {#if streamError}
      <p class="text-xs text-destructive">{streamError}</p>
    {/if}
    {#if status === 'running'}
      <div class="flex items-center gap-2 text-xs text-muted-foreground" aria-hidden="true">
        <span class="size-1.5 animate-pulse rounded-full bg-status-deploying"></span> Working
      </div>
    {/if}
  </div>

  <div class="space-y-2 border-t border-border p-3">
    {#if sendError}<p class="text-xs text-destructive">{sendError}</p>{/if}
    {#if !readonly}
      <form class="flex items-center gap-2" onsubmit={send}>
        <Input
          bind:value={message}
          placeholder={active ? 'Send a follow-up message' : 'This session has ended'}
          disabled={!active}
          class="h-8 text-[13px]"
          aria-label="Follow-up message"
        />
        <Button
          type="submit"
          size="icon-sm"
          disabled={!active || !message.trim()}
          aria-label="Send"
        >
          <Send class="size-3.5" />
        </Button>
      </form>
    {/if}
    <div class="flex items-center justify-between">
      <p class="text-[11px] text-muted-foreground">
        The agent acts only through plans. {readonly ? '' : 'It cannot read secrets.'}
      </p>
      <Button
        variant="outline"
        size="xs"
        disabled={!active || stop.isPending}
        onclick={() => stop.mutate(sessionId)}
      >
        <Square class="size-3" /> Stop
      </Button>
    </div>
  </div>
</div>
