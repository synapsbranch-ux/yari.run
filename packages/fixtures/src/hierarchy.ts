import type { Environment, Organization, Project, Workspace } from './types';

export const organizations: Organization[] = [
  { id: 'org_acme', slug: 'acme', name: 'Acme' },
  { id: 'org_northwind', slug: 'northwind', name: 'Northwind Labs' }
];

export const workspaces: Workspace[] = [
  { id: 'ws_platform', organizationId: 'org_acme', slug: 'platform', name: 'Platform' },
  { id: 'ws_growth', organizationId: 'org_acme', slug: 'growth', name: 'Growth' },
  { id: 'ws_core', organizationId: 'org_northwind', slug: 'core', name: 'Core' }
];

export const projects: Project[] = [
  {
    id: 'prj_storefront',
    organizationId: 'org_acme',
    workspaceId: 'ws_platform',
    slug: 'storefront',
    name: 'Storefront',
    description: 'Customer web app, API and background jobs.'
  },
  {
    id: 'prj_internal',
    organizationId: 'org_acme',
    workspaceId: 'ws_platform',
    slug: 'internal-tools',
    name: 'Internal tools',
    description: 'Admin dashboard and support tooling.'
  },
  {
    id: 'prj_pipeline',
    organizationId: 'org_acme',
    workspaceId: 'ws_growth',
    slug: 'data-pipeline',
    name: 'Data pipeline',
    description: 'Event ingestion and nightly exports.'
  },
  {
    id: 'prj_landing',
    organizationId: 'org_acme',
    workspaceId: 'ws_growth',
    slug: 'landing',
    name: 'Landing site',
    description: 'Marketing site with transactional email.'
  },
  {
    id: 'prj_billing',
    organizationId: 'org_northwind',
    workspaceId: 'ws_core',
    slug: 'billing',
    name: 'Billing',
    description: 'Invoices and usage metering.'
  }
];

function envs(projectId: string, organizationId: string): Environment[] {
  const base = projectId.replace('prj_', '');
  return [
    {
      id: `env_${base}_production`,
      organizationId,
      projectId,
      slug: 'production',
      name: 'Production',
      protected: true,
      requireSecondApprover: base === 'storefront'
    },
    {
      id: `env_${base}_staging`,
      organizationId,
      projectId,
      slug: 'staging',
      name: 'Staging',
      protected: false,
      requireSecondApprover: false
    }
  ];
}

export const environments: Environment[] = projects.flatMap((p) => envs(p.id, p.organizationId));
