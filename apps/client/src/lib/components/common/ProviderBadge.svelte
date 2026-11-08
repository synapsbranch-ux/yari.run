<script lang="ts">
  import { providerLabel } from '#lib/catalog.svelte';
  import { cn } from '#lib/utils.js';

  /** Plain text provider badge. No logos. */
  let {
    provider,
    name,
    class: className
  }: { provider: string; name?: string; class?: string } = $props();
</script>

<span
  class={cn(
    'inline-flex h-[18px] items-center rounded-full border border-border bg-muted/40 px-1.5 text-[10.5px] leading-none font-medium whitespace-nowrap text-muted-foreground',
    className
  )}>{name ?? providerLabel(provider)}</span
>
