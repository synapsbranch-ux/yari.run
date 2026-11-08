import * as fx from '@yari/fixtures';
import type {
  AgentSession,
  AgentStep,
  DataSource,
  HierarchyContext,
  Plan,
  PlanChange,
  ProjectSummary,
  Risk,
  SessionStatus,
  StartSessionInput,
  WatchHandlers
} from './types';

const LATENCY_MS = 120;

function wait<T>(value: T, ms = LATENCY_MS): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(structuredClone(value)), ms));
}

function notFound(what: string): never {
  throw new Error(`${what} not found`);
}

// Mutable demo state, reset on reload.

const plans: Plan[] = structuredClone(fx.plans);
const sessions: AgentSession[] = structuredClone(fx.agentSessions);
let planCounter = 200;
let sessionCounter = 0;

// Agent replay engine.

const STEP_DELAY: Record<string, number> = {
  status: 700,
  message: 1500,
  action: 1100,
  observation: 1000,
  guardrail: 1300,
  error: 1000
};

/** One simulated agent session. Steps are revealed over time to the watchers. */
class FakeRun {
  revealed: AgentStep[] = [];
  private pending: AgentStep[];
  private listeners = new Set<WatchHandlers>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private started = false;

  constructor(
    readonly session: AgentSession,
    steps: AgentStep[],
    private readonly replay: boolean,
    private readonly finalStatus: SessionStatus
  ) {
    this.pending = replay ? structuredClone(steps) : [];
    this.revealed = replay ? [] : structuredClone(steps);
  }

  subscribe(h: WatchHandlers): () => void {
    for (const s of this.revealed) h.onStep(structuredClone(s));
    h.onStatus(this.session.status);
    this.listeners.add(h);
    if (!this.started) {
      this.started = true;
      if (this.pending.length > 0) this.schedule();
    }
    return () => this.listeners.delete(h);
  }

  private schedule() {
    const next = this.pending[0];
    if (!next) return;
    this.timer = setTimeout(() => this.tick(), STEP_DELAY[next.kind] ?? 1000);
  }

  private tick() {
    const step = this.pending.shift();
    if (!step) return;
    this.reveal(step);
    const total = this.revealed.length + this.pending.length;
    this.session.spendUsdMicros = Math.round(
      (this.targetSpend * this.revealed.length) / Math.max(total, 1)
    );
    if (this.pending.length > 0) this.schedule();
    else this.setStatus(this.finalStatus);
  }

  private get targetSpend(): number {
    return fx.agentSessions.find((s) => s.id === this.session.id)?.spendUsdMicros ?? 180_000;
  }

  private reveal(step: AgentStep) {
    this.revealed.push(step);
    for (const l of this.listeners) l.onStep(structuredClone(step));
  }

  private setStatus(status: SessionStatus) {
    this.session.status = status;
    if (status === 'stopped' || status === 'succeeded' || status === 'failed') {
      this.session.sandbox.status = 'stopped';
    }
    for (const l of this.listeners) l.onStatus(status);
  }

  say(message: string) {
    const now = new Date().toISOString();
    this.reveal({
      seq: this.revealed.length + this.pending.length + 1,
      kind: 'message',
      author: 'user',
      location: 'none',
      summary: message,
      at: now
    });
    this.setStatus('running');
    const reply: AgentStep = {
      seq: this.revealed.length + 1,
      kind: 'message',
      author: 'agent',
      location: 'none',
      summary:
        'Noted. I will take that into account. Any change to production still goes through a plan that an admin has to approve.',
      at: now
    };
    setTimeout(() => {
      this.reveal({ ...reply, seq: this.revealed.length + 1, at: new Date().toISOString() });
      this.setStatus(this.pending.length > 0 ? 'running' : 'waiting_approval');
    }, 1500);
  }

  stop() {
    if (this.timer) clearTimeout(this.timer);
    this.pending = [];
    this.reveal({
      seq: this.revealed.length + 1,
      kind: 'status',
      location: 'none',
      summary: 'Stopped by you. The Sandbox was shut down.',
      at: new Date().toISOString()
    });
    this.session.sandbox.status = 'stopped';
    this.setStatus('stopped');
  }
}

const runs = new Map<string, FakeRun>();

function runFor(sessionId: string): FakeRun {
  const existing = runs.get(sessionId);
  if (existing) return existing;
  const session = sessions.find((s) => s.id === sessionId) ?? notFound('Session');
  const steps = fx.agentSteps[sessionId] ?? [];
  // The newest session is replayed live. Finished sessions are shown at once.
  const live = session.status === 'waiting_approval' || session.status === 'running';
  const run = new FakeRun(session, steps, live, session.status);
  if (live) session.status = 'running';
  runs.set(sessionId, run);
  return run;
}

