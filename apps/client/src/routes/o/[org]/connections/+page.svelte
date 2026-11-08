<script lang="ts">
  import ExternalLink from '@lucide/svelte/icons/external-link';
  import Search from '@lucide/svelte/icons/search';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import { Badge } from '#lib/components/ui/badge/index.js';
  import { Button } from '#lib/components/ui/button/index.js';
  import { Input } from '#lib/components/ui/input/index.js';
  import { Skeleton } from '#lib/components/ui/skeleton/index.js';
  import { catalogQuery } from '#lib/data/queries.js';
  import { authLabel, categoryLabel } from '#lib/format.js';
  import { cn } from '#lib/utils.js';

  const catalog = catalogQuery();

  let search = $state('');
  let category = $state<string>('all');

  const categories = $derived([...new Set((catalog.data ?? []).map((p) => p.category))]);

  const filtered = $derived.by(() => {
    const q = search.trim().toLowerCase();
    return (catalog.data ?? []).filter(
      (p) =>
        (category === 'all' || p.category === category) &&
        (!q ||
          p.name.toLowerCase().includes(q) ||
          p.id.toLowerCase().includes(q) ||
          p.kinds.some((k) => k.includes(q)))
    );
  });

  const groups = $derived(
    categories
      .map((c) => ({ id: c, items: filtered.filter((p) => p.category === c) }))
      .filter((g) => g.items.length > 0)
  );

  const alphaCount = $derived((catalog.data ?? []).filter((p) => p.status === 'alpha').length);
</script>

<svelte:head><title>Connections · yari.run</title></svelte:head>

<PageHeader
  title="Connections"
  description="The clouds and services yari.run can deploy to, using accounts you already pay for. yari.run never hosts your workloads."
/>

<div class="mb-5 space-y-3">
  <div class="relative max-w-sm">
    <Search
      class="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
    />
    <Input
      bind:value={search}
      placeholder="Search providers or kinds"
      class="h-8 pl-8 text-[13px]"
    />
  </div>

  <div class="flex flex-wrap gap-1.5" role="group" aria-label="Filter by category">
    {#each ['all', ...categories] as c (c)}
      <button
        type="button"
        onclick={() => (category = c)}
        aria-pressed={category === c}
        class={cn(
          'rounded-full border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground',
          category === c && 'border-foreground/20 bg-accent text-foreground'
        )}
      >
        {c === 'all' ? 'All' : categoryLabel(c)}
      </button>
    {/each}
  </div>
</div>

{#if catalog.isPending}
  <div class="grid gap-3 sm:grid-cols-2">
    {#each [0, 1, 2, 3] as i (i)}<Skeleton class="h-[92px] rounded-xl" />{/each}
  </div>
{:else if catalog.isError}
  <p class="text-sm text-destructive">Could not load the catalog: {catalog.error.message}</p>
{:else if groups.length === 0}
  <EmptyState
    title="No providers match"
    description="Try a different search or clear the category filter."
  />
{:else}
  <p class="mb-5 text-xs text-muted-foreground">
    {catalog.data.length} providers · {alphaCount} with a working adapter (alpha)
  </p>
  <div class="space-y-8">
    {#each groups as g (g.id)}
      <section>
        <h2 class="mb-2.5 text-xs font-medium text-muted-foreground">
          {categoryLabel(g.id)}
        </h2>
        <div class="grid gap-3 sm:grid-cols-2">
          {#each g.items as p (p.id)}
            <article class="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="truncate text-sm font-medium">{p.name}</h3>
                    {#if p.status === 'alpha'}
                      <Badge
                        variant="secondary"
                        class="h-[18px] rounded-md px-1.5 text-[10.5px] text-primary">alpha</Badge
                      >
                    {:else}
                      <Badge
                        variant="outline"
                        class="h-[18px] rounded-md px-1.5 text-[10.5px] text-muted-foreground"
                        >planned</Badge
                      >
                    {/if}
                  </div>
                  <p class="mt-1 text-xs text-muted-foreground">
                    {p.kinds.slice(0, 6).join(', ')}{p.kinds.length > 6
                      ? ` +${p.kinds.length - 6}`
                      : ''}
                  </p>
                </div>
                {#if p.docs}
                  <a
                    href={p.docs}
                    target="_blank"
                    rel="noreferrer"
                    class="mt-0.5 shrink-0 text-muted-foreground hover:text-foreground"
                    aria-label="{p.name} documentation"
                  >
                    <ExternalLink class="size-3.5" />
                  </a>
                {/if}
              </div>
              <div class="mt-auto flex items-center justify-between gap-3">
                <p class="text-[11px] text-muted-foreground">
                  {p.auth.map(authLabel).join(', ')} · {p.adapter} adapter
                </p>
                <Button
                  size="xs"
                  variant={p.status === 'alpha' ? 'default' : 'outline'}
                  disabled={p.status === 'planned'}
                  title={p.status === 'planned' ? 'Coming soon' : undefined}>Connect</Button
                >
              </div>
            </article>
          {/each}
        </div>
      </section>
    {/each}
  </div>
{/if}
