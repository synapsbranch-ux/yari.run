import { agent, ci, dev, maya, sam } from './people';
import { ago } from './time';
import type { AuditEvent, PrincipalRef, Scope } from './types';

const org: Scope = { type: 'organization', id: 'org_acme', name: 'Acme' };
const prod: Scope = { type: 'environment', id: 'env_storefront_production', name: 'Storefront / Production' };
const stg: Scope = { type: 'environment', id: 'env_storefront_staging', name: 'Storefront / Staging' };
const internal: Scope = { type: 'environment', id: 'env_internal_production', name: 'Internal tools / Production' };

let n = 100;
function ev(
  minutes: number,
  principal: PrincipalRef,
  action: string,
  scope: Scope,
  target: string,
  extra: Partial<AuditEvent> = {}
): AuditEvent {
  return { id: n--, organizationId: 'org_acme', occurredAt: ago(minutes), principal, action, scope, target, ...extra };
}

export const auditEvents: AuditEvent[] = [
  ev(2, agent, 'plan.created', prod, 'plan_cache', { planId: 'plan_cache', agentSessionId: 'sess_cache' }),
  ev(3, agent, 'secret.read.denied', prod, 'DATABASE_URL', { agentSessionId: 'sess_cache' }),
  ev(14, maya, 'agent.session.started', prod, 'sess_cache', { agentSessionId: 'sess_cache' }),
  ev(95, dev, 'plan.created', internal, 'plan_cleanup', { planId: 'plan_cleanup' }),
  ev(180, ci, 'plan.applied', stg, 'plan_stg_142'),
  ev(240, sam, 'connection.created', org, 'Backblaze B2'),
  ev(60 * 5, maya, 'grant.created', org, 'Sam Okafor as developer'),
  ev(60 * 26, maya, 'plan.created', { type: 'environment', id: 'env_pipeline_production', name: 'Data pipeline / Production' }, 'plan_pipeline', { planId: 'plan_pipeline' }),
  ev(60 * 30, dev, 'environment.updated', prod, 'require_second_approver enabled'),
  ev(60 * 24 * 2, ci, 'plan.applied', stg, 'plan_stg_139'),
  ev(60 * 24 * 9, dev, 'plan.approved', prod, 'plan_prev', { planId: 'plan_prev' }),
  ev(60 * 24 * 9 + 5, maya, 'plan.applied', prod, 'plan_prev', { planId: 'plan_prev' }),
  ev(60 * 24 * 12, maya, 'project.created', org, 'Storefront'),
  ev(60 * 24 * 14, maya, 'organization.created', org, 'Acme')
];
