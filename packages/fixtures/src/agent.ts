import { ago } from './time';
import type { AgentSession, AgentStep, ModelInfo } from './types';

export const models: ModelInfo[] = [
  { alias: 'claude-sonnet-5-5', displayName: 'Claude Sonnet 5.5', provider: 'Anthropic' },
  { alias: 'claude-haiku-4-5', displayName: 'Claude Haiku 4.5', provider: 'Anthropic' },
  { alias: 'gpt-5', displayName: 'GPT-5', provider: 'OpenAI' }
];

export const defaultModel = 'claude-sonnet-5-5';

const sandbox = { cpu: 2, memoryGb: 4, network: 'none' as const };

export const agentSessions: AgentSession[] = [
  {
    id: 'sess_cache',
    environmentId: 'env_storefront_production',
    environmentLabel: 'Storefront / Production',
    goal: 'Add a Redis cache for the API and cut p95 latency on GET /products.',
    runtime: 'fake',
    model: 'claude-sonnet-5-5',
    status: 'waiting_approval',
    sandbox: { id: 'sbx_8c1f', status: 'running', ...sandbox },
    budgetUsdMicros: 2_000_000,
    spendUsdMicros: 420_000,
    startedAt: ago(14)
  },
  {
    id: 'sess_logs',
    environmentId: 'env_storefront_staging',
    environmentLabel: 'Storefront / Staging',
    goal: 'Find why the staging api deploy keeps restarting.',
    runtime: 'fake',
    model: 'claude-haiku-4-5',
    status: 'succeeded',
    sandbox: { id: 'sbx_2a90', status: 'stopped', ...sandbox },
    budgetUsdMicros: 2_000_000,
    spendUsdMicros: 130_000,
    startedAt: ago(60 * 20)
  },
  {
    id: 'sess_cost',
    environmentId: 'env_internal_production',
    environmentLabel: 'Internal tools / Production',
    goal: 'Propose ways to reduce the monthly bill of internal tools.',
    runtime: 'fake',
    model: 'claude-sonnet-5-5',
    status: 'stopped',
    sandbox: { id: 'sbx_77de', status: 'stopped', ...sandbox },
    budgetUsdMicros: 2_000_000,
    spendUsdMicros: 910_000,
    startedAt: ago(60 * 49)
  }
];

type Draft = Omit<AgentStep, 'seq' | 'at'> & { secondsAgo: number };

function step(secondsAgo: number, s: Omit<AgentStep, 'seq' | 'at'>): Draft {
  return { secondsAgo, ...s };
}

/** Assigns seq numbers (starting at 1) and timestamps. */
function numbered(drafts: Draft[]): AgentStep[] {
  return drafts.map(({ secondsAgo, ...rest }, i) => ({ seq: i + 1, at: ago(secondsAgo / 60), ...rest }));
}

/** The rich fixture session: messages, actions in the Sandbox and in yari, one refusal, one plan. */
export const cacheSessionSteps: AgentStep[] = numbered([
  step(840, { kind: 'status', location: 'none', summary: 'Sandbox started: 2 CPU, 4 GB, no internet. Repository mounted read-write at /workspace.' }),
  step(835, { kind: 'message', author: 'agent', location: 'none', summary: 'I will look at the current topology and the slow endpoint first, then decide whether a cache is the right fix.' }),
  step(820, { kind: 'action', tool: 'yari.get_environment', location: 'yari', summary: 'Read environment Storefront / Production (6 services, 6 edges).' }),
  step(818, { kind: 'observation', location: 'yari', summary: 'api runs on Railway, postgres on Supabase. The worker is sleeping. No cache service exists.' }),
  step(790, { kind: 'action', tool: 'read_file', location: 'sandbox', summary: 'Read /workspace/api/src/routes/products.ts' }),
  step(788, { kind: 'observation', location: 'sandbox', summary: 'GET /products runs 3 queries per request and has no caching layer.' }),
  step(760, { kind: 'action', tool: 'yari.get_logs', location: 'yari', summary: 'Fetched the last 200 log lines for api.' }),
  step(756, { kind: 'observation', location: 'yari', summary: 'p95 for GET /products is 840 ms. About 61% of that time is spent in postgres.' }),
  step(720, { kind: 'action', tool: 'yari.read_secret', location: 'yari', summary: 'Read secret DATABASE_URL of api.' }),
  step(719, { kind: 'guardrail', tool: 'yari.read_secret', location: 'yari', summary: 'Refused: agents can never read secrets (secret.read). Use an injected reference instead.' }),
  step(700, { kind: 'message', author: 'agent', location: 'none', summary: 'Understood. I do not need the value. I will reference the cache by output name and let yari inject it.' }),
  step(640, { kind: 'action', tool: 'edit_file', location: 'sandbox', summary: 'Added a cache-aside layer to /workspace/api/src/routes/products.ts and src/cache.ts.' }),
  step(580, { kind: 'action', tool: 'bash', location: 'sandbox', summary: 'pnpm test --filter api: 14 passed, 0 failed.' }),
  step(520, { kind: 'action', tool: 'yari.create_plan', location: 'yari', summary: 'Created a plan with 3 changes: create cache (Upstash Redis), update api, update worker.', planId: 'plan_cache' }),
  step(515, { kind: 'status', location: 'none', summary: 'Waiting for approval. Production requires approval from an admin.' })
]);

export const logsSessionSteps: AgentStep[] = numbered([
  step(72000, { kind: 'status', location: 'none', summary: 'Sandbox started: 2 CPU, 4 GB, no internet.' }),
  step(71990, { kind: 'action', tool: 'yari.get_logs', location: 'yari', summary: 'Fetched the last 500 log lines for api (staging).' }),
  step(71980, { kind: 'observation', location: 'yari', summary: 'The process exits with code 137 about 40 seconds after start. Out of memory.' }),
  step(71900, { kind: 'message', author: 'agent', location: 'none', summary: 'The staging api is limited to 256 MB but needs about 340 MB at boot. Raising the limit to 512 MB fixes the restarts.' }),
  step(71850, { kind: 'status', location: 'none', summary: 'Session finished.' })
]);

export const costSessionSteps: AgentStep[] = numbered([
  step(176000, { kind: 'status', location: 'none', summary: 'Sandbox started: 2 CPU, 4 GB, no internet.' }),
  step(175900, { kind: 'action', tool: 'yari.get_environment', location: 'yari', summary: 'Read environment Internal tools / Production.' }),
  step(175800, { kind: 'observation', location: 'yari', summary: 'support-db uses 4 GB but peaks below 1 GB. The digest cron has failed 9 runs in a row.' }),
  step(175700, { kind: 'status', location: 'none', summary: 'Stopped by Maya Chen.' })
]);

export const agentSteps: Record<string, AgentStep[]> = {
  sess_cache: cacheSessionSteps,
  sess_logs: logsSessionSteps,
  sess_cost: costSessionSteps
};
