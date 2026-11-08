<script lang="ts">
  import { page } from '$app/state';
  import Activity from '@lucide/svelte/icons/activity';
  import Bot from '@lucide/svelte/icons/bot';
  import FolderKanban from '@lucide/svelte/icons/folder-kanban';
  import Inbox from '@lucide/svelte/icons/inbox';
  import Settings from '@lucide/svelte/icons/settings';
  import { approvalsQuery } from '#lib/data/queries.js';
  import { cn } from '#lib/utils.js';

  const approvals = approvalsQuery();
  const pending = $derived(approvals.data?.length ?? 0);

  const tabs = [
    { href: '/m', label: 'Projects', icon: FolderKanban, exact: true },
    { href: '/m/approvals', label: 'Approvals', icon: Inbox },
    { href: '/m/agent', label: 'Agent', icon: Bot },
    { href: '/m/activity', label: 'Activity', icon: Activity },
    { href: '/m/settings', label: 'Settings', icon: Settings }
  ];

  function active(href: string, exact?: boolean) {
    const p = page.url.pathname;
    return exact ? p === href : p === href || p.startsWith(href + '/');
  }
</script>

<nav class="pb-safe shrink-0 border-t border-border bg-surface" aria-label="Main">
  <ul class="grid grid-cols-5">
    {#each tabs as t (t.href)}
      {@const Icon = t.icon}
      <li>
        <a
          href={t.href}
          aria-current={active(t.href, t.exact) ? 'page' : undefined}
          class={cn(
            'relative flex flex-col items-center gap-1 py-2.5 text-[10.5px] text-muted-foreground',
            active(t.href, t.exact) && 'text-primary'
          )}
        >
          <Icon class="size-5" />
          {t.label}
          {#if t.href === '/m/approvals' && pending > 0}
            <span
              class="absolute top-1.5 left-1/2 ml-2 min-w-4 rounded-full bg-primary px-1 text-center text-[10px] leading-4 text-primary-foreground"
              >{pending}</span
            >
          {/if}
        </a>
      </li>
    {/each}
  </ul>
</nav>
