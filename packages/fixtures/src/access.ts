import type { Grant, Member, RoleDefinition, UsageSummary } from './types';

export const members: Member[] = [
  { id: 'prin_maya', name: 'Maya Chen', email: 'maya@acme.example', kind: 'human', team: 'Platform' },
  { id: 'prin_dev', name: 'Dev Patel', email: 'dev@acme.example', kind: 'human', team: 'Platform' },
  { id: 'prin_sam', name: 'Sam Okafor', email: 'sam@acme.example', kind: 'human', team: 'Growth' },
  { id: 'prin_agent', name: 'yari agent', kind: 'agent' },
  { id: 'prin_ci', name: 'github-actions', kind: 'ci' }
];

export const grants: Grant[] = [
  { id: 'grant_1', principalId: 'prin_maya', role: 'owner', scope: { type: 'organization', id: 'org_acme', name: 'Acme' } },
  { id: 'grant_2', principalId: 'prin_dev', role: 'admin', scope: { type: 'workspace', id: 'ws_platform', name: 'Platform' } },
  { id: 'grant_3', principalId: 'prin_sam', role: 'developer', scope: { type: 'workspace', id: 'ws_growth', name: 'Growth' } },
  { id: 'grant_4', principalId: 'prin_sam', role: 'viewer', scope: { type: 'project', id: 'prj_storefront', name: 'Storefront' } },
  { id: 'grant_5', principalId: 'prin_agent', role: 'developer', scope: { type: 'project', id: 'prj_storefront', name: 'Storefront' } },
  { id: 'grant_6', principalId: 'prin_ci', role: 'developer', scope: { type: 'environment', id: 'env_storefront_staging', name: 'Storefront / Staging' } }
];

const viewer = ['organization.read', 'workspace.read', 'project.read', 'environment.read', 'service.read', 'plan.read', 'logs.read', 'agent.read'];
const developer = ['service.write', 'plan.create', 'connection.use', 'agent.run', 'plan.apply', 'secret.read'];
const admin = ['plan.approve', 'connection.manage', 'member.manage', 'environment.manage', 'audit.read', 'agent.manage'];

export const roles: RoleDefinition[] = [
  { role: 'viewer', summary: 'Read everything, change nothing.', permissions: viewer },
  { role: 'developer', summary: 'Edit services, propose and apply plans, run the agent. Applying to a protected environment needs approval.', permissions: [...viewer, ...developer] },
  { role: 'admin', summary: 'Approve plans, manage connections, members and environments, read the audit log.', permissions: [...viewer, ...developer, ...admin] },
  { role: 'owner', summary: 'Everything, including organization settings.', permissions: [...viewer, ...developer, ...admin, 'organization.manage'] }
];

export const usage: UsageSummary = {
  period: 'This month',
  totalCents: 11300,
  rows: [
    { label: 'Services (monthly estimate)', cents: 10300 },
    { label: 'Agent model spend', cents: 800 },
    { label: 'Sandbox compute', cents: 200 }
  ]
};
