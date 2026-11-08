import * as pb from '@yari/api-client';
import { createClient, type YariClient } from '@yari/api-client';
import * as fx from '@yari/fixtures';
import { session } from '../session.svelte';
import type {
  AgentSession,
  AgentStep,
  AuditEvent,
  ChangeAction,
  DataSource,
  Environment,
  HierarchyContext,
  Organization,
  Plan,
  PlanChange,
  PlanStatus,
  PrincipalRef,
  Project,
  ProjectSummary,
  Risk,
  ScopeType,
  Service,
  ServiceStatus,
  SessionStatus,
  StepKind,
  StepLocation,
  Workspace
} from './types';

// Client

let cached: { url: string; client: YariClient } | undefined;

/** One client per API URL. Auth headers are read at call time, never stored. */
function api(): YariClient {
  if (!cached || cached.url !== session.apiUrl) {
    cached = { url: session.apiUrl, client: createClient(session.apiUrl, session.headers) };
  }
  return cached.client;
}

// Mapping helpers: protobuf shapes to view types.

function iso(ts?: { seconds: bigint; nanos: number }): string {
  if (!ts) return new Date(0).toISOString();
  return new Date(Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1e6)).toISOString();
}

const planStatus: Record<number, PlanStatus> = {
  [pb.PlanStatus.DRAFT]: 'draft',
  [pb.PlanStatus.PENDING_APPROVAL]: 'pending_approval',
  [pb.PlanStatus.APPROVED]: 'approved',
  [pb.PlanStatus.APPLYING]: 'applying',
  [pb.PlanStatus.APPLIED]: 'applied',
  [pb.PlanStatus.FAILED]: 'failed',
  [pb.PlanStatus.CANCELLED]: 'cancelled'
};

const riskOut: Record<number, Risk> = {
  [pb.Risk.LOW]: 'low',
  [pb.Risk.MEDIUM]: 'medium',
  [pb.Risk.HIGH]: 'high'
};

const riskIn: Record<Risk, pb.Risk> = {
  low: pb.Risk.LOW,
  medium: pb.Risk.MEDIUM,
  high: pb.Risk.HIGH
};

const actionOut: Record<number, ChangeAction> = {
  [pb.ChangeAction.CREATE]: 'create',
  [pb.ChangeAction.UPDATE]: 'update',
  [pb.ChangeAction.DELETE]: 'delete',
  [pb.ChangeAction.NOOP]: 'noop'
};

const actionIn: Record<ChangeAction, pb.ChangeAction> = {
  create: pb.ChangeAction.CREATE,
  update: pb.ChangeAction.UPDATE,
  delete: pb.ChangeAction.DELETE,
  noop: pb.ChangeAction.NOOP
};

const sessionStatus: Record<number, SessionStatus> = {
  [pb.SessionStatus.QUEUED]: 'queued',
  [pb.SessionStatus.RUNNING]: 'running',
  [pb.SessionStatus.WAITING_APPROVAL]: 'waiting_approval',
  [pb.SessionStatus.SUCCEEDED]: 'succeeded',
  [pb.SessionStatus.FAILED]: 'failed',
  [pb.SessionStatus.STOPPED]: 'stopped'
};

const stepKind: Record<number, StepKind> = {
  [pb.StepKind.MESSAGE]: 'message',
  [pb.StepKind.ACTION]: 'action',
  [pb.StepKind.OBSERVATION]: 'observation',
  [pb.StepKind.ERROR]: 'error',
  [pb.StepKind.GUARDRAIL]: 'guardrail',
  [pb.StepKind.STATUS]: 'status'
};

const stepLocation: Record<number, StepLocation> = {
  [pb.StepLocation.SANDBOX]: 'sandbox',
  [pb.StepLocation.YARI]: 'yari',
  [pb.StepLocation.NONE]: 'none'
};

const scopeType: Record<number, ScopeType> = {
  [pb.ScopeType.ORGANIZATION]: 'organization',
  [pb.ScopeType.WORKSPACE]: 'workspace',
  [pb.ScopeType.PROJECT]: 'project',
  [pb.ScopeType.ENVIRONMENT]: 'environment'
};

function serviceStatus(raw: string, provider: string): ServiceStatus {
  const v = raw.toLowerCase();
  if (['online', 'running', 'healthy', 'active'].includes(v)) return 'online';
  if (['sleeping', 'idle', 'stopped', 'paused'].includes(v)) return 'sleeping';
  if (['deploying', 'building', 'applying', 'pending'].includes(v)) return 'deploying';
  if (['failed', 'error', 'crashed'].includes(v)) return 'failed';
  return provider ? 'online' : 'draft';
}

