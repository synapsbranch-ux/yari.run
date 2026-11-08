-- +goose Up
-- Row-level security on every tenant table, enabled AND forced so even the table owner is subject to it.
-- The application sets yari.organization_id per transaction (internal/db.WithTenant).
-- nullif guards the empty string that a connection reports after a transaction-local setting ends.

-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'principals', 'api_tokens', 'workspaces', 'projects', 'environments', 'services',
        'service_instances', 'edges', 'grants', 'connections', 'plans', 'agent_sessions',
        'agent_steps', 'audit_events'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I
                 USING (organization_id = nullif(current_setting(''yari.organization_id'', true), '''')::uuid)
                 WITH CHECK (organization_id = nullif(current_setting(''yari.organization_id'', true), '''')::uuid)',
            t);
    END LOOP;
END
$$;
-- +goose StatementEnd

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON organizations
    USING (id = nullif(current_setting('yari.organization_id', true), '')::uuid)
    WITH CHECK (id = nullif(current_setting('yari.organization_id', true), '')::uuid);

-- Least privilege for the application role. It must exist (see db/init/01-roles.sql locally).
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'yari_app') THEN
        RAISE EXCEPTION 'role yari_app must exist before migrating (see db/init/01-roles.sql)';
    END IF;
END
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO yari_app;

GRANT SELECT, INSERT, UPDATE ON users TO yari_app;
GRANT SELECT, INSERT, UPDATE ON organizations, principals, api_tokens, workspaces, projects,
    environments, connections, plans, agent_sessions TO yari_app;
-- Rows that are removed in normal operation.
GRANT SELECT, INSERT, UPDATE, DELETE ON services, service_instances, edges, grants TO yari_app;
-- Append-only tables: no UPDATE, no DELETE (enforced again by triggers).
GRANT SELECT, INSERT ON agent_steps, audit_events TO yari_app;
GRANT USAGE ON SEQUENCE agent_steps_id_seq, audit_events_id_seq TO yari_app;

-- +goose Down
DROP POLICY tenant_isolation ON organizations;
ALTER TABLE organizations DISABLE ROW LEVEL SECURITY;
-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'principals', 'api_tokens', 'workspaces', 'projects', 'environments', 'services',
        'service_instances', 'edges', 'grants', 'connections', 'plans', 'agent_sessions',
        'agent_steps', 'audit_events'
    ] LOOP
        EXECUTE format('DROP POLICY tenant_isolation ON %I', t);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
    END LOOP;
END
$$;
-- +goose StatementEnd
