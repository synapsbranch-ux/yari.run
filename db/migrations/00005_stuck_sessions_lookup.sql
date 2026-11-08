-- +goose Up
-- The worker's reaper must find Agent Sessions whose worker died, across every Organization.
-- Same pattern as the other lookups: a SECURITY DEFINER function owned by the NOLOGIN yari_lookup
-- role, which is the only role besides the tenant policy with read access.
GRANT SELECT ON agent_sessions TO yari_lookup;
CREATE POLICY lookup_read ON agent_sessions FOR SELECT TO yari_lookup USING (true);

-- +goose StatementBegin
CREATE FUNCTION stuck_agent_sessions(p_started_before timestamptz)
RETURNS TABLE (organization_id uuid, session_id uuid, gateway_key_alias text, sandbox_id text)
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
    SELECT s.organization_id, s.id, s.gateway_key_alias, s.sandbox_id
    FROM public.agent_sessions s
    WHERE s.status IN ('queued', 'running')
      AND s.created_at < p_started_before
$$;
-- +goose StatementEnd

ALTER FUNCTION stuck_agent_sessions(timestamptz) OWNER TO yari_lookup;
REVOKE ALL ON FUNCTION stuck_agent_sessions(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stuck_agent_sessions(timestamptz) TO yari_app;

-- +goose Down
DROP FUNCTION stuck_agent_sessions(timestamptz);
DROP POLICY lookup_read ON agent_sessions;
REVOKE SELECT ON agent_sessions FROM yari_lookup;
