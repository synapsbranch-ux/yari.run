<script lang="ts">
  import type { Snippet } from 'svelte';

  let { title, description, actions }: { title: string; description?: string; actions?: Snippet } =
    $props();
</script>

<header class="mb-6 flex items-start justify-between gap-4">
  <div>
    <h1 class="text-lg font-semibold tracking-tight">{title}</h1>
    {#if description}<p class="mt-1 text-[13px] text-muted-foreground">{description}</p>{/if}
  </div>
  {#if actions}<div class="flex shrink-0 items-center gap-2">{@render actions()}</div>{/if}
</header>