function scriptFor(goal: string, startedAt: string): AgentStep[] {
  const t = startedAt;
  const step = (n: number, s: Omit<AgentStep, 'seq' | 'at'>): AgentStep => ({
    seq: n,
    at: t,
    ...s
  });
  return [
    step(1, {
      kind: 'status',
      location: 'none',
      summary: 'Sandbox started: 2 CPU, 4 GB, no internet.'
    }),
    step(2, {
      kind: 'message',
      author: 'agent',
      location: 'none',
      summary: `Goal received: "${goal}". I will inspect the environment first.`
    }),
    step(3, {
      kind: 'action',
      tool: 'yari.get_environment',
      location: 'yari',
      summary: 'Read the current environment, its services and edges.'
    }),
    step(4, {
      kind: 'observation',
      location: 'yari',
      summary: '6 services, 5 online, 1 sleeping. No failed deployments in the last 24 hours.'
    }),
    step(5, {
      kind: 'action',
      tool: 'bash',
      location: 'sandbox',
      summary: 'grep -rn "TODO" /workspace | head -20'
    }),
    step(6, {
      kind: 'observation',
      location: 'sandbox',
      summary: '3 TODO comments found, none related to the goal.'
    }),
    step(7, {
      kind: 'message',
      author: 'agent',
      location: 'none',
      summary: 'I found nothing that needs a change right now. No plan was proposed.'
    }),
    step(8, { kind: 'status', location: 'none', summary: 'Session finished.' })
  ];
}

const live = fx.services; // services are static in the demo

function summaryFor(project: fx.Project): ProjectSummary {
  const workspace =
    fx.workspaces.find((w) => w.id === project.workspaceId) ?? notFound('Workspace');
  const environment =
    fx.environments.find((e) => e.projectId === project.id && e.slug === 'production') ?? null;
  const services = environment
    ? live
        .filter((s) => s.environmentId === environment.id)
        .map((s) => ({
          id: s.id,
          name: s.name,
          kind: s.kind,
          provider: s.provider,
          status: s.status
        }))
    : [];
  return { project, workspace, environment, services };
}

function costOf(changes: PlanChange[]): number {
  return changes.reduce((sum, c) => sum + c.costDeltaCents, 0);
}

function riskOf(changes: PlanChange[]): Risk {
  if (changes.some((c) => c.risk === 'high')) return 'high';
  if (changes.some((c) => c.risk === 'medium')) return 'medium';
  return 'low';
}

