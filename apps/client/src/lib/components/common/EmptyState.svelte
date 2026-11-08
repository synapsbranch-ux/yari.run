<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '#lib/utils.js';

  let {
    title,
    description,
    class: className,
    children
  }: { title: string; description?: string; class?: string; children?: Snippet } = $props();
</script>

<div
  class={cn(
    'flex flex-col items-center gap-1.5 rounded-lg border border-dashed border-border px-6 py-12 text-center',
    className
  )}
>
  <p class="text-sm font-medium">{title}</p>
  {#if description}<p class="max-w-sm text-xs text-muted-foreground">{description}</p>{/if}
  {#if children}<div class="mt-3">{@render children()}</div>{/if}
</div>
