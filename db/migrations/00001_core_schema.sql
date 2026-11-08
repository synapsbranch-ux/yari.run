-- +goose Up
-- Core tables. Tenancy: the tenant boundary is the Organization. Every tenant table carries a
-- non-null organization_id (denormalized on purpose) so row-level security stays a single comparison.
-- Composite foreign keys (id, organization_id) make cross-organization references impossible.
-- Ids are application-generated uuids (UUIDv7).

CREATE TABLE users (
    id                uuid PRIMARY KEY,
    external_subject  text NOT NULL UNIQUE,
    email             text NOT NULL DEFAULT '',
    display_name      text NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
    id          uuid PRIMARY KEY,
    slug        text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE principals (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    kind             text NOT NULL CHECK (kind IN ('human', 'api_key', 'ci', 'agent')),
    user_id          uuid REFERENCES users (id),
    display_name     text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    -- user_id is required for humans and only for humans.
    CONSTRAINT principals_user_iff_human CHECK ((kind = 'human') = (user_id IS NOT NULL)),
    UNIQUE (organization_id, user_id),
    UNIQUE (id, organization_id)
);
-- One agent principal per name inside an organization (named after its workspace).
CREATE UNIQUE INDEX principals_agent_name_idx ON principals (organization_id, display_name) WHERE kind = 'agent';

CREATE TABLE api_tokens (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    principal_id     uuid NOT NULL,
    -- SHA-256 of the token. The plaintext is shown once and never stored.
    token_hash       bytea NOT NULL UNIQUE,
    prefix           text NOT NULL,
    label            text NOT NULL DEFAULT '',
    expires_at       timestamptz NOT NULL,
    revoked_at       timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (principal_id, organization_id) REFERENCES principals (id, organization_id)
);
CREATE INDEX api_tokens_principal_idx ON api_tokens (principal_id);

CREATE TABLE workspaces (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    slug             text NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    name             text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug),
    UNIQUE (id, organization_id)
);

CREATE TABLE projects (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    workspace_id     uuid NOT NULL,
    slug             text NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    name             text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, slug),
    UNIQUE (id, organization_id),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id)
);

CREATE TABLE environments (
    id                       uuid PRIMARY KEY,
    organization_id          uuid NOT NULL REFERENCES organizations (id),
    project_id               uuid NOT NULL,
    slug                     text NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    name                     text NOT NULL,
    protected                boolean NOT NULL DEFAULT false,
    require_second_approver  boolean NOT NULL DEFAULT false,
    created_at               timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug),
    UNIQUE (id, organization_id),
    FOREIGN KEY (project_id, organization_id) REFERENCES projects (id, organization_id)
);

CREATE TABLE services (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    project_id       uuid NOT NULL,
    name             text NOT NULL,
    kind             text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name),
    UNIQUE (id, organization_id),
    FOREIGN KEY (project_id, organization_id) REFERENCES projects (id, organization_id)
);

CREATE TABLE service_instances (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    service_id       uuid NOT NULL,
    environment_id   uuid NOT NULL,
    provider         text NOT NULL,
    external_ref     text,
    status           text NOT NULL DEFAULT 'unknown',
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (service_id, environment_id),
    FOREIGN KEY (service_id, organization_id) REFERENCES services (id, organization_id),
    FOREIGN KEY (environment_id, organization_id) REFERENCES environments (id, organization_id)
);

-- An Edge says that the consumer service depends on the dependency service.
CREATE TABLE edges (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    project_id       uuid NOT NULL,
    from_service_id  uuid NOT NULL, -- consumer
    to_service_id    uuid NOT NULL, -- dependency
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (from_service_id, to_service_id),
    CHECK (from_service_id <> to_service_id),
    FOREIGN KEY (project_id, organization_id) REFERENCES projects (id, organization_id),
    FOREIGN KEY (from_service_id, organization_id) REFERENCES services (id, organization_id),
    FOREIGN KEY (to_service_id, organization_id) REFERENCES services (id, organization_id)
);

CREATE TABLE grants (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    principal_id     uuid NOT NULL,
    role             text NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'viewer')),
    scope_type       text NOT NULL CHECK (scope_type IN ('organization', 'workspace', 'project', 'environment')),
    scope_id         uuid NOT NULL,
    created_by       uuid NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (principal_id, role, scope_type, scope_id),
    FOREIGN KEY (principal_id, organization_id) REFERENCES principals (id, organization_id),
    FOREIGN KEY (created_by, organization_id) REFERENCES principals (id, organization_id)
);
CREATE INDEX grants_scope_idx ON grants (scope_type, scope_id);

