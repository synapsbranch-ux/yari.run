# yari.run

Ship software peacefully, on any cloud.

## What it is

yari.run is a Railway-style control plane that deploys to the clouds you already pay for. It never hosts workloads. You draw services on a canvas, yari.run turns the drawing into a reviewable **Plan**, and once the Plan is approved it drives each provider's native API: PaaS (Railway, Vercel, Render, Fly.io), IaaS clouds (AWS, Google Cloud, Azure, Hetzner) and specialized resource providers (Neon, Backblaze B2, Upstash, Resend). Each Workspace also has an AI DevOps agent that works in its own isolated container and can only act through the same plan-and-approve API as a person.

Three equal pillars:

- **Deploy**: the canvas, the desired-state spec, provider adapters, and Edges that inject one service's outputs into another.
- **Govern**: the Organization > Workspace > Project > Environment hierarchy, roles and grants, approvals, and an append-only audit log.
- **Operate**: logs, metrics, and the agent.

## Status

Pre-alpha scaffold. The Railway, Vercel and Supabase adapters come first; today they are stubs that return `ErrNotImplemented`. Every other provider in the catalog is `planned` and answers `ErrNotSupported` with a link to its API docs. Plans are applied by an in-memory fake provider, so the whole flow runs without any cloud account. No provider API is called anywhere in this repository.

## Architecture

```mermaid
flowchart LR
  subgraph Clients["Clients (one Svelte codebase, three shells)"]
    Web["Web (static SPA)"]
    Desktop["Desktop (Tauri 2)"]
    Mobile["Mobile (Capacitor)"]
  end
  CLI["CLI (yari)"]
  MCPC["External MCP clients"]
  Agent["AI agent (OpenHands)"]

  API["Connect API (cmd/api)"]
  MCP["MCP server (cmd/mcp)"]
  Worker["River worker (cmd/worker)"]
  PG[("PostgreSQL<br/>RLS per Organization")]
  Zitadel["Zitadel (OIDC)"]
  Broker["Credential broker<br/>(future)"]

  Web --> API
  Desktop --> API
  Mobile --> API
  CLI --> API
  MCPC --> MCP
  Agent --> MCP
  MCP --> API
  API --> PG
  API --> Zitadel
  Worker --> PG
  Worker --> Orch["Orchestrator"]
  Orch --> Adapters["Provider adapters"]
  Adapters --> Clouds["Provider APIs"]
  Orch -.-> Broker
  Worker --> SBM["Sandbox manager"]
  SBM --> Box["One container per Agent Session"]
  Agent -.runs in.-> Box
```

Everything goes through one API. The three clients, the CLI, the MCP server and the agent all call the same Connect API, so authentication, authorization, protected environments, approvals and the audit log apply to every actor in exactly one place. No client and no agent tool can call a provider or the database directly.

## Core concepts

```
Organization            tenant boundary (row-level security)
 └─ Workspace           one team or product area, has one agent identity
     └─ Project         a set of Services that belong together
         └─ Environment production, staging, ...
```

| Term | Meaning |
| --- | --- |
| Organization | The tenant. Every tenant table carries `organization_id` and is protected by forced row-level security. |
| Workspace | A group of Projects inside an Organization. Each Workspace has its own agent Principal. |
| Project | A set of Services and the Edges between them. |
| Environment | A deployment target of a Project, such as `production`. `production` is always `protected`. |
| Service | One deployable unit with a kind (`web`, `postgres`, `bucket`, ...), run by a provider from the Provider Catalog. |
| Edge | A dependency between two Services. The consumer receives the dependency's outputs as injected variables. |
| Connection | A stored credential for a provider. Only ciphertext is kept; only the future credential broker may decrypt it. |
| Deployment | One rollout of a Service in an Environment. |
| Plan | A reviewable list of changes with risk and monthly cost delta. Nothing changes without one. |
| Principal | Anyone or anything that acts: a human, an API key, CI, or an agent. |
| Role | `viewer`, `developer`, `admin` or `owner`. |
| Grant | A Role given to a Principal on a scope. Grants carry down the hierarchy, and the highest Role on the chain wins. |
| Agent Session | One run of the DevOps agent, with its own Sandbox, model key, budget and step log. |
| Sandbox | The isolated container an Agent Session runs in. |
| Provider Catalog | The list of clouds yari.run can deploy to, with what each can run and how far its adapter has come. |

