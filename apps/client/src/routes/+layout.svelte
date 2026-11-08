<script lang="ts">
  import '../app.css';
  import { onMount, type Snippet } from 'svelte';
  import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
  import { session } from '#lib/session.svelte.js';
  import { theme } from '#lib/theme.svelte.js';
  import * as Tooltip from '#lib/components/ui/tooltip/index.js';
  import CatalogSync from '#lib/components/common/CatalogSync.svelte';

  let { children }: { children: Snippet } = $props();

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { staleTime: 15_000, retry: 1, refetchOnWindowFocus: false }
    }
  });

  onMount(() => {
    theme.init();
    void session.hydrate();
  });
</script>

<QueryClientProvider client={queryClient}>
  <Tooltip.Provider delayDuration={300}>
    <CatalogSync />
    {@render children()}
  </Tooltip.Provider>
</QueryClientProvider>
