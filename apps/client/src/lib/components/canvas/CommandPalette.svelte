<script lang="ts">
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import Sparkles from '@lucide/svelte/icons/sparkles';
  import ProviderBadge from '#lib/components/common/ProviderBadge.svelte';
  import * as Command from '#lib/components/ui/command/index.js';
  import { getCanvasState } from '#lib/canvas.svelte.js';
  import { catalogQuery } from '#lib/data/queries.js';
  import { CREATE_OPTIONS, type CreateOption } from '#lib/kinds.js';
  import { categoryLabel } from '#lib/format.js';
  import type { ProviderInfo, Service } from '#lib/data/types';

  let { services }: { services: Service[] } = $props();

  const canvas = getCanvasState();
  const catalog = catalogQuery();

  let option = $state<CreateOption | null>(null);
  let search = $state('');

  // Start over whenever the palette closes.
  $effect(() => {
    if (!canvas.paletteOpen) {
      option = null;
      search = '';
    }
  });

  const providers = $derived.by(() => {
    if (!option) return [];
    const kinds = option.kinds;
    return (catalog.data ?? [])
      .filter((p) => p.kinds.some((k) => kinds.includes(k)))
      .sort(
        (a, b) =>
          Number(b.status === 'alpha') - Number(a.status === 'alpha') ||
          a.name.localeCompare(b.name)
      );
  });

  function pickOption(o: CreateOption) {
    option = o;
    search = '';
  }

  function create(provider: ProviderInfo | null) {
    if (!option) return;
    canvas.addDraft({ nodeKind: option.nodeKind, label: option.label }, provider, services);
    canvas.paletteOpen = false;
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Backspace' && option && search === '') {
      e.preventDefault();
      option = null;
    }
  }
</script>

<Command.Dialog
  bind:open={canvas.paletteOpen}
  title={option ? 'Where?' : 'What would you like to create?'}
  description="Create a service on the canvas"
  class="sm:max-w-[520px]"
>
  <div {onkeydown} role="presentation" class="contents">
    <Command.Input
      bind:value={search}
      placeholder={option
        ? `Where should "${option.label}" run?`
        : 'What would you like to create?'}
    />
    <Command.List class="max-h-[380px] p-1">
      <Command.Empty>No results.</Command.Empty>

      {#if !option}
        <Command.Group heading="Create">
          {#each CREATE_OPTIONS as o (o.id)}
            {@const Icon = o.icon}
            <Command.Item value={o.label} onSelect={() => pickOption(o)}>
              <Icon class="text-muted-foreground" />
              <span class="flex-1">{o.label}</span>
              <span class="text-xs text-muted-foreground">{o.hint}</span>
            </Command.Item>
          {/each}
        </Command.Group>
      {:else}
        <Command.Group heading="Where?">
          <Command.Item value="back" onSelect={() => (option = null)} class="text-muted-foreground">
            <ArrowLeft /> Back to {option.label}
          </Command.Item>
          <Command.Item value="let yari.run choose" onSelect={() => create(null)}>
            <Sparkles class="text-primary" />
            <span class="flex-1">Let yari.run choose</span>
            <span class="text-xs text-muted-foreground">Best fit for your connections</span>
          </Command.Item>
          {#each providers as p (p.id)}
            {@const planned = p.status !== 'alpha'}
            <Command.Item
              value={p.name}
              keywords={[p.id, p.category]}
              disabled={planned}
              onSelect={() => create(p)}
            >
              <span class="flex-1">{p.name}</span>
              <span class="mr-2 text-xs text-muted-foreground">{categoryLabel(p.category)}</span>
              {#if planned}
                <span class="text-xs text-muted-foreground">Coming soon</span>
              {:else}
                <ProviderBadge provider={p.id} name="alpha" />
              {/if}
            </Command.Item>
          {:else}
            <p class="px-2 py-3 text-xs text-muted-foreground">
              No catalog provider supports this yet. yari.run can still choose for you.
            </p>
          {/each}
        </Command.Group>
      {/if}
    </Command.List>
  </div>
</Command.Dialog>
