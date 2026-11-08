<script lang="ts">
  import { page } from '$app/state';
  import Bot from '@lucide/svelte/icons/bot';
  import Command from '@lucide/svelte/icons/command';
  import Moon from '@lucide/svelte/icons/moon';
  import Plus from '@lucide/svelte/icons/plus';
  import Rocket from '@lucide/svelte/icons/rocket';
  import Sun from '@lucide/svelte/icons/sun';
  import { CanvasState, setCanvasState } from '#lib/canvas.svelte.js';
  import AgentPanel from '#lib/components/agent/AgentPanel.svelte';
  import Board from '#lib/components/canvas/Board.svelte';
  import Breadcrumb from '#lib/components/canvas/Breadcrumb.svelte';
  import CommandPalette from '#lib/components/canvas/CommandPalette.svelte';
  import PlanDrawer from '#lib/components/canvas/PlanDrawer.svelte';
  import ServicePanel from '#lib/components/canvas/ServicePanel.svelte';
  import Wordmark from '#lib/components/common/Wordmark.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import { edgesQuery, hierarchyQuery, servicesQuery } from '#lib/data/queries.js';
  import { theme } from '#lib/theme.svelte.js';
  import { cn } from '#lib/utils.js';

  const slugs = $derived({
    org: page.params.org ?? '',
    ws: page.params.ws ?? '',
    project: page.params.project ?? '',
    env: page.params.env ?? ''
  });

  const hierarchy = hierarchyQuery(() => slugs);
  const ctx = $derived(hierarchy.data);
  const services = servicesQuery(() => ctx?.environment.id);
  const edges = edgesQuery(() => ctx?.environment.id);

  const canvas = new CanvasState();
  setCanvasState(canvas);

  $effect(() => {
    if (ctx) {
      canvas.environmentId = ctx.environment.id;
      canvas.environmentProtected = ctx.environment.protected;
    }
  });

  // Switching environment starts a fresh canvas.
  $effect(() => {
    void slugs.env;
    void slugs.project;
    canvas.drafts = [];
    canvas.closeDock();
  });

  const all = $derived([...(services.data ?? []), ...canvas.drafts]);
  const selected = $derived(all.find((s) => s.id === canvas.selectedId));

  const isMac = typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.platform);
  const shortcut = isMac ? '⌘K' : 'Ctrl K';

  function onkeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      canvas.paletteOpen = !canvas.paletteOpen;
    }
  }

  const changeCount = $derived(canvas.drafts.length);
</script>

<svelte:window {onkeydown} />
<svelte:head
  ><title>{ctx ? `${ctx.project.name} · ${ctx.environment.name}` : 'Canvas'} · yari.run</title
  ></svelte:head
>

<div class="flex h-dvh flex-col">
  <header class="flex h-12 shrink-0 items-center gap-2 border-b border-border bg-surface px-3">
    <a
      href="/o/{slugs.org}"
      class="mr-1 inline-flex shrink-0 items-center rounded-md p-1"
      aria-label="Organization home"
    >
      <Wordmark class="text-[14px] max-sm:[&>span]:hidden" />
    </a>
    <span class="h-4 w-px bg-border" aria-hidden="true"></span>
    {#if ctx}
      <Breadcrumb {ctx} />
    {:else if hierarchy.isError}
      <span class="text-xs text-destructive">{hierarchy.error.message}</span>
    {:else}
      <span class="h-4 w-56 animate-pulse rounded bg-muted"></span>
    {/if}

    <div class="ml-auto flex items-center gap-1.5">
      <Button
        variant="ghost"
        size="icon-sm"
        class="max-sm:hidden"
        onclick={() => theme.toggle()}
        aria-label="Toggle theme"
      >
        {#if theme.current === 'dark'}<Sun class="size-4" />{:else}<Moon class="size-4" />{/if}
      </Button>
      <Button
        variant="outline"
        size="sm"
        onclick={() => canvas.openAgent()}
        aria-pressed={canvas.dock === 'agent'}
        class={cn(canvas.dock === 'agent' && 'bg-accent')}
        disabled={!ctx}
      >
        <Bot class="size-3.5" /> Agent
      </Button>
      <Button size="sm" onclick={() => canvas.openDeploy()} disabled={!ctx}>
        <Rocket class="size-3.5" /> Deploy
        {#if changeCount > 0}
          <span
            class="ml-0.5 rounded-full bg-primary-foreground/20 px-1.5 text-[11px] leading-[16px]"
          >
            {changeCount}
          </span>
        {/if}
      </Button>
    </div>
  </header>

  <div class="relative flex min-h-0 flex-1">
    <div class="relative min-w-0 flex-1">
      {#if ctx && services.data && edges.data}
        {#key ctx.environment.id}
          <Board services={services.data} edges={edges.data} />
        {/key}
      {:else if hierarchy.isError || services.isError}
        <div class="flex h-full items-center justify-center">
          <p class="text-sm text-muted-foreground">
            This environment could not be loaded.
            <a href="/o/{slugs.org}" class="text-primary hover:underline">Back to projects</a>
          </p>
        </div>
      {:else}
        <div class="flex h-full items-center justify-center">
          <p class="text-xs text-muted-foreground">Loading canvas...</p>
        </div>
      {/if}

      <button
        type="button"
        onclick={() => (canvas.paletteOpen = true)}
        class="absolute top-3 left-3 z-10 flex items-center gap-2 rounded-lg border border-border bg-card py-1.5 pr-2 pl-2.5 text-xs text-muted-foreground shadow-sm hover:bg-accent hover:text-foreground"
      >
        <Plus class="size-3.5" /> Create
        <kbd
          class="flex items-center gap-0.5 rounded bg-muted px-1.5 py-0.5 font-sans text-[10.5px] text-muted-foreground"
        >
          {#if isMac}<Command class="size-2.5" />K{:else}{shortcut}{/if}
        </kbd>
      </button>
    </div>

    {#if canvas.dock && ctx}
      <aside
        class="z-20 w-[460px] max-w-full shrink-0 border-l border-border bg-surface max-lg:absolute max-lg:inset-y-0 max-lg:right-0 max-lg:shadow-2xl"
        aria-label={canvas.dock === 'agent' ? 'Agent' : 'Service details'}
      >
        {#if canvas.dock === 'agent'}
          <AgentPanel {ctx} />
        {:else if selected}
          <ServicePanel
            service={selected}
            services={services.data ?? []}
            edges={edges.data ?? []}
            {ctx}
            onclose={() => canvas.closeDock()}
            ondiscard={() => canvas.removeDraft(selected.id)}
          />
        {/if}
      </aside>
    {/if}
  </div>

  {#if ctx}
    <CommandPalette services={services.data ?? []} />
    <PlanDrawer {ctx} />
  {/if}
</div>
