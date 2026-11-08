<script lang="ts">
  import type { Snippet } from 'svelte';
  import TabBar from '#lib/components/mobile/TabBar.svelte';

  let { children }: { children: Snippet } = $props();
</script>

<div class="pt-safe mx-auto flex h-dvh max-w-[560px] flex-col">
  <main class="min-h-0 flex-1 overflow-y-auto">
    {@render children()}
  </main>
  <TabBar />
</div>
