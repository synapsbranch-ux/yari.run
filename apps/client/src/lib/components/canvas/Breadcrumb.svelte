<script lang="ts">
  import Check from '@lucide/svelte/icons/check';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import Lock from '@lucide/svelte/icons/lock';
  import * as DropdownMenu from '#lib/components/ui/dropdown-menu/index.js';
  import type { HierarchyContext } from '#lib/data/types';

  let { ctx }: { ctx: HierarchyContext } = $props();

  const org = $derived(ctx.organization.slug);
  const ws = $derived(ctx.workspace.slug);
  const project = $derived(ctx.project.slug);

  const levels = $derived([
    {
      label: 'Organization',
      current: ctx.organization.name,
      items: ctx.organizations.map((o) => ({
        id: o.id,
        name: o.name,
        href: `/o/${o.slug}`,
        active: o.id === ctx.organization.id
      }))
    },
    {
      label: 'Workspace',
      current: ctx.workspace.name,
      items: ctx.workspaces.map((w) => ({
        id: w.id,
        name: w.name,
        href: `/o/${org}`,
        active: w.id === ctx.workspace.id
      }))
    },
    {
      label: 'Project',
      current: ctx.project.name,
      items: ctx.projects.map((p) => ({
        id: p.id,
        name: p.name,
        href: `/o/${org}/w/${ws}/p/${p.slug}/e/production`,
        active: p.id === ctx.project.id
      }))
    },
    {
      label: 'Environment',
      current: ctx.environment.name,
      locked: ctx.environment.protected,
      items: ctx.environments.map((e) => ({
        id: e.id,
        name: e.name,
        href: `/o/${org}/w/${ws}/p/${project}/e/${e.slug}`,
        active: e.id === ctx.environment.id,
        locked: e.protected
      }))
    }
  ]);
</script>

<nav aria-label="Breadcrumb" class="flex min-w-0 items-center text-[13px]">
  {#each levels as level, i (level.label)}
    {#if i > 0}<span class="px-0.5 text-muted-foreground/50 select-none" aria-hidden="true">/</span
      >{/if}
    <DropdownMenu.Root>
      <DropdownMenu.Trigger
        class="flex max-w-[160px] items-center gap-1 rounded-md px-1.5 py-1 outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring/50 {i ===
        3
          ? 'font-medium text-foreground'
          : 'text-muted-foreground hover:text-foreground'} {i < 3 ? 'max-md:hidden' : ''}"
        aria-label="{level.label}: {level.current}"
      >
        {#if level.locked}<Lock class="size-3 shrink-0" aria-label="Protected environment" />{/if}
        <span class="truncate">{level.current}</span>
        <ChevronDown class="size-3 shrink-0 opacity-60" />
      </DropdownMenu.Trigger>
      <DropdownMenu.Content align="start" class="w-52">
        <DropdownMenu.Label class="text-xs text-muted-foreground">{level.label}</DropdownMenu.Label>
        {#each level.items as item (item.id)}
          <DropdownMenu.Item>
            {#snippet child({ props })}
              <a href={item.href} {...props}>
                {#if 'locked' in item && item.locked}<Lock class="size-3" />{/if}
                <span class="flex-1 truncate">{item.name}</span>
                {#if item.active}<Check class="size-3.5" />{/if}
              </a>
            {/snippet}
          </DropdownMenu.Item>
        {/each}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  {/each}
</nav>
