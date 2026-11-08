-- +goose Up
-- Two lookups must work before the caller's organization is known: which organizations a user
-- belongs to, and which principal a token belongs to. They are SECURITY DEFINER functions owned by
-- a dedicated NOLOGIN role (yari_lookup) that can only read the two tables it needs, and that is
-- the only role besides yari_app with a policy on them. Search path is pinned and every name is
-- schema qualified.

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'yari_lookup') THEN
        CREATE ROLE yari_lookup NOLOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
END
$$;
-- +goose StatementEnd

-- Needed so a non-superuser migration role can hand ownership to yari_lookup.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT pg_has_role(current_user, 'yari_lookup', 'MEMBER') THEN
        EXECUTE format('GRANT yari_lookup TO %I', current_user);
    END IF;
END
$$;
-- +goose StatementEnd

GRANT SELECT ON principals, api_tokens TO yari_lookup;
CREATE POLICY lookup_read ON principals FOR SELECT TO yari_lookup USING (true);
CREATE POLICY lookup_read ON api_tokens FOR SELECT TO yari_lookup USING (true);

-- +goose StatementBegin
CREATE FUNCTION organizations_for_user(p_user_id uuid) RETURNS SETOF uuid
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
    SELECT DISTINCT p.organization_id FROM public.principals p WHERE p.user_id = p_user_id
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION principal_for_token(p_token_hash bytea) RETURNS TABLE (principal_id uuid, organization_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
    SELECT t.principal_id, t.organization_id
    FROM public.api_tokens t
    WHERE t.token_hash = p_token_hash
      AND t.revoked_at IS NULL
      AND t.expires_at > now()
$$;
-- +goose StatementEnd

ALTER FUNCTION organizations_for_user(uuid) OWNER TO yari_lookup;
ALTER FUNCTION principal_for_token(bytea) OWNER TO yari_lookup;
REVOKE ALL ON FUNCTION organizations_for_user(uuid), principal_for_token(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION organizations_for_user(uuid), principal_for_token(bytea) TO yari_app;

-- +goose Down
DROP FUNCTION principal_for_token(bytea);
DROP FUNCTION organizations_for_user(uuid);
DROP POLICY lookup_read ON api_tokens;
DROP POLICY lookup_read ON principals;
REVOKE SELECT ON principals, api_tokens FROM yari_lookup;
