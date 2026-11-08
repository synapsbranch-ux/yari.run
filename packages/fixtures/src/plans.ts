import { agent, dev, maya } from './people';
import { ago } from './time';
import type { Plan, PlanChange } from './types';

/** Agent-proposed plan on production. Pending approval. */
export const agentChanges: PlanChange[] = [
  {
    service: 'cache',
    kind: 'redis',
    provider: 'upstash',
    action: 'create',
    risk: 'low',
    costDeltaCents: 1000,
    description: 'Create a Redis cache (pay as you go, 256 MB) in us-east-1.'
  },
  {
    service: 'api',
    kind: 'web',
    provider: 'railway',
    action: 'update',
    risk: 'medium',
    costDeltaCents: 1400,
    description: 'Scale from 1 to 2 replicas and inject CACHE_URL from cache.'
  },
  {
    service: 'worker',
    kind: 'worker',
    provider: 'railway',
    action: 'update',
    risk: 'low',
    costDeltaCents: 0,
    description: 'Inject CACHE_URL from cache. No restart of running jobs.'
  }
];

export const plans: Plan[] = [
  {
    id: 'plan_cache',
    organizationId: 'org_acme',
    organizationSlug: 'acme',
    workspaceSlug: 'platform',
    environmentId: 'env_storefront_production',
    projectName: 'Storefront',
    projectSlug: 'storefront',
    environmentName: 'Production',
    environmentSlug: 'production',
    environmentProtected: true,
    status: 'pending_approval',
    proposedBy: agent,
    agentSessionId: 'sess_cache',
    summary: 'Add a Redis cache in front of GET /products to cut p95 latency.',
    changes: agentChanges,
    costDeltaCents: 2400,
    risk: 'medium',
    createdAt: ago(7)
  },
  {
    id: 'plan_cleanup',
    organizationId: 'org_acme',
    organizationSlug: 'acme',
    workspaceSlug: 'platform',
    environmentId: 'env_internal_production',
    projectName: 'Internal tools',
    projectSlug: 'internal-tools',
    environmentName: 'Production',
    environmentSlug: 'production',
    environmentProtected: true,
    status: 'pending_approval',
    proposedBy: dev,
    summary: 'Replace the support database with a smaller instance and drop the unused digest cron.',
    changes: [
      {
        service: 'support-db',
        kind: 'postgres',
        provider: 'supabase',
        action: 'update',
        risk: 'high',
        costDeltaCents: -1500,
        description: 'Resize from 4 GB to 2 GB memory. Causes about 30 seconds of downtime.'
      },
      {
        service: 'digest',
        kind: 'cron',
        provider: 'railway',
        action: 'delete',
        risk: 'medium',
        costDeltaCents: -300,
        description: 'Delete the weekly digest cron. It has not run successfully since September.'
      }
    ],
    costDeltaCents: -1800,
    risk: 'high',
    createdAt: ago(95)
  },
  {
    id: 'plan_pipeline',
    organizationId: 'org_acme',
    organizationSlug: 'acme',
    workspaceSlug: 'growth',
    environmentId: 'env_pipeline_production',
    projectName: 'Data pipeline',
    projectSlug: 'data-pipeline',
    environmentName: 'Production',
    environmentSlug: 'production',
    environmentProtected: true,
    status: 'pending_approval',
    proposedBy: maya,
    summary: 'Point ingest at a new warehouse database.',
    changes: [
      {
        service: 'warehouse',
        kind: 'postgres',
        provider: 'supabase',
        action: 'create',
        risk: 'low',
        costDeltaCents: 2500,
        description: 'Create a new Postgres database (2 GB, us-east-1).'
      },
      {
        service: 'ingest',
        kind: 'web',
        provider: 'railway',
        action: 'update',
        risk: 'low',
        costDeltaCents: 0,
        description: 'Switch DATABASE_URL to the new warehouse.'
      }
    ],
    costDeltaCents: 2500,
    risk: 'low',
    createdAt: ago(60 * 26)
  },
  {
    id: 'plan_prev',
    organizationId: 'org_acme',
    organizationSlug: 'acme',
    workspaceSlug: 'platform',
    environmentId: 'env_storefront_production',
    projectName: 'Storefront',
    projectSlug: 'storefront',
    environmentName: 'Production',
    environmentSlug: 'production',
    environmentProtected: true,
    status: 'applied',
    proposedBy: maya,
    summary: 'Add transactional email.',
    changes: [
      {
        service: 'email',
        kind: 'email',
        provider: 'resend',
        action: 'create',
        risk: 'low',
        costDeltaCents: 2000,
        description: 'Create a Resend sending domain for mail.acme.example.'
      }
    ],
    costDeltaCents: 2000,
    risk: 'low',
    approvedBy: 'Dev Patel',
    createdAt: ago(60 * 24 * 9)
  }
];

/** A plan of local, not yet proposed changes shown by Deploy when the canvas has no drafts. */
export const stagedChanges: PlanChange[] = [
  {
    service: 'api',
    kind: 'web',
    provider: 'railway',
    action: 'update',
    risk: 'low',
    costDeltaCents: 0,
    description: 'Set LOG_LEVEL to info.'
  },
  {
    service: 'worker',
    kind: 'worker',
    provider: 'railway',
    action: 'update',
    risk: 'medium',
    costDeltaCents: 800,
    description: 'Wake the worker and raise concurrency from 2 to 4.'
  }
];
