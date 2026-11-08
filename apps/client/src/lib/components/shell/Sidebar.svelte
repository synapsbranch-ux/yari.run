<script lang="ts">
  import { page } from '$app/state';
  import Activity from '@lucide/svelte/icons/activity';
  import Check from '@lucide/svelte/icons/check';
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
  import FolderKanban from '@lucide/svelte/icons/folder-kanban';
  import Gauge from '@lucide/svelte/icons/gauge';
  import Moon from '@lucide/svelte/icons/moon';
  import Plug from '@lucide/svelte/icons/plug';
  import ScrollText from '@lucide/svelte/icons/scroll-text';
  import Settings from '@lucide/svelte/icons/settings';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import Sun from '@lucide/svelte/icons/sun';
  import Users from '@lucide/svelte/icons/users';
  import Wordmark from '#lib/components/common/Wordmark.svelte';
  import * as DropdownMenu from '#lib/components/ui/dropdown-menu/index.js';
  import { approvalsQuery, organizationsQuery } from '#lib/data/queries.js';
  import { theme } from '#lib/theme.svelte.js';
  import { cn } from '#lib/utils.js';

  let { orgSlug }: { orgSlug: string } = $props();

  const orgs = organizationsQuery();
  const approvals = approvalsQuery();

  const org = $derived(orgs.data?.find((o) => o.slug === orgSlug));
  const pending = $derived(approvals.data?.length ?? 0);

  const items = $derived([
    { href: `/o/${orgSlug}`, label: 'Projects', icon: FolderKanban, exact: true },
    { href: `/o/${orgSlug}/connections`, label: 'Connections', icon: Plug },
    { href: `/o/${orgSlug}/members`, label: 'Members & Teams', icon: Users },
    { href: `/o/${orgSlug}/roles`, label: 'Roles', icon: ShieldCheck },
    { href: `/o/${orgSlug}/audit`, label: 'Audit log', icon: ScrollText },
    { href: `/o/${orgSlug}/usage`, label: 'Usage', icon: Gauge },
    { href: `/o/${orgSlug}/settings`, label: 'Settings', icon: Settings }
  ]);

  function active(href: string, exact = false) {
    const path = page.url.pathname;
    return exact ? path === href : path === href || path.startsWith(href + '/');
  }
</script>

<aside
  class="flex shrink-0 flex-col border-b border-border bg-surface md:w-[232px] md:border-r md:border-b-0"
>
  <div class="flex items-center justify-between px-4 pt-4 pb-3 md:block md:space-y-4">
    <a href="/o/{orgSlug}" class="inline-flex"><Wordmark class="text-[15px]" /></a>

    <DropdownMenu.Root>
      <DropdownMenu.Trigger
        class="flex w-40 items-center justify-between gap-2 rounded-md border border-border px-2.5 py-1.5 text-left hover:bg-accent md:w-full"
      >
        <span class="flex min-w-0 items-center gap-2">
          <span
            class="flex size-5 shrink-0 items-center justify-center rounded bg-primary/15 text-[10px] font-semibold text-primary"
            >{(org?.name ?? orgSlug)[0]?.toUpperCase()}</span
          >
          <span class="truncate text-[13px] font-medium">{org?.name ?? orgSlug}</span>
        </span>
        <ChevronsUpDown class="size-3.5 shrink-0 text-muted-foreground" />
      </DropdownMenu.Trigger>
      <DropdownMenu.Content align="start" class="w-56">
        <DropdownMenu.Label class="text-xs text-muted-foreground">Organizations</DropdownMenu.Label>
        {#each orgs.data ?? [] as o (o.id)}
          <DropdownMenu.Item>
            {#snippet child({ props })}
              <a href="/o/{o.slug}" {...props}>
                <span class="flex-1">{o.name}</span>
                {#if o.slug === orgSlug}<Check class="size-3.5" />{/if}
              </a>
            {/snippet}
          </DropdownMenu.Item>
        {/each}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  </div>

  <nav
    class="flex gap-0.5 overflow-x-auto px-2 pb-2 md:flex-1 md:flex-col md:overflow-visible md:pb-0"
  >
    {#each items as item (item.href)}
      {@const Icon = item.icon}
      <a
        href={item.href}
        class={cn(
          'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] whitespace-nowrap text-muted-foreground hover:bg-accent hover:text-foreground',
          active(item.href, item.exact) && 'bg-accent text-foreground'
        )}
      >
        <Icon class="size-4 shrink-0" />
        {item.label}
      </a>
    {/each}

    <div class="mx-2 my-2 hidden h-px bg-border md:block"></div>

    <a
      href="/approvals"
      class={cn(
        'flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-[13px] whitespace-nowrap text-muted-foreground hover:bg-accent hover:text-foreground',
        active('/approvals') && 'bg-accent text-foreground'
      )}
    >
      <Activity class="size-4 shrink-0" />
      <span class="flex-1">Approvals</span>
      {#if pending > 0}
        <span
          class="rounded-full bg-primary/15 px-1.5 text-[11px] leading-[18px] font-medium text-primary"
          >{pending}</span
        >
      {/if}
    </a>
  </nav>

  <div class="hidden items-center justify-between p-3 md:flex">
    <a href="/m" class="text-xs text-muted-foreground hover:text-foreground">Mobile view</a>
    <button
      type="button"
      class="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
      onclick={() => theme.toggle()}
      aria-label="Toggle theme"
    >
      {#if theme.current === 'dark'}<Sun class="size-4" />{:else}<Moon class="size-4" />{/if}
    </button>
  </div>
</aside>
