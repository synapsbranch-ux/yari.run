<script lang="ts">
  import type { Snippet } from 'svelte';
  import Sidebar from './Sidebar.svelte';

  let { orgSlug, children }: { orgSlug: string; children: Snippet } = $props();
</script>

<div class="flex h-dvh flex-col md:flex-row">
  <Sidebar {orgSlug} />
  <main class="min-w-0 flex-1 overflow-y-auto">
    <div class="mx-auto w-full max-w-5xl px-5 py-6 md:px-10 md:py-9">
      {@render children()}
    </div>
  </main>
</div>
