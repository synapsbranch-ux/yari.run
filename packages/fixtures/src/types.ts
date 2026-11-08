/**
 * View types for the yari.run clients.
 *
 * These mirror the shapes of the Connect API but are plain TypeScript, independent of
 * protobuf. Timestamps are ISO 8601 strings, money is integer cents (USD) and enums are
 * lowercase string unions. The real-data path of a client maps API responses into these types.
 */

export type Iso = string;

export interface Organization {
  id: string;
  slug: string;
  name: string;
}

export interface Workspace {
  id: string;
  organizationId: string;
  slug: string;
  name: string;
}

export interface Project {
  id: string;
  organizationId: string;
  workspaceId: string;
  slug: string;
  name: string;
  description?: string;
}

export interface Environment {
  id: string;
  organizationId: string;
  projectId: string;
  slug: string;
  name: string;
  /** Protected environments gate secret access and plan applies. */
  protected: boolean;
  /** A plan needs approval from a second principal with plan.approve. */
  requireSecondApprover: boolean;
}

export type ServiceStatus = 'online' | 'sleeping' | 'deploying' | 'failed' | 'draft';

/** A deployable unit of a Project with its state in one Environment. */
export interface Service {
  id: string;
  environmentId: string;
  name: string;
  /** Service kind, for example web, worker, postgres, bucket, email. */
  kind: string;
  /** Provider Catalog id. */
  provider: string;
  status: ServiceStatus;
  domain?: string;
  region?: string;
  /** Suggested canvas position. Clients lay out services that have none. */
  position?: { x: number; y: number };
  monthlyCostCents?: number;
}

/** One variable that an Edge injects into its consumer. */
export interface InjectedVariable {
  name: string;
  /** Output of the dependency that provides the value, for example "url". */
  fromOutput: string;
  secret: boolean;
}

/** Consumer to dependency: `from` consumes what `to` provides. */
export interface Edge {
  id: string;
  environmentId: string;
  from: string;
  to: string;
  variables: InjectedVariable[];
}

export interface Deployment {
  id: string;
  serviceId: string;
  status: 'succeeded' | 'failed' | 'in_progress' | 'rolled_back';
  commit: string;
  message: string;
  by: string;
  at: Iso;
  durationSeconds: number;
}

export interface LogLine {
  at: Iso;
  level: 'info' | 'warn' | 'error';
  message: string;
}

export interface MetricSeries {
  label: string;
  unit: string;
  points: number[];
}

export interface ServiceDetail {
  deployments: Deployment[];
  logs: LogLine[];
  metrics: MetricSeries[];
}

// Plans

export type PlanStatus =
  | 'draft'
  | 'pending_approval'
  | 'approved'
  | 'applying'
  | 'applied'
  | 'failed'
  | 'cancelled';

export type Risk = 'low' | 'medium' | 'high';
export type ChangeAction = 'create' | 'update' | 'delete' | 'noop';

export interface PlanChange {
  service: string;
  kind: string;
  provider: string;
  action: ChangeAction;
  risk: Risk;
  costDeltaCents: number;
  description: string;
}

export type PrincipalKind = 'human' | 'api_key' | 'ci' | 'agent';

export interface PrincipalRef {
  id: string;
  name: string;
  kind: PrincipalKind;
}

export interface Plan {
  id: string;
  organizationId: string;
  environmentId: string;
  /** Display names, resolved for convenience. */
  organizationSlug: string;
  workspaceSlug: string;
  projectName: string;
  projectSlug: string;
  environmentName: string;
  environmentSlug: string;
  environmentProtected: boolean;
  status: PlanStatus;
  proposedBy: PrincipalRef;
  agentSessionId?: string;
  summary: string;
  changes: PlanChange[];
  costDeltaCents: number;
  risk: Risk;
  approvedBy?: string;
  createdAt: Iso;
}

// Access

export type Role = 'viewer' | 'developer' | 'admin' | 'owner';
export type ScopeType = 'organization' | 'workspace' | 'project' | 'environment';

export interface Scope {
  type: ScopeType;
  id: string;
  name: string;
}

export interface Member {
  id: string;
  name: string;
  email?: string;
  kind: PrincipalKind;
  team?: string;
}

export interface Grant {
  id: string;
  principalId: string;
  role: Role;
  scope: Scope;
}

export interface RoleDefinition {
  role: Role;
  summary: string;
  permissions: string[];
}

// Audit

export interface AuditEvent {
  id: number;
  organizationId: string;
  occurredAt: Iso;
  principal: PrincipalRef;
  action: string;
  scope: Scope;
  target: string;
  planId?: string;
  agentSessionId?: string;
}

// Provider Catalog

export interface ProviderInfo {
  id: string;
  name: string;
  category: string;
  kinds: string[];
  auth: string[];
  adapter: string;
  status: string;
  docs: string;
}

// Agent

export type SessionStatus =
  | 'queued'
  | 'running'
  | 'waiting_approval'
  | 'succeeded'
  | 'failed'
  | 'stopped';

export type StepKind = 'message' | 'action' | 'observation' | 'error' | 'guardrail' | 'status';
export type StepLocation = 'sandbox' | 'yari' | 'none';

export interface AgentStep {
  seq: number;
  kind: StepKind;
  /** For message steps: who wrote it. */
  author?: 'agent' | 'user';
  tool?: string;
  location: StepLocation;
  summary: string;
  planId?: string;
  at: Iso;
}

export interface SandboxInfo {
  id: string;
  status: 'starting' | 'running' | 'stopped';
  cpu: number;
  memoryGb: number;
  network: 'none' | 'allowlist';
}

export interface AgentSession {
  id: string;
  environmentId: string;
  /** Display names for lists that span environments. */
  environmentLabel: string;
  goal: string;
  runtime: string;
  model: string;
  status: SessionStatus;
  sandbox: SandboxInfo;
  budgetUsdMicros: number;
  spendUsdMicros: number;
  startedAt: Iso;
}

export interface ModelInfo {
  alias: string;
  displayName: string;
  provider: string;
}

export interface UsageSummary {
  period: string;
  totalCents: number;
  rows: { label: string; cents: number }[];
}
