export type {
  AgentSession,
  AgentStep,
  AuditEvent,
  ChangeAction,
  Deployment,
  Edge,
  Environment,
  Grant,
  InjectedVariable,
  LogLine,
  Member,
  MetricSeries,
  ModelInfo,
  Organization,
  Plan,
  PlanChange,
  PlanStatus,
  PrincipalKind,
  PrincipalRef,
  Project,
  ProviderInfo,
  Risk,
  Role,
  RoleDefinition,
  SandboxInfo,
  Scope,
  ScopeType,
  Service,
  ServiceDetail,
  ServiceStatus,
  SessionStatus,
  StepKind,
  StepLocation,
  UsageSummary,
  Workspace
} from '@yari/fixtures';

import type {
  AgentSession,
  AgentStep,
  AuditEvent,
  Edge,
  Environment,
  Grant,
  Member,
  ModelInfo,
  Organization,
  Plan,
  PlanChange,
  Project,
  ProviderInfo,
  RoleDefinition,
  Service,
  ServiceDetail,
  SessionStatus,
  UsageSummary,
  Workspace
} from '@yari/fixtures';

/** The path from the URL resolved to real records, plus the siblings for the breadcrumb menus. */
export interface HierarchyContext {
  organizations: Organization[];
  organization: Organization;
  workspaces: Workspace[];
  workspace: Workspace;
  projects: Project[];
  project: Project;
  environments: Environment[];
  environment: Environment;
}

/** A project card on the Organization home. */
export interface ProjectSummary {
  project: Project;
  workspace: Workspace;
  /** The environment the card reports on (production when it exists). */
  environment: Environment | null;
  services: { id: string; name: string; kind: string; provider: string; status: string }[];
}

export interface WatchHandlers {
  onStep: (step: AgentStep) => void;
  onStatus: (status: SessionStatus) => void;
  onError?: (error: unknown) => void;
}

export interface StartSessionInput {
  environmentId: string;
  goal: string;
  model: string;
}

/**
 * Everything the UI needs from the backend. Two implementations: fixtures (`fake.ts`) and the
 * Connect API (`real.ts`). Components never import either directly, only `data` from index.ts.
 */
export interface DataSource {
  readonly kind: 'fake' | 'api';

  listOrganizations(): Promise<Organization[]>;
  listWorkspaces(organizationId: string): Promise<Workspace[]>;
  listProjects(workspaceId: string): Promise<Project[]>;
  /** Projects of every workspace of the organization, with a status summary. */
  listProjectSummaries(organizationId: string): Promise<ProjectSummary[]>;
  listEnvironments(projectId: string): Promise<Environment[]>;
  resolveHierarchy(slugs: {
    org: string;
    ws: string;
    project: string;
    env: string;
  }): Promise<HierarchyContext>;

  listServices(environmentId: string): Promise<Service[]>;
  listEdges(environmentId: string): Promise<Edge[]>;
  getServiceDetail(serviceId: string): Promise<ServiceDetail>;

  listCatalog(): Promise<ProviderInfo[]>;

  listPendingApprovals(): Promise<Plan[]>;
  approvePlan(planId: string): Promise<void>;
  /** Rejecting cancels the plan. */
  rejectPlan(planId: string): Promise<void>;
  getPlan(planId: string): Promise<Plan | null>;
  getStagedPlan(environmentId: string): Promise<Plan | null>;
  /** Creates a plan from the given changes and asks to apply it. Protected environments go to approval. */
  submitPlan(
    environmentId: string,
    changes: PlanChange[]
  ): Promise<{ plan: Plan; requiresApproval: boolean }>;

  listAuditEvents(organizationId: string): Promise<AuditEvent[]>;
  listMembers(organizationId: string): Promise<Member[]>;
  listGrants(organizationId: string): Promise<Grant[]>;
  listRoles(): Promise<RoleDefinition[]>;
  getUsage(organizationId: string): Promise<UsageSummary>;

  listModels(): Promise<ModelInfo[]>;
  listAgentSessions(environmentId?: string): Promise<AgentSession[]>;
  getAgentSession(sessionId: string): Promise<AgentSession>;
  startAgentSession(input: StartSessionInput): Promise<AgentSession>;
  sendAgentMessage(sessionId: string, message: string): Promise<void>;
  stopAgentSession(sessionId: string): Promise<AgentSession>;
  /** Streams the step log of a session until the signal aborts or the stream ends. */
  watchAgentSession(sessionId: string, handlers: WatchHandlers, signal: AbortSignal): Promise<void>;
}
