<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    title,
    subtitle,
    action,
    children
  }: { title: string; subtitle?: string; action?: Snippet; children: Snippet } = $props();
</script>

<div class="px-4 pt-5 pb-6">
  <header class="mb-4 flex items-end justify-between gap-3">
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{title}</h1>
      {#if subtitle}<p class="mt-0.5 text-xs text-muted-foreground">{subtitle}</p>{/if}
    </div>
    {#if action}{@render action()}{/if}
  </header>
  {@render children()}
</div>
