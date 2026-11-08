<script lang="ts">
  import { cn } from '#lib/utils.js';
  import { signedMoney } from '#lib/format';

  let {
    cents,
    suffix = '/mo',
    class: className
  }: { cents: number; suffix?: string; class?: string } = $props();
</script>

<span class={cn('tabular-nums', className)}
  >{signedMoney(cents)}<span class="text-[0.72em] text-muted-foreground">{suffix}</span></span
>
