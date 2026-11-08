import type { Component } from 'svelte';
import Box from '@lucide/svelte/icons/box';
import Clock from '@lucide/svelte/icons/clock';
import Cog from '@lucide/svelte/icons/cog';
import Container from '@lucide/svelte/icons/container';
import Database from '@lucide/svelte/icons/database';
import FileText from '@lucide/svelte/icons/file-text';
import GitBranch from '@lucide/svelte/icons/git-branch';
import Globe from '@lucide/svelte/icons/globe';
import HardDrive from '@lucide/svelte/icons/hard-drive';
import KeyRound from '@lucide/svelte/icons/key-round';
import LayoutTemplate from '@lucide/svelte/icons/layout-template';
import ListOrdered from '@lucide/svelte/icons/list-ordered';
import Mail from '@lucide/svelte/icons/mail';
import MemoryStick from '@lucide/svelte/icons/memory-stick';
import Network from '@lucide/svelte/icons/network';
import Search from '@lucide/svelte/icons/search';
import Server from '@lucide/svelte/icons/server';
import SquareDashed from '@lucide/svelte/icons/square-dashed';
import Archive from '@lucide/svelte/icons/archive';
import Zap from '@lucide/svelte/icons/zap';

type Icon = Component<{ class?: string }>;

const ICONS: Record<string, Icon> = {
  web: Globe,
  worker: Cog,
  cron: Clock,
  function: Zap,
  static: FileText,
  postgres: Database,
  mysql: Database,
  sqlite: Database,
  mongodb: Database,
  redis: MemoryStick,
  kv: MemoryStick,
  bucket: Archive,
  queue: ListOrdered,
  search: Search,
  email: Mail,
  dns: Network,
  cdn: Network,
  domain: Network,
  vm: Server,
  observability: Box,
  auth: KeyRound,
  repository: GitBranch,
  registry: Container,
  volume: HardDrive,
  image: Container,
  template: LayoutTemplate
};

export function kindIcon(kind: string): Icon {
  return ICONS[kind] ?? SquareDashed;
}

const LABELS: Record<string, string> = {
  web: 'Web service',
  worker: 'Worker',
  cron: 'Cron job',
  function: 'Function',
  static: 'Static site',
  postgres: 'Postgres',
  mysql: 'MySQL',
  sqlite: 'SQLite',
  mongodb: 'MongoDB',
  redis: 'Redis',
  kv: 'Key-value store',
  bucket: 'Bucket',
  queue: 'Queue',
  search: 'Search',
  email: 'Email',
  dns: 'DNS',
  cdn: 'CDN',
  domain: 'Domain',
  vm: 'Virtual machine',
  observability: 'Observability',
  auth: 'Auth',
  repository: 'Repository',
  registry: 'Registry',
  volume: 'Volume',
  image: 'Docker image'
};

export function kindLabel(kind: string): string {
  return LABELS[kind] ?? kind;
}

export interface CreateOption {
  id: string;
  label: string;
  hint: string;
  icon: Icon;
  /** Catalog kinds that can run this option. Empty means any provider that runs web services. */
  kinds: string[];
  /** The kind given to the draft node. */
  nodeKind: string;
  /** Whether the Where? step asks for a provider. Template has none. */
  needsProvider: boolean;
}

/** What the command palette offers, with the mapping to catalog kinds. */
export const CREATE_OPTIONS: CreateOption[] = [
  {
    id: 'github',
    label: 'GitHub Repository',
    hint: 'Deploy from a repository',
    icon: GitBranch,
    kinds: ['web', 'worker'],
    nodeKind: 'web',
    needsProvider: true
  },
  {
    id: 'database',
    label: 'Database',
    hint: 'Postgres, MySQL and more',
    icon: Database,
    kinds: ['postgres', 'mysql', 'sqlite', 'mongodb', 'redis', 'kv'],
    nodeKind: 'postgres',
    needsProvider: true
  },
  {
    id: 'bucket',
    label: 'Bucket',
    hint: 'Object storage',
    icon: Archive,
    kinds: ['bucket'],
    nodeKind: 'bucket',
    needsProvider: true
  },
  {
    id: 'email',
    label: 'Email',
    hint: 'Transactional email',
    icon: Mail,
    kinds: ['email'],
    nodeKind: 'email',
    needsProvider: true
  },
  {
    id: 'queue',
    label: 'Queue',
    hint: 'Messaging and jobs',
    icon: ListOrdered,
    kinds: ['queue'],
    nodeKind: 'queue',
    needsProvider: true
  },
  {
    id: 'search',
    label: 'Search',
    hint: 'Full-text search',
    icon: Search,
    kinds: ['search'],
    nodeKind: 'search',
    needsProvider: true
  },
  {
    id: 'template',
    label: 'Template',
    hint: 'Start from a known stack',
    icon: LayoutTemplate,
    kinds: ['web'],
    nodeKind: 'web',
    needsProvider: true
  },
  {
    id: 'docker',
    label: 'Docker Image',
    hint: 'Run a container image',
    icon: Container,
    kinds: ['web', 'worker', 'vm'],
    nodeKind: 'web',
    needsProvider: true
  },
  {
    id: 'function',
    label: 'Function',
    hint: 'Serverless function',
    icon: Zap,
    kinds: ['function'],
    nodeKind: 'function',
    needsProvider: true
  },
  {
    id: 'volume',
    label: 'Volume',
    hint: 'Persistent disk',
    icon: HardDrive,
    kinds: ['vm', 'volume'],
    nodeKind: 'volume',
    needsProvider: true
  },
  {
    id: 'empty',
    label: 'Empty Service',
    hint: 'Configure from scratch',
    icon: SquareDashed,
    kinds: ['web', 'worker'],
    nodeKind: 'web',
    needsProvider: true
  }
];

/** Rough monthly price, in cents, used to estimate a draft node in the Plan. */
export function estimateMonthlyCents(kind: string): number {
  switch (kind) {
    case 'postgres':
    case 'mysql':
      return 2500;
    case 'redis':
    case 'kv':
    case 'queue':
      return 1000;
    case 'bucket':
      return 600;
    case 'email':
      return 2000;
    case 'search':
      return 2900;
    case 'vm':
      return 1200;
    case 'volume':
      return 500;
    case 'function':
    case 'static':
      return 0;
    default:
      return 500;
  }
}