export const fake: DataSource = {
  kind: 'fake',

  listOrganizations: () => wait(fx.organizations),
  listWorkspaces: (organizationId) =>
    wait(fx.workspaces.filter((w) => w.organizationId === organizationId)),
  listProjects: (workspaceId) => wait(fx.projects.filter((p) => p.workspaceId === workspaceId)),
  listProjectSummaries: (organizationId) =>
    wait(fx.projects.filter((p) => p.organizationId === organizationId).map(summaryFor)),
  listEnvironments: (projectId) => wait(fx.environments.filter((e) => e.projectId === projectId)),

  async resolveHierarchy({ org, ws, project, env }): Promise<HierarchyContext> {
    const organization = fx.organizations.find((o) => o.slug === org) ?? notFound('Organization');
    const workspaces = fx.workspaces.filter((w) => w.organizationId === organization.id);
    const workspace = workspaces.find((w) => w.slug === ws) ?? notFound('Workspace');
    const projects = fx.projects.filter((p) => p.workspaceId === workspace.id);
    const proj = projects.find((p) => p.slug === project) ?? notFound('Project');
    const environments = fx.environments.filter((e) => e.projectId === proj.id);
    const environment = environments.find((e) => e.slug === env) ?? notFound('Environment');
    return wait({
      organizations: fx.organizations,
      organization,
      workspaces,
      workspace,
      projects,
      project: proj,
      environments,
      environment
    });
  },

  listServices: (environmentId) => wait(live.filter((s) => s.environmentId === environmentId)),
  listEdges: (environmentId) => wait(fx.edges.filter((e) => e.environmentId === environmentId)),
  getServiceDetail: (serviceId) => wait(fx.serviceDetail(serviceId)),

  listCatalog: () => wait(fx.catalog),

  listPendingApprovals: () => wait(plans.filter((p) => p.status === 'pending_approval')),
  async approvePlan(planId) {
    const plan = plans.find((p) => p.id === planId) ?? notFound('Plan');
    plan.status = 'approved';
    plan.approvedBy = 'You';
    await wait(null);
  },
  async rejectPlan(planId) {
    const plan = plans.find((p) => p.id === planId) ?? notFound('Plan');
    plan.status = 'cancelled';
    await wait(null);
  },
  getPlan: (planId) => wait(plans.find((p) => p.id === planId) ?? null),
  async getStagedPlan(environmentId) {
    if (environmentId !== 'env_storefront_production') return wait(null);
    const env = fx.environments.find((e) => e.id === environmentId) ?? notFound('Environment');
    return wait<Plan>({
      id: 'plan_staged',
      organizationId: env.organizationId,
      organizationSlug: 'acme',
      workspaceSlug: 'platform',
      environmentId,
      projectName: 'Storefront',
      projectSlug: 'storefront',
      environmentName: env.name,
      environmentSlug: env.slug,
      environmentProtected: env.protected,
      status: 'draft',
      proposedBy: fx.maya,
      summary: 'Local changes',
      changes: fx.stagedChanges,
      costDeltaCents: costOf(fx.stagedChanges),
      risk: riskOf(fx.stagedChanges),
      createdAt: new Date().toISOString()
    });
  },
  async submitPlan(environmentId, changes) {
    const env = fx.environments.find((e) => e.id === environmentId) ?? notFound('Environment');
    const project = fx.projects.find((p) => p.id === env.projectId) ?? notFound('Project');
    const workspace =
      fx.workspaces.find((w) => w.id === project.workspaceId) ?? notFound('Workspace');
    const requiresApproval = env.protected;
    const plan: Plan = {
      id: `plan_${++planCounter}`,
      organizationId: env.organizationId,
      organizationSlug: fx.organizations.find((o) => o.id === env.organizationId)?.slug ?? '',
      workspaceSlug: workspace.slug,
      environmentId,
      projectName: project.name,
      projectSlug: project.slug,
      environmentName: env.name,
      environmentSlug: env.slug,
      environmentProtected: env.protected,
      status: requiresApproval ? 'pending_approval' : 'applying',
      proposedBy: fx.maya,
      summary: 'Changes from the canvas',
      changes,
      costDeltaCents: costOf(changes),
      risk: riskOf(changes),
      createdAt: new Date().toISOString()
    };
    plans.unshift(plan);
    return wait({ plan, requiresApproval });
  },

  listAuditEvents: (organizationId) =>
    wait(fx.auditEvents.filter((e) => e.organizationId === organizationId)),
  listMembers: () => wait(fx.members),
  listGrants: () => wait(fx.grants),
  listRoles: () => wait(fx.roles),
  getUsage: () => wait(fx.usage),

  listModels: () => wait(fx.models),
  listAgentSessions: (environmentId) =>
    wait(sessions.filter((s) => !environmentId || s.environmentId === environmentId)),
  async getAgentSession(sessionId) {
    const s = sessions.find((x) => x.id === sessionId) ?? notFound('Session');
    return wait(s, 20);
  },
  async startAgentSession(input: StartSessionInput) {
    const env =
      fx.environments.find((e) => e.id === input.environmentId) ?? notFound('Environment');
    const project = fx.projects.find((p) => p.id === env.projectId);
    const now = new Date().toISOString();
    const id = `sess_new_${++sessionCounter}`;
    const session: AgentSession = {
      id,
      environmentId: env.id,
      environmentLabel: `${project?.name ?? ''} / ${env.name}`,
      goal: input.goal,
      runtime: 'fake',
      model: input.model,
      status: 'running',
      sandbox: { id: `sbx_${id}`, status: 'running', cpu: 2, memoryGb: 4, network: 'none' },
      budgetUsdMicros: 2_000_000,
      spendUsdMicros: 0,
      startedAt: now
    };
    sessions.unshift(session);
    fx.agentSteps[id] = scriptFor(input.goal, now);
    runs.set(id, new FakeRun(session, fx.agentSteps[id], true, 'succeeded'));
    return wait(session);
  },
  async sendAgentMessage(sessionId, message) {
    runFor(sessionId).say(message);
    await wait(null, 30);
  },
  async stopAgentSession(sessionId) {
    const run = runFor(sessionId);
    run.stop();
    return wait(run.session, 30);
  },
  watchAgentSession(sessionId, handlers, signal) {
    return new Promise<void>((resolve) => {
      const unsubscribe = runFor(sessionId).subscribe(handlers);
      const done = () => {
        unsubscribe();
        resolve();
      };
      if (signal.aborted) done();
      else signal.addEventListener('abort', done, { once: true });
    });
  }
};
