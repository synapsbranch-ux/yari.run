<script lang="ts">
  import { page } from '$app/state';
  import ChevronLeft from '@lucide/svelte/icons/chevron-left';
  import SessionView from '#lib/components/agent/SessionView.svelte';

  const id = $derived(page.params.id ?? '');
</script>

<svelte:head><title>Agent session · yari.run</title></svelte:head>

<div class="flex h-full flex-col">
  <a href="/m/agent" class="flex items-center gap-1 px-3 pt-4 pb-2 text-xs text-muted-foreground">
    <ChevronLeft class="size-4" /> Sessions
  </a>
  <div class="min-h-0 flex-1">
    {#key id}<SessionView sessionId={id} readonly />{/key}
  </div>
</div>
