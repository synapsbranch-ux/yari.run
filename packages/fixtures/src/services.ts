import type { Edge, Service, ServiceDetail } from './types';
import { ago } from './time';

const prod = 'env_storefront_production';
const staging = 'env_storefront_staging';

export const services: Service[] = [
  // Storefront, production
  {
    id: 'svc_web',
    environmentId: prod,
    name: 'web',
    kind: 'web',
    provider: 'vercel',
    status: 'online',
    domain: 'shop.acme.example',
    region: 'iad1',
    position: { x: 0, y: 130 },
    monthlyCostCents: 2000
  },
  {
    id: 'svc_api',
    environmentId: prod,
    name: 'api',
    kind: 'web',
    provider: 'railway',
    status: 'online',
    domain: 'api.acme.example',
    region: 'us-east',
    position: { x: 400, y: 130 },
    monthlyCostCents: 2400
  },
  {
    id: 'svc_worker',
    environmentId: prod,
    name: 'worker',
    kind: 'worker',
    provider: 'railway',
    status: 'sleeping',
    region: 'us-east',
    position: { x: 400, y: 330 },
    monthlyCostCents: 800
  },
  {
    id: 'svc_postgres',
    environmentId: prod,
    name: 'postgres',
    kind: 'postgres',
    provider: 'supabase',
    status: 'online',
    region: 'us-east-1',
    position: { x: 820, y: 170 },
    monthlyCostCents: 2500
  },
  {
    id: 'svc_media',
    environmentId: prod,
    name: 'media',
    kind: 'bucket',
    provider: 'backblaze-b2',
    status: 'online',
    domain: 'media.acme.example',
    region: 'us-west-004',
    position: { x: 820, y: 0 },
    monthlyCostCents: 600
  },
  {
    id: 'svc_email',
    environmentId: prod,
    name: 'email',
    kind: 'email',
    provider: 'resend',
    status: 'online',
    domain: 'mail.acme.example',
    position: { x: 820, y: 340 },
    monthlyCostCents: 2000
  },
  // Storefront, staging
  {
    id: 'svc_web_stg',
    environmentId: staging,
    name: 'web',
    kind: 'web',
    provider: 'vercel',
    status: 'online',
    domain: 'staging.shop.acme.example',
    position: { x: 0, y: 100 }
  },
  {
    id: 'svc_api_stg',
    environmentId: staging,
    name: 'api',
    kind: 'web',
    provider: 'railway',
    status: 'deploying',
    domain: 'api.staging.acme.example',
    position: { x: 380, y: 100 }
  },
  {
    id: 'svc_postgres_stg',
    environmentId: staging,
    name: 'postgres',
    kind: 'postgres',
    provider: 'supabase',
    status: 'online',
    position: { x: 780, y: 100 }
  },
  // Internal tools, production
  {
    id: 'svc_admin',
    environmentId: 'env_internal_production',
    name: 'admin',
    kind: 'web',
    provider: 'railway',
    status: 'online',
    domain: 'admin.acme.example'
  },
  {
    id: 'svc_support_db',
    environmentId: 'env_internal_production',
    name: 'support-db',
    kind: 'postgres',
    provider: 'supabase',
    status: 'online'
  },
  {
    id: 'svc_digest',
    environmentId: 'env_internal_production',
    name: 'digest',
    kind: 'cron',
    provider: 'railway',
    status: 'online'
  },
  {
    id: 'svc_alerts',
    environmentId: 'env_internal_production',
    name: 'alerts',
    kind: 'email',
    provider: 'resend',
    status: 'online'
  },
  // Data pipeline, production
  {
    id: 'svc_ingest',
    environmentId: 'env_pipeline_production',
    name: 'ingest',
    kind: 'web',
    provider: 'railway',
    status: 'online',
    domain: 'ingest.acme.example'
  },
  {
    id: 'svc_events',
    environmentId: 'env_pipeline_production',
    name: 'events',
    kind: 'queue',
    provider: 'upstash',
    status: 'online'
  },
  {
    id: 'svc_warehouse',
    environmentId: 'env_pipeline_production',
    name: 'warehouse',
    kind: 'postgres',
    provider: 'supabase',
    status: 'failed'
  },
  // Landing site, production
  {
    id: 'svc_site',
    environmentId: 'env_landing_production',
    name: 'site',
    kind: 'static',
    provider: 'vercel',
    status: 'online',
    domain: 'acme.example'
  },
  {
    id: 'svc_forms',
    environmentId: 'env_landing_production',
    name: 'forms',
    kind: 'function',
    provider: 'vercel',
    status: 'online'
  },
  // Billing, production
  {
    id: 'svc_billing_api',
    environmentId: 'env_billing_production',
    name: 'billing-api',
    kind: 'web',
    provider: 'railway',
    status: 'online',
    domain: 'billing.northwind.example'
  },
  {
    id: 'svc_billing_db',
    environmentId: 'env_billing_production',
    name: 'billing-db',
    kind: 'postgres',
    provider: 'supabase',
    status: 'online'
  }
];

