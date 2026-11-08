<script lang="ts">
  import { setCatalogNames } from '#lib/catalog.svelte.js';
  import { catalogQuery } from '#lib/data/queries.js';

  // Keeps provider display names in sync with the active catalog. Renders nothing.
  const catalog = catalogQuery();
  $effect(() => {
    if (catalog.data) setCatalogNames(catalog.data);
  });
</script>
