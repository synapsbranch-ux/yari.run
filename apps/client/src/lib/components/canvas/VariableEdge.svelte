<script lang="ts">
  import { BaseEdge, EdgeLabel, getBezierPath, type Edge, type EdgeProps } from '@xyflow/svelte';
  import { cn } from '#lib/utils.js';
  import type { InjectedVariable } from '#lib/data/types';

  type VariableEdgeType = Edge<{ variables: InjectedVariable[] }, 'variable'>;

  let {
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    data,
    selected,
    markerEnd
  }: EdgeProps<VariableEdgeType> = $props();

  const geometry = $derived(
    getBezierPath({ sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition })
  );
  // Label on the last third of the curve, so labels of edges that fan out of one service
  // do not stack on top of each other near the source.
  const labelAt = $derived.by(() => {
    const t = 0.72;
    const off = Math.max(Math.abs(targetX - sourceX) * 0.5, 20);
    const c1x = sourceX + off;
    const c2x = targetX - off;
    const u = 1 - t;
    const x = u ** 3 * sourceX + 3 * u * u * t * c1x + 3 * u * t * t * c2x + t ** 3 * targetX;
    const y =
      u ** 3 * sourceY + 3 * u * u * t * sourceY + 3 * u * t * t * targetY + t ** 3 * targetY;
    return { x, y };
  });
  const variables = $derived(data?.variables ?? []);
  const label = $derived(
    variables.length === 0
      ? ''
      : variables.length === 1
        ? variables[0].name
        : `${variables[0].name} +${variables.length - 1}`
  );
</script>

<BaseEdge {id} path={geometry[0]} {markerEnd} />
{#if label}
  <EdgeLabel x={labelAt.x} y={labelAt.y}>
    <span
      class={cn(
        'rounded-md border border-border bg-card px-1.5 py-0.5 font-mono text-[10px] leading-none whitespace-nowrap text-muted-foreground',
        selected && 'border-primary text-foreground'
      )}
      title={variables.map((v) => v.name).join(', ')}>{label}</span
    >
  </EdgeLabel>
{/if}
