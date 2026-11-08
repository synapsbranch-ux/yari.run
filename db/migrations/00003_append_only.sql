-- +goose Up
-- audit_events and agent_steps are append-only. Defense in depth: no privilege for the app role,
-- and triggers that raise for everyone else (including the owner).
REVOKE UPDATE, DELETE, TRUNCATE ON audit_events, agent_steps FROM yari_app;

-- +goose StatementBegin
CREATE FUNCTION forbid_mutation() RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
    RAISE EXCEPTION '% on % is not allowed: table is append-only', TG_OP, TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
END
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER audit_events_no_truncate
    BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION forbid_mutation();

CREATE TRIGGER agent_steps_append_only
    BEFORE UPDATE OR DELETE ON agent_steps
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER agent_steps_no_truncate
    BEFORE TRUNCATE ON agent_steps
    FOR EACH STATEMENT EXECUTE FUNCTION forbid_mutation();

-- +goose Down
DROP TRIGGER agent_steps_no_truncate ON agent_steps;
DROP TRIGGER agent_steps_append_only ON agent_steps;
DROP TRIGGER audit_events_no_truncate ON audit_events;
DROP TRIGGER audit_events_append_only ON audit_events;
DROP FUNCTION forbid_mutation();
