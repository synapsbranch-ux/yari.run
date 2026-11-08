import { providerName as fixtureName } from '@yari/fixtures';
import type { ProviderInfo } from '#lib/data/types';

/** Provider names from whichever catalog is active, filled by the root layout. */
let names = $state<Record<string, string>>({});

export function setCatalogNames(list: ProviderInfo[]) {
  names = Object.fromEntries(list.map((p) => [p.id, p.name]));
}

export function providerLabel(id: string): string {
  return names[id] ?? fixtureName(id);
}