-- Plaintext credentials are never stored. Only the future credential broker may decrypt.
CREATE TABLE connections (
    id                uuid PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organizations (id),
    workspace_id      uuid,
    provider          text NOT NULL, -- Provider Catalog id
    name              text NOT NULL,
    auth_method       text NOT NULL,
    ciphertext        bytea NOT NULL,
    wrapped_data_key  bytea NOT NULL,
    kms_key_ref       text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, organization_id) REFERENCES workspaces (id, organization_id)
);

CREATE TABLE plans (
    id                uuid PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organizations (id),
    environment_id    uuid NOT NULL,
    created_by        uuid NOT NULL,
    agent_session_id  uuid,
    status            text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'pending_approval', 'approved', 'applying', 'applied', 'failed', 'cancelled')),
    changes           jsonb NOT NULL DEFAULT '[]'::jsonb,
    cost_delta_cents  bigint NOT NULL DEFAULT 0,
    risk              text NOT NULL DEFAULT 'low' CHECK (risk IN ('low', 'medium', 'high')),
    approved_by       uuid,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    FOREIGN KEY (environment_id, organization_id) REFERENCES environments (id, organization_id),
    FOREIGN KEY (created_by, organization_id) REFERENCES principals (id, organization_id),
    FOREIGN KEY (approved_by, organization_id) REFERENCES principals (id, organization_id)
);
CREATE INDEX plans_status_idx ON plans (organization_id, status);

CREATE TABLE agent_sessions (
    id                        uuid PRIMARY KEY,
    organization_id           uuid NOT NULL REFERENCES organizations (id),
    environment_id            uuid NOT NULL,
    agent_principal_id        uuid NOT NULL,
    started_by                uuid NOT NULL, -- human principal
    goal                      text NOT NULL,
    runtime                   text NOT NULL CHECK (runtime IN ('openhands', 'fake')),
    model                     text NOT NULL,
    status                    text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'waiting_approval', 'succeeded', 'failed', 'stopped')),
    sandbox_id                text,
    external_conversation_id  text,
    -- The alias of the session's LiteLLM virtual key. Never the key itself.
    gateway_key_alias         text,
    budget_usd_micros         bigint NOT NULL,
    spend_usd_micros          bigint NOT NULL DEFAULT 0,
    max_iterations            integer NOT NULL,
    started_at                timestamptz,
    ended_at                  timestamptz,
    created_at                timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    FOREIGN KEY (environment_id, organization_id) REFERENCES environments (id, organization_id),
    FOREIGN KEY (agent_principal_id, organization_id) REFERENCES principals (id, organization_id),
    FOREIGN KEY (started_by, organization_id) REFERENCES principals (id, organization_id)
);
CREATE INDEX agent_sessions_env_idx ON agent_sessions (organization_id, environment_id, created_at DESC);

ALTER TABLE plans
    ADD CONSTRAINT plans_agent_session_fk
    FOREIGN KEY (agent_session_id, organization_id) REFERENCES agent_sessions (id, organization_id);

-- Append-only, hash-chained log of everything an agent did.
-- hash = sha256(prev_hash || seq || kind || tool || summary || raw_event), computed in Go.
CREATE TABLE agent_steps (
    id                bigserial PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organizations (id),
    session_id        uuid NOT NULL,
    seq               integer NOT NULL,
    kind              text NOT NULL CHECK (kind IN ('message', 'action', 'observation', 'error', 'guardrail', 'status')),
    tool              text,
    location          text NOT NULL CHECK (location IN ('sandbox', 'yari', 'none')),
    summary           text NOT NULL,
    raw_event         jsonb NOT NULL DEFAULT '{}'::jsonb, -- harness event, secrets redacted
    external_event_id text,
    plan_id           uuid,
    prev_hash         bytea NOT NULL DEFAULT '\x'::bytea,
    hash              bytea NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, seq),
    FOREIGN KEY (session_id, organization_id) REFERENCES agent_sessions (id, organization_id)
);

-- Append-only log of every mutation.
CREATE TABLE audit_events (
    id                bigserial PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organizations (id),
    occurred_at       timestamptz NOT NULL DEFAULT now(),
    principal_id      uuid NOT NULL,
    action            text NOT NULL,
    scope_type        text NOT NULL,
    scope_id          uuid NOT NULL,
    target            text NOT NULL DEFAULT '',
    plan_id           uuid,
    agent_session_id  uuid,
    metadata          jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX audit_events_org_idx ON audit_events (organization_id, id DESC);
CREATE INDEX audit_events_scope_idx ON audit_events (organization_id, scope_type, scope_id, id DESC);

-- +goose Down
DROP TABLE audit_events, agent_steps, agent_sessions, plans, connections, grants, edges,
    service_instances, services, environments, projects, workspaces, api_tokens, principals,
    organizations, users CASCADE;