## Provider Catalog

The catalog is `internal/provider/catalog/catalog.yaml`, embedded into the binary and validated by tests (unique ids, closed lists of categories, kinds and auth methods, https docs URLs, exactly three `alpha` entries). It is served over the API by `CatalogService.ListProviders`.

Categories: `paas`, `cloud`, `database`, `storage`, `cdn_dns`, `email`, `messaging`, `search`, `observability`, `auth`, `source`.

Status:

- `alpha`: an adapter exists and is being built (Railway, Vercel, Supabase). Today these are stubs.
- `planned`: listed, not built. `Registry.Get` returns `ErrNotSupported` wrapped with the docs URL. It is never a silent no-op.

Adapter family: `native` (one adapter per provider), `s3compat` (one adapter family for S3-compatible storage), `pulumi` (the large clouds), `none`.

### Add a provider

1. Add an entry to `catalog.yaml` and set `docs` to the provider's official API documentation. `go test ./internal/provider/...` validates it. Keep `packages/fixtures/src/catalog.json` identical (a test checks it) and refresh the table below with `make catalog-table`.
2. Add the adapter under `internal/provider/<id>/` implementing `provider.Provider`, and register it in `internal/provider/all`. Until then the entry answers `ErrNotSupported`.
3. Move its `status` to `alpha` only when you also change the "exactly three alpha entries" rule in the catalog validator.

### Catalog

