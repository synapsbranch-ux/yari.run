<script lang="ts">
  import { cn } from '#lib/utils.js';

  let { class: className }: { class?: string } = $props();
</script>

<span class={cn('inline-flex items-center gap-2 font-semibold tracking-tight', className)}>
  <svg viewBox="0 0 32 32" class="size-5" aria-hidden="true">
    <rect width="32" height="32" rx="8" class="fill-primary/15" />
    <path
      d="M9 9l7 8 7-8M16 17v7"
      fill="none"
      class="stroke-primary"
      stroke-width="2.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
  <span>yari<span class="text-muted-foreground">.run</span></span>
</span>
