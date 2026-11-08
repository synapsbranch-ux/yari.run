<script lang="ts">
  import {
    Background,
    BackgroundVariant,
    Controls,
    MarkerType,
    SvelteFlow,
    SvelteFlowProvider,
    type Edge as FlowEdge,
    type Node as FlowNode
  } from '@xyflow/svelte';
  import { untrack } from 'svelte';
  import { SvelteMap } from 'svelte/reactivity';
  import { getCanvasState } from '#lib/canvas.svelte.js';
  import { theme } from '#lib/theme.svelte.js';
  import type { Edge, Service } from '#lib/data/types';
  import { autoLayout, COLUMN_WIDTH, ROW_HEIGHT } from './layout';
  import FitOnDock from './FitOnDock.svelte';
  import ServiceNode from './ServiceNode.svelte';
  import VariableEdge from './VariableEdge.svelte';

  let { services, edges }: { services: Service[]; edges: Edge[] } = $props();

  const canvas = getCanvasState();
  const nodeTypes = { service: ServiceNode };
  const edgeTypes = { variable: VariableEdge };

  let nodes = $state.raw<FlowNode[]>([]);
  let flowEdges = $state.raw<FlowEdge[]>([]);

  // Positions the user dragged to, kept across data refreshes.
  const dragged = new SvelteMap<string, { x: number; y: number }>();

  $effect(() => {
    const all = [...services, ...canvas.drafts];
    const fallback = autoLayout(all, edges);
    const selectedId = untrack(() => canvas.selectedId);
    const place = (s: Service) => dragged.get(s.id) ?? s.position ?? fallback.get(s.id);
    const placed = services.map((s) => place(s) ?? { x: 0, y: 0 });
    const rightEdge = placed.reduce((m, p) => Math.max(m, p.x), 0);
    const top = placed.reduce((m, p) => Math.min(m, p.y), Infinity);
    nodes = all.map((s) => {
      const draftIndex = canvas.drafts.findIndex((d) => d.id === s.id);
      const position =
        draftIndex >= 0
          ? (dragged.get(s.id) ?? {
              x: rightEdge + COLUMN_WIDTH - 40,
              y: top + draftIndex * ROW_HEIGHT
            })
          : (place(s) ?? { x: 0, y: 0 });
      return {
        id: s.id,
        type: 'service',
        position,
        data: { service: s },
        selected: s.id === selectedId
      };
    });
    flowEdges = edges.map((e) => ({
      id: e.id,
      source: e.from,
      target: e.to,
      type: 'variable',
      data: { variables: e.variables },
      markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 }
    }));
  });

  // Keep the Flow selection in step with the panel state.
  $effect(() => {
    const id = canvas.selectedId;
    const current = untrack(() => nodes);
    if (current.some((n) => !!n.selected !== (n.id === id))) {
      nodes = current.map((n) => ({ ...n, selected: n.id === id }));
    }
  });
</script>

<SvelteFlowProvider>
  <SvelteFlow
    bind:nodes
    edges={flowEdges}
    {nodeTypes}
    {edgeTypes}
    colorMode={theme.current}
    fitView
    fitViewOptions={{ padding: 0.18, maxZoom: 1.15 }}
    minZoom={0.3}
    maxZoom={1.6}
    nodesConnectable={false}
    elementsSelectable
    deleteKey={null}
    proOptions={{ hideAttribution: true }}
    onnodeclick={({ node }) => canvas.select(node.id)}
    onpaneclick={() => canvas.select(null)}
    onnodedragstop={({ targetNode }) => {
      if (targetNode) dragged.set(targetNode.id, { ...targetNode.position });
    }}
  >
    <Background variant={BackgroundVariant.Dots} gap={22} size={1.4} />
    <Controls position="bottom-left" showLock={false} />
    <FitOnDock />
  </SvelteFlow>
</SvelteFlowProvider>
