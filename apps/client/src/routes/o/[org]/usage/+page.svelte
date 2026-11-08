<script lang="ts">
  import PageHeader from '#lib/components/common/PageHeader.svelte';
  import { usageQuery } from '#lib/data/queries.js';
  import { money } from '#lib/format.js';
  import { getOrgContext } from '#lib/org-context.js';

  const ctx = getOrgContext();
  const usage = usageQuery(() => ctx.org?.id);
</script>

<svelte:head><title>Usage · yari.run</title></svelte:head>

<PageHeader
  title="Usage"
  description="An estimate of what your connected providers bill you, plus agent spend. yari.run does not bill for workloads."
/>

{#if usage.data}
  <div class="rounded-xl border border-border bg-card p-5">
    <p class="text-xs text-muted-foreground">{usage.data.period}</p>
    <p class="mt-1 text-2xl font-semibold tracking-tight tabular-nums">
      {money(usage.data.totalCents)}
    </p>
    <ul class="mt-5 divide-y border-t border-border">
      {#each usage.data.rows as r (r.label)}
        <li class="flex items-center justify-between py-2.5 text-[13px]">
          <span class="text-muted-foreground">{r.label}</span>
          <span class="tabular-nums">{money(r.cents)}</span>
        </li>
      {/each}
    </ul>
  </div>
{:else}
  <p class="text-sm text-muted-foreground">Loading...</p>
{/if}