<!-- catalog-table:begin -->
| Provider | Category | Kinds | Auth | Adapter | Status | API docs |
| --- | --- | --- | --- | --- | --- | --- |
| Railway (`railway`) | paas | web, worker, cron, postgres, redis | api_token, oauth | native | alpha | [docs](https://docs.railway.com/integrations/api) |
| Vercel (`vercel`) | paas | web, function, static, cron | api_token | native | alpha | [docs](https://vercel.com/docs/rest-api) |
| Render (`render`) | paas | web, worker, cron, static, postgres, kv | api_key | native | planned | [docs](https://render.com/docs/api) |
| Netlify (`netlify`) | paas | static, function, cron | api_token, oauth | native | planned | [docs](https://docs.netlify.com/api-and-cli-guides/api-guides/get-started-with-api/) |
| Cloudflare (`cloudflare`) | paas | function, static, cron, kv, bucket, sqlite, queue, dns, cdn | api_token | native | planned | [docs](https://developers.cloudflare.com/api/) |
| Fly.io (`fly`) | paas | web, worker, postgres | api_token | native | planned | [docs](https://docs.fly.io/machines/api) |
| Heroku (`heroku`) | paas | web, worker, cron, postgres, redis | oauth, api_token | native | planned | [docs](https://devcenter.heroku.com/articles/platform-api-reference) |
| Koyeb (`koyeb`) | paas | web, worker, postgres | api_token | native | planned | [docs](https://www.koyeb.com/docs/reference/api) |
| Northflank (`northflank`) | paas | web, worker, cron, postgres, mysql, redis, mongodb | api_token | native | planned | [docs](https://northflank.com/docs/v1/api/introduction) |
| Amazon Web Services (`aws`) | cloud | web, worker, cron, function, postgres, mysql, redis, bucket, queue, email, dns, cdn, vm | iam_role, access_key | pulumi | planned | [docs](https://docs.aws.amazon.com/general/latest/gr/aws-apis.html) |
| Google Cloud (`gcp`) | cloud | web, worker, cron, function, postgres, mysql, redis, bucket, queue, dns, cdn, vm | workload_identity, oauth | pulumi | planned | [docs](https://cloud.google.com/apis) |
| Microsoft Azure (`azure`) | cloud | web, worker, cron, function, postgres, mysql, redis, bucket, queue, dns, cdn, vm | workload_identity, oauth | pulumi | planned | [docs](https://learn.microsoft.com/en-us/rest/api/azure/) |
| DigitalOcean (`digitalocean`) | cloud | web, worker, cron, static, postgres, mysql, redis, mongodb, bucket, vm, dns | api_token, oauth, access_key | native | planned | [docs](https://docs.digitalocean.com/reference/api/) |
| Hetzner (`hetzner`) | cloud | vm, bucket, dns | api_token, access_key | native | planned | [docs](https://docs.hetzner.cloud/) |
| Akamai (Linode) (`akamai-linode`) | cloud | vm, postgres, mysql, bucket, dns | api_token, oauth, access_key | native | planned | [docs](https://techdocs.akamai.com/linode-api/reference/api) |
| Vultr (`vultr`) | cloud | vm, postgres, mysql, redis, bucket, dns | api_key, access_key | native | planned | [docs](https://www.vultr.com/api/) |
| Scaleway (`scaleway`) | cloud | web, function, vm, postgres, mysql, redis, bucket, dns | api_key | native | planned | [docs](https://www.scaleway.com/en/developers/api/) |
| OVHcloud (`ovhcloud`) | cloud | vm, postgres, mysql, bucket, dns | oauth, api_key | native | planned | [docs](https://docs.ovhcloud.com/en/guides/manage-and-operate/api/first-steps) |
| Supabase (`supabase`) | database | postgres, bucket, function | oauth, api_token | native | alpha | [docs](https://supabase.com/docs/reference/api/introduction) |
| Neon (`neon`) | database | postgres | api_key, oauth | native | planned | [docs](https://neon.com/docs/reference/api) |
| PlanetScale (`planetscale`) | database | mysql, postgres | api_token, oauth | native | planned | [docs](https://planetscale.com/docs/api/reference/getting-started-with-planetscale-api) |
| Turso (`turso`) | database | sqlite | api_token | native | planned | [docs](https://docs.turso.tech/api-reference/introduction) |
| MongoDB Atlas (`mongodb-atlas`) | database | mongodb | oauth, api_key | native | planned | [docs](https://www.mongodb.com/docs/atlas/api/) |
| Upstash (`upstash`) | database | redis, queue | api_key | native | planned | [docs](https://upstash.com/docs/devops/developer-api/introduction) |
| Aiven (`aiven`) | database | postgres, mysql, redis, queue, search | api_token | native | planned | [docs](https://api.aiven.io/doc/) |
| Redis Cloud (`redis-cloud`) | database | redis | api_key | native | planned | [docs](https://redis.io/docs/latest/operate/rc/api/) |
| Backblaze B2 (`backblaze-b2`) | storage | bucket | access_key | s3compat | planned | [docs](https://www.backblaze.com/docs/cloud-storage-s3-compatible-api) |
| Wasabi (`wasabi`) | storage | bucket | access_key | s3compat | planned | [docs](https://docs.wasabi.com/) |
| Tigris (`tigris`) | storage | bucket | access_key | s3compat | planned | [docs](https://www.tigrisdata.com/docs/) |
| bunny.net (`bunny`) | cdn_dns | cdn, dns, bucket | api_key | native | planned | [docs](https://bunny.net/docs/api-reference/core) |
| Resend (`resend`) | email | email | api_key | native | planned | [docs](https://resend.com/docs/api-reference/introduction) |
| Postmark (`postmark`) | email | email | api_token | native | planned | [docs](https://postmarkapp.com/developer) |
| SendGrid (`sendgrid`) | email | email | api_key | native | planned | [docs](https://www.twilio.com/docs/sendgrid/api-reference) |
| Mailgun (`mailgun`) | email | email | api_key | native | planned | [docs](https://documentation.mailgun.com/docs/mailgun/api-reference/mg-auth) |
| CloudAMQP (`cloudamqp`) | messaging | queue | api_key | native | planned | [docs](https://docs.cloudamqp.com/instance-api.html) |
| Confluent Cloud (`confluent`) | messaging | queue | api_key, oauth | native | planned | [docs](https://docs.confluent.io/cloud/current/api.html) |
| Algolia (`algolia`) | search | search | api_key | native | planned | [docs](https://www.algolia.com/doc/rest-api/search/) |
| Meilisearch Cloud (`meilisearch-cloud`) | search | search | api_key | native | planned | [docs](https://www.meilisearch.com/docs/reference/api/authorization) |
| Typesense Cloud (`typesense-cloud`) | search | search | api_key | native | planned | [docs](https://typesense.org/docs/cloud-management-api/v1/) |
| Elastic Cloud (`elastic-cloud`) | search | search | api_key | native | planned | [docs](https://www.elastic.co/docs/api/doc/cloud/) |
| Grafana Cloud (`grafana-cloud`) | observability | observability | api_token | native | planned | [docs](https://grafana.com/docs/grafana-cloud/learn-and-build/developer-resources/api-reference/cloud-api/) |
| Better Stack (`better-stack`) | observability | observability | api_token | native | planned | [docs](https://betterstack.com/docs/uptime/api/getting-started-with-uptime-api/) |
| Sentry (`sentry`) | observability | observability | api_token, oauth | native | planned | [docs](https://docs.sentry.io/api/) |
| Axiom (`axiom`) | observability | observability | api_token | native | planned | [docs](https://axiom.co/docs/restapi/introduction) |
| Clerk (`clerk`) | auth | auth | api_key | native | planned | [docs](https://clerk.com/docs/reference/backend-api) |
| Auth0 (`auth0`) | auth | auth | oauth | native | planned | [docs](https://auth0.com/docs/api/management/v2) |
| WorkOS (`workos`) | auth | auth | api_key | native | planned | [docs](https://workos.com/docs/reference) |
| ZITADEL Cloud (`zitadel-cloud`) | auth | auth | api_token, oauth | native | planned | [docs](https://zitadel.com/docs/apis/introduction) |
| GitHub (`github`) | source | repository | oauth, api_token | native | planned | [docs](https://docs.github.com/en/rest) |
| GitLab (`gitlab`) | source | repository | oauth, api_token | native | planned | [docs](https://docs.gitlab.com/api/rest/) |
| Bitbucket (`bitbucket`) | source | repository | oauth, api_token | native | planned | [docs](https://developer.atlassian.com/cloud/bitbucket/rest/intro/) |
| GitHub Container Registry (`ghcr`) | source | registry | api_token | native | planned | [docs](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry) |
| Docker Hub (`docker-hub`) | source | registry | api_token | native | planned | [docs](https://docs.docker.com/reference/api/hub/latest/) |
<!-- catalog-table:end -->

Intended capabilities were checked against each provider's current documentation. The corrections made are listed in the pull request description.

## AI agent and sandboxes

Every Agent Session runs the DevOps agent in **its own isolated container**. The harness is the **OpenHands Software Agent SDK**: OpenHands owns planning, tool calling, file editing, shell, context management and the agent loop. yari.run owns everything around it: the hardened sandbox, the model gateway with per-session keys and budgets, the yari tools exposed over MCP, the event log and the approvals. There is no agent loop, no tool-calling loop and no model client in this repository.

```mermaid
flowchart LR
  W["Worker (outside)"]
  subgraph net["yari-agent-net (internal, no internet)"]
    SB["Sandbox container<br/>OpenHands agent server<br/>read-only root, non-root, no capabilities"]
    L["LiteLLM gateway<br/>holds the real model keys"]
    M["yari MCP server"]
  end
  A["Connect API"]
  P["Model providers"]

  W -- "REST + WebSocket (session key)" --> SB
  W -- "admin API (master key)" --> L
  SB -- "session virtual key" --> L
  SB -- "session token" --> M
  M -- "session token" --> A
  L --> P
```

**Integration path (decided in step 6.0): Path A, pure Go.** The agent server image is pinned to `ghcr.io/openhands/agent-server:1.60.1-python` by tag and digest (never `latest`). A Go client is generated with `oapi-codegen` from a trimmed copy of the server's OpenAPI spec (`internal/agent/openhands/client`, regenerate with the recipe in its `doc.go`), plus a WebSocket consumer. The worker creates a conversation (`POST /api/conversations`) with the default tool preset (`terminal`, `file_editor`, `task_tracker`), the yari MCP server with `Authorization: Bearer <session token>`, the LLM pointed at LiteLLM with the session key, and the iteration limit. It sends the goal as the first message with `run: false`, then starts the loop, and streams events from `/sockets/events/{id}`. Follow-ups, pause, resume and stop use `/events`, `/pause`, `/run` and `/interrupt`. The server is protected by `OH_SESSION_API_KEYS_0` and the `X-Session-API-Key` header. Everything verified about the server, with the exact JSON and the hardening matrix, is in [`agent/image/README.md`](agent/image/README.md). No Python runner was needed.

Things worth knowing about the upstream server, all handled:

- It runs as user `openhands` (uid 10001) on port 8000 and needs writable `/tmp` (with exec, it is a bundled binary that unpacks there), `/home/openhands` and `/workspace`.
- The base image gives that user passwordless `sudo` and starts a VS Code server on port 8001 that uses the session key as its token. Our image purges `sudo` and disables the VS Code server.
- An MCP server it cannot reach is skipped silently. The runtime checks that the yari tools appear in the agent's system prompt and ends the session with an error if they do not.

### Sandbox hardening

Created by `internal/sandbox/docker` with the Docker Engine Go SDK (`github.com/moby/moby/client`). All of it is mandatory:

- the `runsc` (gVisor) runtime when the daemon has it. Without it, development falls back to the default runtime with a loud warning; outside `YARI_ENV=dev` the worker refuses to start. **gVisor was not available where this scaffold was built, so that path is untested.**
- a non-root user (an image that would run as root is refused), read-only root filesystem, and tmpfs for `/workspace`, `/tmp` and `/home/openhands`
- `cap-drop ALL`, `no-new-privileges`, not privileged, init process
- CPU, memory (swap equals memory) and pids limits, 2 CPUs, 4 GB and 512 pids by default
- attached **only** to `yari-agent-net`, a Docker network created with `internal: true` (no internet egress, no published ports), or to no network at all
- no host mounts, never the Docker socket, labels `yari.session` and `yari.expires` so the reaper can find expired sandboxes

One item is relaxed, with the reason: **tmpfs mounts allow `exec`** (Docker defaults to `noexec`). The agent server is a bundled binary that unpacks its shared libraries into `/tmp` and cannot start otherwise. `nosuid` and `nodev` stay on. Network isolation, credentials, root and the Docker socket are not relaxed.

The worker reaches the agent server on the sandbox's address inside `yari-agent-net`, which works from a Linux host (the bridge is routable from the host). On Docker Desktop that network is not routable from the host; run the worker as a container attached to `yari-agent-net` there. The sandbox itself gets no route out.

### What enters a sandbox

No provider credential, no real model API key, no LiteLLM master key and no database URL ever enters a sandbox. `sandbox.ValidateEnv` rejects any variable whose name or value looks like a long-lived credential. Exactly three **session-scoped, short-lived, revocable** secrets exist, and all three die when the session ends:

| Secret | Where it lives | Revoked by |
| --- | --- | --- |
| Agent server session key (`ohs_...`) | container env `OH_SESSION_API_KEYS_0`, the only secret allowed there | the container is destroyed |
| LiteLLM virtual key | sent through the agent server API when the conversation is created, never as container env. Restricted to one model, with a budget and an expiry | `Revoke` on the gateway |
| yari API token | sent the same way, as the MCP header. Expires with the session | revoked in the database |

The agent can read its own session key from the container environment; it is useless once the container is gone. The token plaintext travels in the job arguments of the session job and is not returned or logged; River keeps job rows, which is acceptable only because the token is dead by then. All three values and anything shaped like a credential are redacted from events before they reach the step log.

### Budgets, guardrails and the step log

- **Budget**: each session key has `max_budget` (default 2 USD, `YARI_AGENT_SESSION_BUDGET_USD`). The worker watches spend; when it is exhausted it records a `guardrail` step and ends the session as `failed` with reason "budget exhausted".
- **Limits**: an iteration limit and a session time limit end a session as `failed` the same way.
- **Deny list, in code not in prompts**: an agent cannot propose a `delete` on a `postgres`, `mysql`, `mongodb`, `sqlite`, `redis` or `bucket` service. It is refused in the MCP server and again in the API, where the refusal is also written to the audit log.
- **Protected environments**: an agent's `plan.apply` on a protected environment only ever reaches `pending_approval`; it cannot approve plans, read secrets or start sessions. When the last plan of a session is waiting, the session ends as `waiting_approval`.
- **Step log**: every event becomes an `agent_steps` row, written by the worker as the only writer, append-only and hash-chained. `audit.VerifyChain` detects edits, removals and reordering.
- **Cleanup**: the sandbox is always destroyed, the key and the token always revoked, even on error or panic. A scheduled job reaps expired sandboxes and cleans up sessions whose worker died.

### How to switch models

Models are the aliases in [`agent/litellm/config.yaml`](agent/litellm/config.yaml): `claude-sonnet-5-5` and `gpt-6.1-sol` ship enabled, and a commented Ollama example shows a local model. Add an entry, restart the `litellm` container, and the alias appears in the client's model picker (`ListModels` reads that file). Pick it per session in the client or with `yari agent start --model <alias>`, or change the default with `YARI_AGENT_DEFAULT_MODEL`. Provider keys go in `.env` and are read only by the LiteLLM container.

## Repository layout

```
.
├── cmd/            api, worker, migrate, mcp, yari (CLI)
├── internal/       config, db, authn, authz, audit, api, jobs, provider/*, orchestrator,
│                   sandbox/*, gateway, agent/*, mcpserver, plan
├── proto/yari/v1   the API contract (buf)
├── gen/            generated Go code from proto (committed)
├── db/             migrations (goose), queries (sqlc), init/01-roles.sql (local dev only)
├── agent/          image/ (sandbox image), litellm/ (gateway config), prompts/, fixtures/sample-repo
├── apps/client     the only UI codebase (SvelteKit SPA)
├── apps/desktop    Tauri 2 shell around the client build
├── apps/mobile     Capacitor shell around the client build
├── packages/       api-client (generated TypeScript client), fixtures (shared fake data)
├── docker-compose.yml, Makefile, buf.yaml, buf.gen.yaml, sqlc.yaml
```

## Prerequisites

| Tool | Version used | Needed for |
| --- | --- | --- |
| Go | 1.27.3 (the `go` line in `go.mod` downloads it automatically) | everything backend |
| Node.js | 22.23 (22.12 or newer) | client, shells |
| pnpm | 12.18.0 | workspace |
| Docker Engine with Compose | 29.9 | Postgres, sandboxes, LiteLLM, MCP image |
| gVisor (`runsc`) | optional | stronger sandbox isolation. Required outside dev |
| A model provider key | optional | only for the real agent (`openhands` runtime). The `fake` runtime needs none |
| Rust | 1.97 | desktop shell |
| Tauri Linux libraries | `libwebkit2gtk-4.1-dev build-essential curl wget file libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev` | desktop shell on Linux |
| Android SDK (platform 36) and JDK 21, or Xcode | | building the mobile app. Syncing the project needs neither |

Pinned versions of the main pieces, checked against the registries and docs on the day the scaffold was written:

| Area | Versions |
| --- | --- |
| Database and infra | PostgreSQL 18.6, Zitadel v4.20.1, LiteLLM `v1.83.14-stable.patch.3` (`litellm-database`), OpenHands agent server 1.60.1 |
| Go libraries | connect 1.21.1, pgx 5.12.0, River 0.51.1, go-oidc 3.21.0, MCP go-sdk 1.8.0, Docker SDK (`moby/moby/client`) 0.6.2, cobra 1.10, oapi-codegen 2.8.2 |
| Go tools (`tool` directives) | buf 1.75.0, sqlc 1.31.1, goose 3.29.0, protoc-gen-go, protoc-gen-connect-go. buf requires an unreleased `google.golang.org/protobuf` snapshot, which `go.mod` therefore pins |
| TypeScript | Svelte 5.57.3, SvelteKit 3.1.1, adapter-static 4.0.1, Vite 8.3.6, Tailwind 4.3.5, shadcn-svelte 1.8.0, @xyflow/svelte 1.7.1, TanStack Svelte Query 6.4.1, TypeScript 6.0.3, protobuf-es and Connect-ES 2.x |
| Shells | Tauri 2.12.5 (plugin-notification 2.8.0), Capacitor 8.6.0 (local-notifications 8.3.1, `@aparajita/capacitor-secure-storage` 8.0.1) |

Mobile secure storage uses `@aparajita/capacitor-secure-storage` (Android Keystore with AES-GCM, iOS Keychain), chosen over `capacitor-secure-storage-plugin` because it is more recently maintained and typed. Tokens never go to Preferences or localStorage.

## Quickstart

```bash
git clone https://github.com/synapsbranch-ux/yari.run && cd yari.run
make setup          # creates .env from .env.example, downloads Go modules, installs the pnpm workspace
make up migrate gen agent-image
```

`make up` starts Postgres, `make migrate` applies goose then River migrations, `make gen` regenerates Go, TypeScript and SQL code (no diff expected), and `make agent-image` builds the hardened agent image `yari-agent:dev` and the MCP image.

Run the services, each in its own terminal:

```bash
make api            # Connect API on :8080
make worker         # River worker: plan applies and agent sessions
make client         # the web client on http://localhost:5173
```

Open <http://localhost:5173>. The client starts on fake data (`PUBLIC_USE_FAKE_DATA=true`): a canvas, the Provider Catalog, approvals, the agent panel and the mobile routes at `/m`. Set `PUBLIC_USE_FAKE_DATA=false` in `.env` to use the API; in dev builds the Settings page lets you set a dev subject.

`make mcp` runs the MCP server on the host for external MCP clients. Agent sandboxes reach the one in the compose stack instead.

### Try the agent without any model key

The `fake` runtime plays the agent from a script. It still runs in a real hardened container, runs real commands there, and calls the yari MCP server with a real session token.

```bash
docker compose --profile agent up -d --wait    # LiteLLM and the MCP server (no provider key needed)
make demo                                       # org, protected production environment, agent session
```

Or by hand with the CLI:

```bash
export YARI_API_URL=http://localhost:8080 YARI_DEV_SUBJECT=demo
go run ./cmd/yari orgs create acme
go run ./cmd/yari agent start --env <production-environment-id> "Make the sample build pass and deploy it"
go run ./cmd/yari agent watch <session-id>
```

You will see the agent read the spec, watch `make build` fail in the sandbox, fix the Makefile, rerun it, propose a plan and apply it. Because the environment is `production`, the apply only reaches `pending_approval` and the session ends as `waiting_approval`. The agent proposed the plan, so a person can approve it in the client's approvals page (with `PUBLIC_USE_FAKE_DATA=false`).

### Switch to the real agent

1. Put a provider key in `.env` (`ANTHROPIC_API_KEY` or `OPENAI_API_KEY`) and set `LITELLM_MASTER_KEY` to your own value in `.env`.
2. `docker compose --profile agent up -d --force-recreate litellm yari-mcp`
3. Set `YARI_AGENT_RUNTIME=openhands` in `.env` and restart `make worker`.

Optional identity provider: `docker compose --profile auth up -d` starts Zitadel on port 8081 (single container, local only). OIDC verification is implemented in the API but this compose profile has not been exercised end to end in this scaffold.

### Desktop

Install the Tauri Linux libraries above, then:

```bash
make client           # or build the SPA once: pnpm --filter client build
make desktop          # Tauri dev shell, loads the client dev server
pnpm --filter client build && pnpm --filter desktop tauri build
```

The shell has a tray (open, pending approvals, running agent sessions, quit) and a native notification at startup. Its CSP allows only `http://localhost:8080` for the API; change it in `apps/desktop/src-tauri/tauri.conf.json` for another API URL. The counts in the tray are fake for now. See `apps/desktop/README.md`.

### Mobile

```bash
make mobile                                   # builds the client and runs cap sync android
pnpm --filter mobile exec cap open android    # needs Android Studio
```

The app starts on `/m`. A phone cannot use `http://localhost:8080`, so mobile builds need an `https` `PUBLIC_API_URL` and the API's `YARI_CORS_ORIGINS` already allows `https://localhost` (Android) and `capacitor://localhost` (iOS). iOS: on macOS run `pnpm --filter mobile add @capacitor/ios@8.6.0` and `pnpm --filter mobile exec cap add ios`. See `apps/mobile/README.md`.

## Make targets

| Target | What it does |
| --- | --- |
| `make setup` | Create `.env` from `.env.example`, download Go modules, install the pnpm workspace |
| `make up` | Start Postgres and wait until it is healthy |
| `make down` | Stop all local infrastructure (all compose profiles), keep the data volume |
| `make migrate` | Apply goose migrations, then River migrations, as the owner role |
| `make gen` | `buf generate` (Go and TypeScript) and `sqlc generate` |
| `make lint` | `buf lint`, `go vet`, and lint plus type check of the TypeScript workspace |
| `make test` | `go test ./...` (see below for the opt-in database, Docker and OpenHands tests) |
| `make build` | Build the Go binaries into `./bin` and the client SPA |
| `make api` / `make worker` / `make mcp` | Run the API, the worker, the MCP server |
| `make agent-image` | Cross-build the MCP binary, build `yari-agent:dev` and the MCP image |
| `make service-binaries` | Cross-build the Go services that compose images use |
| `make catalog-table` | Print the Provider Catalog as Markdown |
| `make client` | Client dev server on port 5173 |
| `make desktop` | Tauri dev shell |
| `make mobile` | Build the client and sync it into the Capacitor Android project |

Opt-in tests, because they need infrastructure:

| Variable | Enables |
| --- | --- |
| `YARI_TEST_DATABASE_URL` | Database tests: RLS isolation, append-only logs, security definer functions, authorization inheritance, the API, the worker. Use the `yari_app` URL. |
| `YARI_TEST_OWNER_DATABASE_URL` | The owner-role checks of the append-only triggers |
| `YARI_TEST_DOCKER=1` | Hardened sandbox tests and the scripted agent in a real container. Needs `docker compose --profile agent up -d` and `make agent-image` |
| `YARI_TEST_OPENHANDS=1` | The OpenHands runtime against the pinned agent server image |

## Code generation

- `make gen` runs `go tool buf generate`: Go messages and Connect handlers into `gen/` (committed) and TypeScript messages and service descriptors into `packages/api-client/src/gen` with `protoc-gen-es` (Connect-ES v2 needs no separate connect plugin). Run `pnpm install` once first so the plugin binary exists.
- It also runs `go tool sqlc generate`: typed queries from `db/queries` over the goose migrations into `internal/db/dbsqlc`.
- The tools are pinned by `tool` directives in `go.mod`, so there is nothing to install.
- Generated code is committed. After `make gen` there should be no diff.

## Security model

- **Tenancy**: the tenant boundary is the Organization. Every tenant table has a non-null `organization_id` and **enabled and forced** row-level security keyed on a transaction-local setting (`yari.organization_id`). Composite foreign keys make cross-organization references impossible.
- **App role**: services connect as `yari_app`, which is not a superuser and cannot bypass RLS, with only the privileges it needs. Migrations run as the owner. Two lookups that must work before the tenant is known (`organizations_for_user`, `principal_for_token`) plus the reaper's `stuck_agent_sessions` are `SECURITY DEFINER` functions owned by a dedicated `NOLOGIN` role that can read only the tables it needs, with a pinned `search_path`.
- **Append-only logs**: `audit_events` and `agent_steps` have UPDATE, DELETE and TRUNCATE revoked from the app role and triggers that raise for everyone else. Agent steps are hash-chained (`hash = sha256(prev_hash || seq || kind || tool || summary || raw_event)`, each field length-prefixed) so tampering or removal is detectable.
- **Authorization**: built-in roles live in Go (`internal/authz`). Protected environments allow `secret.read` only for admin and owner, make a developer's `plan.apply` need approval, and `require_second_approver` extends that to admins. An agent never gets `secret.read`, always needs approval to apply on a protected environment, cannot approve plans and cannot start agent sessions. Nobody can approve a plan they created.
- **Tokens**: API tokens are shown once and stored as SHA-256 hashes. Session tokens expire with the session and are revoked when it ends. The dev identity header `X-Yari-Dev-Subject` works only when `YARI_ENV=dev`, and the API logs a warning at start.
- **Credentials**: provider credentials are stored only as ciphertext with a wrapped data key and handed to adapters as short-lived, operation-scoped leases. The broker that issues leases is not part of this scaffold. Secret outputs redact themselves when printed, and the orchestrator strips secret values from errors. Plans refuse literal secret values.
- **Sandbox isolation**: see the agent section above.
- **Never commit secrets**. `.env` is git-ignored, `.env.example` holds only `change-me` placeholders, and test data is obviously fake (`postgres://fake:fake@fake.invalid/fake`).

## Roadmap

1. **MVP**: the canvas, real Railway, Vercel and Supabase adapters, Edges, logs, rollback, governance, and the OpenHands agent with per-session sandboxes behind the LiteLLM gateway.
2. More adapters by demand, starting with `s3compat` storage.
3. Unified logs and metrics.
4. Real push notifications and OIDC on desktop and mobile.
5. Firecracker or E2B sandboxes with an egress allowlist.

## Contributing

- Work on a branch and open a pull request. Do not push to the default branch.
- Use Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, ...).
- Before pushing run `make lint test`. There is no hosted CI configured in this repository.

## License

To be decided.
