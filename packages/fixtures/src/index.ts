import catalogJson from './catalog.json';
import type { ProviderInfo } from './types';

export * from './types';
export * from './hierarchy';
export * from './services';
export * from './plans';
export * from './audit';
export * from './access';
export * from './agent';
export * from './people';

/** A copy of the Provider Catalog. Keys: id, name, category, kinds, auth, adapter, status, docs. */
export const catalog: ProviderInfo[] = catalogJson as ProviderInfo[];

/** Display names for providers that the demo uses, in case the catalog uses other ids. */
const fallbackNames: Record<string, string> = {
  railway: 'Railway',
  vercel: 'Vercel',
  supabase: 'Supabase',
  upstash: 'Upstash',
  resend: 'Resend',
  'backblaze-b2': 'Backblaze B2',
  b2: 'Backblaze B2',
  backblaze_b2: 'Backblaze B2'
};

export function providerName(id: string): string {
  return catalog.find((p) => p.id === id)?.name ?? fallbackNames[id] ?? id;
}
