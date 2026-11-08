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

@@AGENT_SECTION@@

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

@@PREREQ@@

## Quickstart

@@QUICKSTART@@

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