const mapOrg = (o: pb.Organization): Organization => ({ id: o.id, slug: o.slug, name: o.name });
const mapWorkspace = (w: pb.Workspace): Workspace => ({
  id: w.id,
  organizationId: w.organizationId,
  slug: w.slug,
  name: w.name
});
const mapProject = (p: pb.Project): Project => ({
  id: p.id,
  organizationId: p.organizationId,
  workspaceId: p.workspaceId,
  slug: p.slug,
  name: p.name
});
const mapEnv = (e: pb.Environment): Environment => ({
  id: e.id,
  organizationId: e.organizationId,
  projectId: e.projectId,
  slug: e.slug,
  name: e.name,
  protected: e.protected,
  requireSecondApprover: e.requireSecondApprover
});
const mapService = (s: pb.Service, environmentId: string): Service => ({
  id: s.id,
  environmentId,
  name: s.name,
  kind: s.kind,
  provider: s.provider,
  status: serviceStatus(s.status, s.provider)
});

function mapChange(c: pb.PlanChange): PlanChange {
  return {
    service: c.service,
    kind: c.after?.kind ?? '',
    provider: c.after?.provider ?? '',
    action: actionOut[c.action] ?? 'noop',
    risk: riskOut[c.risk] ?? 'low',
    costDeltaCents: Number(c.costDeltaCents),
    description: c.description
  };
}

function mapSession(s: pb.AgentSession, label: string): AgentSession {
  return {
    id: s.id,
    environmentId: s.environmentId,
    environmentLabel: label,
    goal: s.goal,
    runtime: s.runtime,
    model: s.model,
    status: sessionStatus[s.status] ?? 'queued',
    sandbox: {
      id: s.sandboxId,
      status: s.sandboxId ? (s.endedAt ? 'stopped' : 'running') : 'starting',
      // The API does not expose sandbox limits yet. These are the configured defaults.
      cpu: 2,
      memoryGb: 4,
      network: 'none'
    },
    budgetUsdMicros: Number(s.budgetUsdMicros),
    spendUsdMicros: Number(s.spendUsdMicros),
    startedAt: iso(s.startedAt ?? s.createdAt)
  };
}

function mapStep(s: pb.AgentStep): AgentStep {
  return {
    seq: s.seq,
    kind: stepKind[s.kind] ?? 'status',
    tool: s.tool || undefined,
    location: stepLocation[s.location] ?? 'none',
    summary: s.summary,
    planId: s.planId || undefined,
    at: iso(s.createdAt)
  };
}

// Hierarchy index: resolves ids to names for plans and sessions. Cached briefly.

interface HierarchyIndex {
  orgs: Organization[];
  workspaces: Workspace[];
  projects: Project[];
  environments: Environment[];
}

let indexCache: { at: number; value: Promise<HierarchyIndex> } | undefined;

function hierarchyIndex(): Promise<HierarchyIndex> {
  if (indexCache && Date.now() - indexCache.at < 20_000) return indexCache.value;
  const value = (async () => {
    const c = api();
    const orgs = (await c.hierarchy.listOrganizations({})).organizations.map(mapOrg);
    const workspaces = (
      await Promise.all(orgs.map((o) => c.hierarchy.listWorkspaces({ organizationId: o.id })))
    ).flatMap((r) => r.workspaces.map(mapWorkspace));
    const projects = (
      await Promise.all(workspaces.map((w) => c.hierarchy.listProjects({ workspaceId: w.id })))
    ).flatMap((r) => r.projects.map(mapProject));
    const environments = (
      await Promise.all(projects.map((p) => c.hierarchy.listEnvironments({ projectId: p.id })))
    ).flatMap((r) => r.environments.map(mapEnv));
    return { orgs, workspaces, projects, environments };
  })();
  indexCache = { at: Date.now(), value };
  value.catch(() => {
    if (indexCache?.value === value) indexCache = undefined;
  });
  return value;
}

function principal(id: string, agentSession?: string): PrincipalRef {
  if (agentSession) return { id, name: 'yari agent', kind: 'agent' };
  return { id, name: id ? `${id.slice(0, 8)}` : 'unknown', kind: 'human' };
}

function mapPlan(p: pb.Plan, idx: HierarchyIndex): Plan {
  const env = idx.environments.find((e) => e.id === p.environmentId);
  const project = idx.projects.find((x) => x.id === env?.projectId);
  const workspace = idx.workspaces.find((w) => w.id === project?.workspaceId);
  const org = idx.orgs.find((o) => o.id === p.organizationId);
  const changes = p.changes.map(mapChange);
  return {
    id: p.id,
    organizationId: p.organizationId,
    organizationSlug: org?.slug ?? '',
    workspaceSlug: workspace?.slug ?? '',
    environmentId: p.environmentId,
    projectName: project?.name ?? 'Unknown project',
    projectSlug: project?.slug ?? '',
    environmentName: env?.name ?? 'Unknown environment',
    environmentSlug: env?.slug ?? '',
    environmentProtected: env?.protected ?? false,
    status: planStatus[p.status] ?? 'draft',
    proposedBy: principal(p.createdBy, p.agentSessionId),
    agentSessionId: p.agentSessionId || undefined,
    summary: changes.length === 1 ? changes[0].description : `${changes.length} changes`,
    changes,
    costDeltaCents: Number(p.costDeltaCents),
    risk: riskOut[p.risk] ?? 'low',
    approvedBy: p.approvedBy || undefined,
    createdAt: iso(p.createdAt)
  };
}

