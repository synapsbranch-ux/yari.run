import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
import { data } from './index';

/** Query keys, grouped so that mutations can invalidate precisely. */
export const keys = {
  organizations: ['organizations'] as const,
  hierarchy: (slugs: object) => ['hierarchy', slugs] as const,
  projectSummaries: (orgId: string) => ['project-summaries', orgId] as const,
  services: (envId: string) => ['services', envId] as const,
  edges: (envId: string) => ['edges', envId] as const,
  serviceDetail: (serviceId: string) => ['service-detail', serviceId] as const,
  catalog: ['catalog'] as const,
  approvals: ['approvals'] as const,
  plan: (id: string) => ['plan', id] as const,
  stagedPlan: (envId: string) => ['staged-plan', envId] as const,
  audit: (orgId: string) => ['audit', orgId] as const,
  members: (orgId: string) => ['members', orgId] as const,
  grants: (orgId: string) => ['grants', orgId] as const,
  roles: ['roles'] as const,
  usage: (orgId: string) => ['usage', orgId] as const,
  models: ['models'] as const,
  sessions: (envId: string | undefined) => ['sessions', envId ?? 'all'] as const,
  session: (id: string) => ['session', id] as const
};

type Slugs = { org: string; ws: string; project: string; env: string };

export const organizationsQuery = () =>
  createQuery(() => ({ queryKey: keys.organizations, queryFn: () => data.listOrganizations() }));

export const hierarchyQuery = (slugs: () => Slugs) =>
  createQuery(() => ({
    queryKey: keys.hierarchy(slugs()),
    queryFn: () => data.resolveHierarchy(slugs()),
    retry: false
  }));

export const projectSummariesQuery = (orgId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.projectSummaries(orgId() ?? ''),
    queryFn: () => data.listProjectSummaries(orgId()!),
    enabled: !!orgId()
  }));

export const servicesQuery = (envId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.services(envId() ?? ''),
    queryFn: () => data.listServices(envId()!),
    enabled: !!envId()
  }));

export const edgesQuery = (envId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.edges(envId() ?? ''),
    queryFn: () => data.listEdges(envId()!),
    enabled: !!envId()
  }));

export const serviceDetailQuery = (serviceId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.serviceDetail(serviceId() ?? ''),
    queryFn: () => data.getServiceDetail(serviceId()!),
    enabled: !!serviceId()
  }));

export const catalogQuery = () =>
  createQuery(() => ({
    queryKey: keys.catalog,
    queryFn: () => data.listCatalog(),
    staleTime: 5 * 60_000
  }));

export const approvalsQuery = () =>
  createQuery(() => ({ queryKey: keys.approvals, queryFn: () => data.listPendingApprovals() }));

export const planQuery = (id: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.plan(id() ?? ''),
    queryFn: () => data.getPlan(id()!),
    enabled: !!id()
  }));

export const stagedPlanQuery = (envId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.stagedPlan(envId() ?? ''),
    queryFn: () => data.getStagedPlan(envId()!),
    enabled: !!envId()
  }));

export const auditQuery = (orgId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.audit(orgId() ?? ''),
    queryFn: () => data.listAuditEvents(orgId()!),
    enabled: !!orgId()
  }));

export const membersQuery = (orgId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.members(orgId() ?? ''),
    queryFn: () => data.listMembers(orgId()!),
    enabled: !!orgId()
  }));

export const grantsQuery = (orgId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.grants(orgId() ?? ''),
    queryFn: () => data.listGrants(orgId()!),
    enabled: !!orgId()
  }));

export const rolesQuery = () =>
  createQuery(() => ({ queryKey: keys.roles, queryFn: () => data.listRoles() }));

export const usageQuery = (orgId: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.usage(orgId() ?? ''),
    queryFn: () => data.getUsage(orgId()!),
    enabled: !!orgId()
  }));

export const modelsQuery = () =>
  createQuery(() => ({
    queryKey: keys.models,
    queryFn: () => data.listModels(),
    staleTime: 5 * 60_000
  }));

export const sessionsQuery = (envId?: () => string | undefined) =>
  createQuery(() => ({
    queryKey: keys.sessions(envId?.()),
    queryFn: () => data.listAgentSessions(envId?.())
  }));

export const sessionQuery = (id: () => string | undefined, poll: () => boolean) =>
  createQuery(() => ({
    queryKey: keys.session(id() ?? ''),
    queryFn: () => data.getAgentSession(id()!),
    enabled: !!id(),
    refetchInterval: poll() ? 2000 : false
  }));

// Mutations

export function approvalMutations() {
  const client = useQueryClient();
  const done = () => client.invalidateQueries({ queryKey: keys.approvals });
  return {
    approve: createMutation(() => ({
      mutationFn: (planId: string) => data.approvePlan(planId),
      onSuccess: done
    })),
    reject: createMutation(() => ({
      mutationFn: (planId: string) => data.rejectPlan(planId),
      onSuccess: done
    }))
  };
}

export function stopSessionMutation() {
  const client = useQueryClient();
  return createMutation(() => ({
    mutationFn: (sessionId: string) => data.stopAgentSession(sessionId),
    onSuccess: (session) => {
      client.invalidateQueries({ queryKey: keys.session(session.id) });
      client.invalidateQueries({ queryKey: ['sessions'] });
    }
  }));
}
