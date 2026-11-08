<script lang="ts">
  import KindIcon from '#lib/components/common/KindIcon.svelte';
  import StatusDot from '#lib/components/common/StatusDot.svelte';
  import Screen from '#lib/components/mobile/Screen.svelte';
  import { Skeleton } from '#lib/components/ui/skeleton/index.js';
  import { organizationsQuery, projectSummariesQuery } from '#lib/data/queries.js';
  import { cn } from '#lib/utils.js';
  import type { ServiceStatus } from '#lib/data/types';

  const orgs = organizationsQuery();
  let orgId = $state<string | null>(null);
  const org = $derived(orgs.data?.find((o) => o.id === orgId) ?? orgs.data?.[0]);
  const summaries = projectSummariesQuery(() => org?.id);

  function health(services: { status: string }[]): ServiceStatus {
    if (services.some((s) => s.status === 'failed')) return 'failed';
    if (services.some((s) => s.status === 'deploying')) return 'deploying';
    if (services.length === 0) return 'draft';
    return services.every((s) => s.status === 'online') ? 'online' : 'sleeping';
  }
</script>

<svelte:head><title>Projects · yari.run</title></svelte:head>

<Screen title="Projects" subtitle="Status of every environment">
  {#if (orgs.data?.length ?? 0) > 1}
    <div class="mb-4 flex gap-1.5 overflow-x-auto" role="group" aria-label="Organization">
      {#each orgs.data ?? [] as o (o.id)}
        <button
          type="button"
          onclick={() => (orgId = o.id)}
          aria-pressed={org?.id === o.id}
          class={cn(
            'shrink-0 rounded-full border border-border px-3 py-1 text-xs text-muted-foreground',
            org?.id === o.id && 'border-foreground/20 bg-accent text-foreground'
          )}>{o.name}</button
        >
      {/each}
    </div>
  {/if}

  {#if summaries.isPending}
    <div class="space-y-2">
      {#each [0, 1, 2, 3] as i (i)}<Skeleton class="h-[72px] rounded-xl" />{/each}
    </div>
  {:else if summaries.isError}
    <p class="text-sm text-destructive">Could not load projects: {summaries.error.message}</p>
  {:else}
    <ul class="space-y-2">
      {#each summaries.data as s (s.project.id)}
        {@const online = s.services.filter((x) => x.status === 'online').length}
        <li class="rounded-xl border border-border bg-card p-3.5">
          <div class="flex items-center justify-between gap-3">
            <p class="truncate text-sm font-medium">{s.project.name}</p>
            <StatusDot status={health(s.services)} />
          </div>
          <p class="mt-0.5 text-xs text-muted-foreground">
            {s.workspace.name} · {s.environment
              ? s.environment.name.toLowerCase()
              : 'no environment'}
            · {online}/{s.services.length} services online
          </p>
          <div class="mt-2.5 flex flex-wrap gap-1.5">
            {#each s.services as svc (svc.id)}
              <span
                class="inline-flex items-center gap-1.5 rounded-md bg-muted py-0.5 pr-2 pl-1.5 text-[11px] text-muted-foreground"
              >
                <KindIcon kind={svc.kind} class="size-3" />
                {svc.name}
                <StatusDot status={svc.status as ServiceStatus} />
              </span>
            {/each}
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</Screen>
