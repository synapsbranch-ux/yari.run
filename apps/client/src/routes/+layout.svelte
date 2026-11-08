<script lang="ts">
  import '../app.css';
  import { onMount, type Snippet } from 'svelte';
  import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
  import { goto as navigate } from '$app/navigation';
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

    // The desktop tray asks to open a route. Navigating in the app keeps the in-memory token store,
    // which a full page load would drop.
    const onNavigate = (e: Event) => {
      const path = (e as CustomEvent<{ path: string }>).detail?.path;
      if (typeof path !== 'string' || !path.startsWith('/')) return;
      e.preventDefault();
      void navigate(path);
    };
    window.addEventListener('yari:navigate', onNavigate);
    return () => window.removeEventListener('yari:navigate', onNavigate);
  });
</script>

<QueryClientProvider client={queryClient}>
  <Tooltip.Provider delayDuration={300}>
    <CatalogSync />
    {@render children()}
  </Tooltip.Provider>
</QueryClientProvider>