export const edges: Edge[] = [
  {
    id: 'edge_web_api',
    environmentId: prod,
    from: 'svc_web',
    to: 'svc_api',
    variables: [{ name: 'API_URL', fromOutput: 'url', secret: false }]
  },
  {
    id: 'edge_api_postgres',
    environmentId: prod,
    from: 'svc_api',
    to: 'svc_postgres',
    variables: [{ name: 'DATABASE_URL', fromOutput: 'connection_url', secret: true }]
  },
  {
    id: 'edge_api_media',
    environmentId: prod,
    from: 'svc_api',
    to: 'svc_media',
    variables: [
      { name: 'S3_ENDPOINT', fromOutput: 'endpoint', secret: false },
      { name: 'S3_BUCKET', fromOutput: 'bucket', secret: false },
      { name: 'S3_ACCESS_KEY_ID', fromOutput: 'access_key_id', secret: true },
      { name: 'S3_SECRET_ACCESS_KEY', fromOutput: 'secret_access_key', secret: true }
    ]
  },
  {
    id: 'edge_api_email',
    environmentId: prod,
    from: 'svc_api',
    to: 'svc_email',
    variables: [{ name: 'RESEND_API_KEY', fromOutput: 'api_key', secret: true }]
  },
  {
    id: 'edge_worker_postgres',
    environmentId: prod,
    from: 'svc_worker',
    to: 'svc_postgres',
    variables: [{ name: 'DATABASE_URL', fromOutput: 'connection_url', secret: true }]
  },
  {
    id: 'edge_worker_email',
    environmentId: prod,
    from: 'svc_worker',
    to: 'svc_email',
    variables: [{ name: 'RESEND_API_KEY', fromOutput: 'api_key', secret: true }]
  },
  {
    id: 'edge_stg_web_api',
    environmentId: staging,
    from: 'svc_web_stg',
    to: 'svc_api_stg',
    variables: [{ name: 'API_URL', fromOutput: 'url', secret: false }]
  },
  {
    id: 'edge_stg_api_pg',
    environmentId: staging,
    from: 'svc_api_stg',
    to: 'svc_postgres_stg',
    variables: [{ name: 'DATABASE_URL', fromOutput: 'connection_url', secret: true }]
  },
  {
    id: 'edge_admin_db',
    environmentId: 'env_internal_production',
    from: 'svc_admin',
    to: 'svc_support_db',
    variables: [{ name: 'DATABASE_URL', fromOutput: 'connection_url', secret: true }]
  },
  {
    id: 'edge_ingest_events',
    environmentId: 'env_pipeline_production',
    from: 'svc_ingest',
    to: 'svc_events',
    variables: [{ name: 'QUEUE_URL', fromOutput: 'url', secret: true }]
  },
  {
    id: 'edge_events_wh',
    environmentId: 'env_pipeline_production',
    from: 'svc_ingest',
    to: 'svc_warehouse',
    variables: [{ name: 'DATABASE_URL', fromOutput: 'connection_url', secret: true }]
  },
  {
    id: 'edge_billing',
    environmentId: 'env_billing_production',
    from: 'svc_billing_api',
    to: 'svc_billing_db',
    variables: [{ name: 'DATABASE_URL', fromOutput: 'connection_url', secret: true }]
  }
];

const commits = [
  ['9f3a1c2', 'Cache product listing for 60 seconds', 'maya'],
  ['41be7d0', 'Bump runtime to Node 22.12', 'dev'],
  ['c07d9aa', 'Fix pagination off-by-one on /orders', 'maya'],
  ['5ae2290', 'Add structured request logging', 'sam'],
  ['d81f6b3', 'Revert "Switch image resizer"', 'dev']
] as const;

const people: Record<string, string> = {
  maya: 'Maya Chen',
  dev: 'Dev Patel',
  sam: 'Sam Okafor'
};

function series(seed: number, base: number, amp: number): number[] {
  const out: number[] = [];
  for (let i = 0; i < 24; i++) {
    const v = base + Math.sin(i / 3 + seed) * amp + Math.cos(i * 1.7 + seed * 2) * amp * 0.35;
    out.push(Math.max(0, Math.round(v * 10) / 10));
  }
  return out;
}

/** Deployments, logs and metrics for any service. Deterministic per service id. */
export function serviceDetail(serviceId: string): ServiceDetail {
  const seed = [...serviceId].reduce((a, c) => a + c.charCodeAt(0), 0) % 7;
  const failed = serviceId.includes('warehouse');
  return {
    deployments: commits.map(([sha, message, who], i) => ({
      id: `dpl_${serviceId}_${i}`,
      serviceId,
      status: failed && i === 0 ? 'failed' : i === 4 ? 'rolled_back' : 'succeeded',
      commit: sha,
      message,
      by: people[who] ?? who,
      at: ago(35 + i * 340 + seed * 11),
      durationSeconds: 48 + ((seed * 13 + i * 29) % 90)
    })),
    logs: [
      ['info', 'listening on :8080'],
      ['info', 'GET /healthz 200 1ms'],
      ['info', 'GET /products?limit=24 200 112ms'],
      ['info', 'GET /products/812 200 38ms'],
      ['warn', 'slow query: 640ms select * from orders where user_id = $1'],
      ['info', 'POST /cart 201 54ms'],
      ['info', 'job email.receipt enqueued'],
      ['error', 'upstream timeout after 5000ms (retrying, attempt 1 of 3)'],
      ['info', 'retry succeeded'],
      ['info', 'GET /orders?page=2 200 91ms']
    ].map(([level, message], i) => ({
      at: ago(10 - i),
      level: level as 'info' | 'warn' | 'error',
      message: message as string
    })),
    metrics: [
      { label: 'CPU', unit: '%', points: series(seed, 22, 9) },
      { label: 'Memory', unit: 'MB', points: series(seed + 1, 310, 40) },
      { label: 'Requests', unit: 'req/min', points: series(seed + 2, 180, 60) },
      { label: 'p95 latency', unit: 'ms', points: series(seed + 3, 140, 45) }
    ]
  };
}
