<script lang="ts">
  import { kindIcon } from '#lib/kinds';

  let { kind, class: className = 'size-4' }: { kind: string; class?: string } = $props();
  const Icon = $derived(kindIcon(kind));
</script>

<Icon class={className} />
