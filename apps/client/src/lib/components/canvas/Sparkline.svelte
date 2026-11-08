<script lang="ts">
  let { points, class: className = '' }: { points: number[]; class?: string } = $props();

  const W = 200;
  const H = 44;
  const path = $derived.by(() => {
    if (points.length < 2) return '';
    const min = Math.min(...points);
    const max = Math.max(...points);
    const span = max - min || 1;
    return points
      .map((p, i) => {
        const x = (i / (points.length - 1)) * W;
        const y = H - 4 - ((p - min) / span) * (H - 8);
        return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
  });
</script>

<svg
  viewBox="0 0 {W} {H}"
  class="h-11 w-full {className}"
  preserveAspectRatio="none"
  aria-hidden="true"
>
  <path
    d={path}
    fill="none"
    class="stroke-primary"
    stroke-width="1.5"
    vector-effect="non-scaling-stroke"
  />
</svg>
