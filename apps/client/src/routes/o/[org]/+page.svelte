<script lang="ts">
  import Plus from '@lucide/svelte/icons/plus';
  import EmptyState from '#lib/components/common/EmptyState.svelte';
  import KindIcon from '#lib/components/common/KindIcon.svelte';
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import StatusDot from '#lib/components/common/StatusDot.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import { Skeleton } from '#lib/components/ui/skeleton/index.js';
  import { projectSummariesQuery } from '#lib/data/queries.js';
  import { getOrgContext } from '#lib/org-context.js';
  import type { ServiceStatus } from '#lib/data/types';

  const ctx = getOrgContext();
  const summaries = projectSummariesQuery(() => ctx.org?.id);

  function line(s: { environment: { name: string } | null; services: { status: string }[] }) {
    if (!s.environment) return 'No environments yet';
    const online = s.services.filter((x) => x.status === 'online').length;
    return `${s.environment.name.toLowerCase()} · ${online}/${s.services.length} services online`;
  }

  function health(services: { status: string }[]): ServiceStatus {
    if (services.some((s) => s.status === 'failed')) return 'failed';
    if (services.some((s) => s.status === 'deploying')) return 'deploying';
    if (services.length === 0) return 'draft';
    return services.every((s) => s.status === 'online') ? 'online' : 'sleeping';
  }
</script>

<svelte:head><title>Projects · {ctx.org?.name ?? 'yari.run'}</title></svelte:head>

<PageHeader
  title="Projects"
  description="Every project across the workspaces of {ctx.org?.name ?? 'this organization'}."
>
  {#snippet actions()}
    <Button size="sm" disabled><Plus class="size-3.5" /> New project</Button>
  {/snippet}
</PageHeader>

{#if summaries.isPending}
  <div class="grid gap-3 sm:grid-cols-2">
    {#each [0, 1, 2, 3] as i (i)}<Skeleton class="h-[108px] rounded-xl" />{/each}
  </div>
{:else if summaries.isError}
  <p class="text-sm text-destructive">Could not load projects: {summaries.error.message}</p>
{:else if summaries.data.length === 0}
  <EmptyState
    title="No projects yet"
    description="Create a project to start drawing services on a canvas."
  />
{:else}
  <div class="grid gap-3 sm:grid-cols-2">
    {#each summaries.data as s (s.project.id)}
      <a
        href={s.environment
          ? `/o/${ctx.slug}/w/${s.workspace.slug}/p/${s.project.slug}/e/${s.environment.slug}`
          : `/o/${ctx.slug}`}
        class="group flex flex-col gap-4 rounded-xl border border-border bg-card p-4 transition-colors hover:border-foreground/20"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium">{s.project.name}</p>
            <p class="mt-0.5 text-xs text-muted-foreground">{s.workspace.name}</p>
          </div>
          <StatusDot status={health(s.services)} class="mt-1.5" />
        </div>

        <div class="flex items-center gap-1.5">
          {#each s.services.slice(0, 6) as svc (svc.id)}
            <span
              class="flex size-6 items-center justify-center rounded-md bg-muted text-muted-foreground"
              title="{svc.name} ({svc.kind})"
            >
              <KindIcon kind={svc.kind} class="size-3.5" />
            </span>
          {/each}
          {#if s.services.length > 6}
            <span class="text-xs text-muted-foreground">+{s.services.length - 6}</span>
          {/if}
        </div>

        <p class="text-xs text-muted-foreground">{line(s)}</p>
      </a>
    {/each}
  </div>
{/if}