function bySlug<T extends { slug: string }>(items: T[], slug: string, what: string): T {
  return (
    items.find((i) => i.slug === slug) ??
    (() => {
      throw new Error(`${what} "${slug}" not found`);
    })()
  );
}

export const real: DataSource = {
  kind: 'api',

  async listOrganizations() {
    return (await api().hierarchy.listOrganizations({})).organizations.map(mapOrg);
  },
  async listWorkspaces(organizationId) {
    return (await api().hierarchy.listWorkspaces({ organizationId })).workspaces.map(mapWorkspace);
  },
  async listProjects(workspaceId) {
    return (await api().hierarchy.listProjects({ workspaceId })).projects.map(mapProject);
  },
  async listProjectSummaries(organizationId): Promise<ProjectSummary[]> {
    const c = api();
    const workspaces = (await c.hierarchy.listWorkspaces({ organizationId })).workspaces.map(
      mapWorkspace
    );
    const perWorkspace = await Promise.all(
      workspaces.map(async (workspace) => {
        const projects = (await c.hierarchy.listProjects({ workspaceId: workspace.id })).projects;
        return Promise.all(
          projects.map(async (p): Promise<ProjectSummary> => {
            const envs = (await c.hierarchy.listEnvironments({ projectId: p.id })).environments;
            const env = envs.find((e) => e.slug === 'production') ?? envs[0];
            const services = env
              ? (await c.hierarchy.listServices({ environmentId: env.id })).services
              : [];
            return {
              project: mapProject(p),
              workspace,
              environment: env ? mapEnv(env) : null,
              services: services.map((s) => ({
                id: s.id,
                name: s.name,
                kind: s.kind,
                provider: s.provider,
                status: serviceStatus(s.status, s.provider)
              }))
            };
          })
        );
      })
    );
    return perWorkspace.flat();
  },
  async listEnvironments(projectId) {
    return (await api().hierarchy.listEnvironments({ projectId })).environments.map(mapEnv);
  },
  async resolveHierarchy({ org, ws, project, env }): Promise<HierarchyContext> {
    const c = api();
    const organizations = (await c.hierarchy.listOrganizations({})).organizations.map(mapOrg);
    const organization = bySlug(organizations, org, 'Organization');
    const workspaces = (
      await c.hierarchy.listWorkspaces({ organizationId: organization.id })
    ).workspaces.map(mapWorkspace);
    const workspace = bySlug(workspaces, ws, 'Workspace');
    const projects = (await c.hierarchy.listProjects({ workspaceId: workspace.id })).projects.map(
      mapProject
    );
    const proj = bySlug(projects, project, 'Project');
    const environments = (
      await c.hierarchy.listEnvironments({ projectId: proj.id })
    ).environments.map(mapEnv);
    const environment = bySlug(environments, env, 'Environment');
    return {
      organizations,
      organization,
      workspaces,
      workspace,
      projects,
      project: proj,
      environments,
      environment
    };
  },

  async listServices(environmentId) {
    const res = await api().hierarchy.listServices({ environmentId });
    return res.services.map((s) => mapService(s, environmentId));
  },
  // TODO(api): edges, deployments, logs and metrics have no API yet.
  async listEdges() {
    return [];
  },
  async getServiceDetail() {
    return { deployments: [], logs: [], metrics: [] };
  },

  async listCatalog() {
    const res = await api().catalog.listProviders({});
    return res.providers.map((p) => ({
      id: p.id,
      name: p.name,
      category: p.category,
      kinds: p.kinds,
      auth: p.authMethods,
      adapter: p.adapterFamily,
      status: p.status,
      docs: p.docsUrl
    }));
  },

  async listPendingApprovals() {
    const [res, idx] = await Promise.all([api().plan.listPendingApprovals({}), hierarchyIndex()]);
    return res.plans.map((p) => mapPlan(p, idx));
  },
  async approvePlan(planId) {
    await api().plan.approvePlan({ planId });
    indexCache = undefined;
  },
  async rejectPlan(planId) {
    await api().plan.cancelPlan({ planId });
  },
  async getPlan(planId) {
    const [res, idx] = await Promise.all([api().plan.getPlan({ planId }), hierarchyIndex()]);
    return res.plan ? mapPlan(res.plan, idx) : null;
  },
  // TODO(api): staged (not yet proposed) changes live in the client for now.
  async getStagedPlan() {
    return null;
  },
  async submitPlan(environmentId, changes) {
    const c = api();
    const created = await c.plan.createPlan({
      environmentId,
      changes: changes.map((ch) => ({
        service: ch.service,
        action: actionIn[ch.action],
        risk: riskIn[ch.risk],
        costDeltaCents: BigInt(ch.costDeltaCents),
        description: ch.description,
        after: { name: ch.service, kind: ch.kind, provider: ch.provider }
      }))
    });
    const planId = created.plan?.id ?? '';
    const applied = await c.plan.applyPlan({ planId });
    const idx = await hierarchyIndex();
    const plan = mapPlan(applied.plan ?? created.plan!, idx);
    return { plan, requiresApproval: applied.requiresApproval };
  },

  async listAuditEvents(organizationId) {
    const res = await api().audit.listAuditEvents({ organizationId, pageSize: 100 });
    return res.events.map((e): AuditEvent => ({
      id: Number(e.id),
      organizationId: e.organizationId,
      occurredAt: iso(e.occurredAt),
      principal: principal(e.principalId, e.agentSessionId),
      action: e.action,
      scope: {
        type: scopeType[e.scope?.type ?? 0] ?? 'organization',
        id: e.scope?.id ?? '',
        name: e.scope?.id ?? ''
      },
      target: e.target,
      planId: e.planId || undefined,
      agentSessionId: e.agentSessionId || undefined
    }));
  },
  // TODO(api): no member listing yet. Grants are real, built-in roles are static.
  async listMembers() {
    return [];
  },
  async listGrants(organizationId) {
    const res = await api().access.listGrants({ organizationId });
    const role: Record<number, fx.Role> = {
      [pb.Role.VIEWER]: 'viewer',
      [pb.Role.DEVELOPER]: 'developer',
      [pb.Role.ADMIN]: 'admin',
      [pb.Role.OWNER]: 'owner'
    };
    return res.grants.map((g) => ({
      id: g.id,
      principalId: g.principalId,
      role: role[g.role] ?? 'viewer',
      scope: {
        type: scopeType[g.scope?.type ?? 0] ?? 'organization',
        id: g.scope?.id ?? '',
        name: g.scope?.id ?? ''
      }
    }));
  },
  async listRoles() {
    return fx.roles;
  },
  async getUsage() {
    return { period: 'This month', totalCents: 0, rows: [] };
  },

  async listModels() {
    const res = await api().agent.listModels({});
    return res.models.map((m) => ({
      alias: m.alias,
      displayName: m.displayName,
      provider: m.provider
    }));
  },
  async listAgentSessions(environmentId) {
    const [res, idx] = await Promise.all([
      api().agent.listSessions({ environmentId: environmentId ?? '', pageSize: 50 }),
      hierarchyIndex()
    ]);
    return res.sessions.map((s) => mapSession(s, labelFor(s.environmentId, idx)));
  },
  async getAgentSession(sessionId) {
    const [res, idx] = await Promise.all([api().agent.getSession({ sessionId }), hierarchyIndex()]);
    const s = res.session;
    if (!s) throw new Error('Session not found');
    return mapSession(s, labelFor(s.environmentId, idx));
  },
  async startAgentSession(input) {
    const [res, idx] = await Promise.all([api().agent.startSession(input), hierarchyIndex()]);
    const s = res.session;
    if (!s) throw new Error('The API returned no session');
    return mapSession(s, labelFor(s.environmentId, idx));
  },
  async sendAgentMessage(sessionId, message) {
    await api().agent.sendMessage({ sessionId, message });
  },
  async stopAgentSession(sessionId) {
    const [res, idx] = await Promise.all([
      api().agent.stopSession({ sessionId }),
      hierarchyIndex()
    ]);
    const s = res.session;
    if (!s) throw new Error('The API returned no session');
    return mapSession(s, labelFor(s.environmentId, idx));
  },
  async watchAgentSession(sessionId, handlers, signal) {
    try {
      const stream = api().agent.watchSession({ sessionId, afterSeq: 0 }, { signal });
      for await (const msg of stream) {
        if (msg.step) handlers.onStep(mapStep(msg.step));
        if (msg.status !== pb.SessionStatus.UNSPECIFIED) {
          const mapped = sessionStatus[msg.status];
          if (mapped) handlers.onStatus(mapped);
        }
      }
    } catch (err) {
      if (!signal.aborted) handlers.onError?.(err);
    }
  }
};

function labelFor(environmentId: string, idx: HierarchyIndex): string {
  const env = idx.environments.find((e) => e.id === environmentId);
  const project = idx.projects.find((p) => p.id === env?.projectId);
  return env && project ? `${project.name} / ${env.name}` : environmentId;
}
