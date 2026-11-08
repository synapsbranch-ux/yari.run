<script lang="ts">
  import { useQueryClient } from '@tanstack/svelte-query';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import Plus from '@lucide/svelte/icons/plus';
  import X from '@lucide/svelte/icons/x';
  import { getCanvasState } from '#lib/canvas.svelte.js';
  import { Button } from '#lib/components/ui/button/index.js';
  import * as DropdownMenu from '#lib/components/ui/dropdown-menu/index.js';
  import * as Select from '#lib/components/ui/select/index.js';
  import { Textarea } from '#lib/components/ui/textarea/index.js';
  import { data } from '#lib/data/index.js';
  import { keys, modelsQuery, sessionsQuery } from '#lib/data/queries.js';
  import { relativeTime } from '#lib/format.js';
  import SessionView from './SessionView.svelte';
  import type { HierarchyContext } from '#lib/data/types';

  let { ctx }: { ctx: HierarchyContext } = $props();

  const canvas = getCanvasState();
  const client = useQueryClient();
  const sessions = sessionsQuery(() => ctx.environment.id);
  const models = modelsQuery();

  let chosenId = $state<string | null>(null);
  let composing = $state(false);
  let goal = $state('');
  let model = $state('');
  let startError = $state<string | null>(null);
  let starting = $state(false);

  const list = $derived(sessions.data ?? []);
  const current = $derived(
    composing ? undefined : (list.find((s) => s.id === chosenId) ?? list[0])
  );
  const showForm = $derived(composing || (!sessions.isPending && list.length === 0));

  const selectedModel = $derived(model || models.data?.[0]?.alias || '');
  const modelLabel = $derived(
    models.data?.find((m) => m.alias === selectedModel)?.displayName ?? selectedModel
  );

  async function start(e: SubmitEvent) {
    e.preventDefault();
    if (!goal.trim() || !selectedModel) return;
    starting = true;
    startError = null;
    try {
      const created = await data.startAgentSession({
        environmentId: ctx.environment.id,
        goal: goal.trim(),
        model: selectedModel
      });
      await client.invalidateQueries({ queryKey: keys.sessions(ctx.environment.id) });
      chosenId = created.id;
      composing = false;
      goal = '';
    } catch (err) {
      startError = err instanceof Error ? err.message : 'Could not start the session.';
    } finally {
      starting = false;
    }
  }
</script>

<div class="flex h-full min-h-0 flex-col">
  <header class="flex items-center gap-2 border-b border-border px-4 py-2.5">
    <h2 class="text-sm font-semibold">Agent</h2>
    {#if list.length > 0}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger
          class="flex min-w-0 items-center gap-1 rounded-md px-1.5 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
        >
          <span class="max-w-[170px] truncate">{current?.goal ?? 'New session'}</span>
          <ChevronDown class="size-3 shrink-0" />
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="start" class="w-72">
          <DropdownMenu.Label class="text-xs text-muted-foreground">Sessions</DropdownMenu.Label>
          {#each list as s (s.id)}
            <DropdownMenu.Item
              onSelect={() => {
                chosenId = s.id;
                composing = false;
              }}
            >
              <div class="min-w-0">
                <p class="truncate text-[13px]">{s.goal}</p>
                <p class="text-[11px] text-muted-foreground">
                  {s.status.replace('_', ' ')} · {relativeTime(s.startedAt)}
                </p>
              </div>
            </DropdownMenu.Item>
          {/each}
        </DropdownMenu.Content>
      </DropdownMenu.Root>
    {/if}
    <div class="ml-auto flex items-center gap-1">
      {#if !showForm}
        <Button variant="ghost" size="xs" onclick={() => (composing = true)}>
          <Plus class="size-3" /> New
        </Button>
      {/if}
      <Button
        variant="ghost"
        size="icon-sm"
        onclick={() => canvas.closeDock()}
        aria-label="Close panel"
      >
        <X class="size-4" />
      </Button>
    </div>
  </header>

  {#if showForm}
    <form class="flex-1 space-y-4 overflow-y-auto p-4" onsubmit={start}>
      <div>
        <label for="agent-goal" class="text-[13px] font-medium">What should the agent do?</label>
        <p class="mt-0.5 mb-2 text-xs text-muted-foreground">
          It works in its own Sandbox and can only change things by proposing a plan.
        </p>
        <Textarea
          id="agent-goal"
          bind:value={goal}
          rows={4}
          placeholder="For example: find out why the api restarts and propose a fix"
          class="text-[13px]"
        />
      </div>

      <div>
        <p class="mb-1.5 text-[13px] font-medium">Model</p>
        <Select.Root type="single" bind:value={model}>
          <Select.Trigger class="h-8 w-full text-[13px]" aria-label="Model">
            {modelLabel || 'Choose a model'}
          </Select.Trigger>
          <Select.Content>
            {#each models.data ?? [] as m (m.alias)}
              <Select.Item value={m.alias} label={m.displayName}>
                {m.displayName}
                <span class="ml-2 text-xs text-muted-foreground">{m.provider}</span>
              </Select.Item>
            {/each}
          </Select.Content>
        </Select.Root>
        <p class="mt-1.5 text-xs text-muted-foreground">
          Budget per session: $2.00. Network: none.
        </p>
      </div>

      {#if startError}<p class="text-xs text-destructive">{startError}</p>{/if}
      <div class="flex justify-end gap-2">
        {#if list.length > 0}
          <Button type="button" variant="ghost" size="sm" onclick={() => (composing = false)}
            >Cancel</Button
          >
        {/if}
        <Button type="submit" size="sm" disabled={starting || !goal.trim()}>Start session</Button>
      </div>
    </form>
  {:else if current}
    <div class="min-h-0 flex-1">
      <SessionView sessionId={current.id} onreviewplan={(id) => canvas.reviewPlan(id)} />
    </div>
  {:else}
    <p class="p-4 text-xs text-muted-foreground">Loading sessions...</p>
  {/if}
</div>
