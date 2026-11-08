<script lang="ts">
  import { useSvelteFlow } from '@xyflow/svelte';
  import { untrack } from 'svelte';
  import { getCanvasState } from '#lib/canvas.svelte.js';

  // The side panel changes the size of the canvas. Fit the graph again when it opens or closes.
  const canvas = getCanvasState();
  const { fitView } = useSvelteFlow();

  let first = true;
  $effect(() => {
    const open = canvas.dock !== null;
    void open;
    if (first) {
      first = false;
      return;
    }
    const timer = setTimeout(
      () => untrack(() => fitView({ padding: 0.18, maxZoom: 1.15, duration: 250 })),
      60
    );
    return () => clearTimeout(timer);
  });
</script>
