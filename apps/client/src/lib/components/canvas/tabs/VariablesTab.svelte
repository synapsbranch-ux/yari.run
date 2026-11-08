<script lang="ts">
  import EyeOff from '@lucide/svelte/icons/eye-off';
  import Lock from '@lucide/svelte/icons/lock';
  import type { Edge, Service } from '#lib/data/types';

  let { service, services, edges }: { service: Service; services: Service[]; edges: Edge[] } =
    $props();

  const injected = $derived(
    edges
      .filter((e) => e.from === service.id)
      .flatMap((e) => {
        const dep = services.find((s) => s.id === e.to);
        return e.variables.map((v) => ({ ...v, source: dep?.name ?? 'unknown', dep }));
      })
  );

  const own = [
    { name: 'NODE_ENV', value: 'production' },
    { name: 'PORT', value: '8080' },
    { name: 'LOG_LEVEL', value: 'info' }
  ];

  function shown(v: (typeof injected)[number]): string {
    if (v.secret) return '';
    if (v.fromOutput === 'url' && v.dep?.domain) return `https://${v.dep.domain}`;
    if (v.fromOutput === 'bucket') return v.dep?.name ?? 'bucket';
    if (v.fromOutput === 'endpoint') return 's3.example.invalid';
    return '';
  }
</script>

<div class="space-y-6">
  <section>
    <h3 class="mb-1 text-[13px] font-medium">Injected by edges</h3>
    <p class="mb-3 text-xs text-muted-foreground">
      Values come from the services this one depends on. They are resolved at deploy time and secret
      values are never shown here.
    </p>
    {#if injected.length === 0}
      <p class="rounded-lg border border-dashed border-border p-4 text-xs text-muted-foreground">
        This service does not depend on another service.
      </p>
    {:else}
      <ul class="divide-y divide-border overflow-hidden rounded-lg border border-border bg-card">
        {#each injected as v (v.source + v.name)}
          <li class="flex items-center justify-between gap-3 px-3.5 py-2.5">
            <div class="min-w-0">
              <p class="font-mono text-xs">{v.name}</p>
              <p class="mt-0.5 text-[11px] text-muted-foreground">
                injected from <span class="text-foreground">{v.source}</span>
              </p>
            </div>
            <span
              class="flex shrink-0 items-center gap-1.5 font-mono text-xs text-muted-foreground"
            >
              {#if v.secret}
                <Lock class="size-3" /> ••••••••••
              {:else}
                {shown(v) || '••••••••'}
              {/if}
            </span>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <section>
    <h3 class="mb-3 text-[13px] font-medium">Service variables</h3>
    <ul class="divide-y divide-border overflow-hidden rounded-lg border border-border bg-card">
      {#each own as v (v.name)}
        <li class="flex items-center justify-between gap-3 px-3.5 py-2.5">
          <p class="font-mono text-xs">{v.name}</p>
          <span class="font-mono text-xs text-muted-foreground">{v.value}</span>
        </li>
      {/each}
    </ul>
    <p class="mt-3 flex items-start gap-1.5 text-xs text-muted-foreground">
      <EyeOff class="mt-0.5 size-3.5 shrink-0" />
      <span>
        Reading secret values needs <code class="rounded bg-muted px-1">secret.read</code>. Agents
        never have it.
      </span>
    </p>
  </section>
</div>
