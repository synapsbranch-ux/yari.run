/** "+$24.00" or "-$18.00" from cents. */
export function signedMoney(cents: number): string {
  const abs = Math.abs(cents) / 100;
  const text = abs.toLocaleString('en-US', { style: 'currency', currency: 'USD' });
  if (cents === 0) return text;
  return `${cents > 0 ? '+' : '-'}${text}`;
}

export function money(cents: number): string {
  return (cents / 100).toLocaleString('en-US', { style: 'currency', currency: 'USD' });
}

/** Cost from USD micros, for budgets and spend: "$0.42". */
export function usdMicros(micros: number): string {
  return (micros / 1_000_000).toLocaleString('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2
  });
}

const RTF = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'short' });

export function relativeTime(iso: string, now = Date.now()): string {
  const diff = (new Date(iso).getTime() - now) / 1000;
  const abs = Math.abs(diff);
  if (abs < 45) return 'just now';
  if (abs < 3600) return RTF.format(Math.round(diff / 60), 'minute');
  if (abs < 86400) return RTF.format(Math.round(diff / 3600), 'hour');
  return RTF.format(Math.round(diff / 86400), 'day');
}

export function clockTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  });
}

export function titleCase(value: string): string {
  return value
    .replace(/[_-]+/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
    .replace(/\bPaas\b/, 'PaaS')
    .replace(/\bCdn Dns\b/, 'CDN and DNS');
}

export const CATEGORY_LABELS: Record<string, string> = {
  paas: 'PaaS',
  cloud: 'Cloud (IaaS)',
  database: 'Databases',
  storage: 'Storage',
  cdn_dns: 'CDN and DNS',
  email: 'Email',
  messaging: 'Messaging',
  search: 'Search',
  observability: 'Observability',
  auth: 'Auth',
  source: 'Source'
};

export function categoryLabel(id: string): string {
  return CATEGORY_LABELS[id] ?? titleCase(id);
}

const AUTH_LABELS: Record<string, string> = {
  oauth: 'OAuth',
  api_token: 'API token',
  api_key: 'API key',
  access_key: 'Access key',
  iam_role: 'IAM role',
  workload_identity: 'Workload identity'
};

export function authLabel(id: string): string {
  return AUTH_LABELS[id] ?? titleCase(id);
}
