-- name: CreateWorkspace :one
INSERT INTO workspaces (id, organization_id, slug, name) VALUES (@id, @organization_id, @slug, @name) RETURNING *;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = @id;

-- name: ListWorkspaces :many
SELECT * FROM workspaces ORDER BY created_at;

-- name: CreateProject :one
INSERT INTO projects (id, organization_id, workspace_id, slug, name)
VALUES (@id, @organization_id, @workspace_id, @slug, @name) RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = @id;

-- name: ListProjects :many
SELECT * FROM projects WHERE workspace_id = @workspace_id ORDER BY created_at;

-- name: CreateEnvironment :one
INSERT INTO environments (id, organization_id, project_id, slug, name, protected, require_second_approver)
VALUES (@id, @organization_id, @project_id, @slug, @name, @protected, @require_second_approver)
RETURNING *;

-- name: GetEnvironment :one
SELECT * FROM environments WHERE id = @id;

-- name: ListEnvironments :many
SELECT * FROM environments WHERE project_id = @project_id ORDER BY created_at;

-- name: ListServicesForEnvironment :many
-- Services of the environment's project, with their state in that environment.
SELECT s.id, s.organization_id, s.project_id, s.name, s.kind,
       COALESCE(si.provider, '') AS provider,
       COALESCE(si.status, 'not_deployed') AS status
FROM services s
JOIN environments e ON e.project_id = s.project_id AND e.id = @environment_id
LEFT JOIN service_instances si ON si.service_id = s.id AND si.environment_id = e.id
ORDER BY s.name;

-- name: ScopeChain :many
-- The ancestor chain of a scope, including the scope itself, in one query.
-- scope_type is one of environment, project, workspace, organization.
WITH s AS (
    SELECT @scope_type::text AS t, @scope_id::uuid AS id
), env AS (
    SELECT e.id, e.project_id FROM environments e, s WHERE s.t = 'environment' AND e.id = s.id
), proj AS (
    SELECT p.id, p.workspace_id FROM projects p
    WHERE p.id = (SELECT project_id FROM env) OR p.id = (SELECT id FROM s WHERE t = 'project')
), ws AS (
    SELECT w.id, w.organization_id FROM workspaces w
    WHERE w.id = (SELECT workspace_id FROM proj) OR w.id = (SELECT id FROM s WHERE t = 'workspace')
), org AS (
    SELECT o.id FROM organizations o
    WHERE o.id = (SELECT organization_id FROM ws) OR o.id = (SELECT id FROM s WHERE t = 'organization')
)
SELECT 'environment'::text AS scope_type, id AS scope_id FROM env
UNION ALL SELECT 'project', id FROM proj
UNION ALL SELECT 'workspace', id FROM ws
UNION ALL SELECT 'organization', id FROM org;

-- name: CreateGrant :one
INSERT INTO grants (id, organization_id, principal_id, role, scope_type, scope_id, created_by)
VALUES (@id, @organization_id, @principal_id, @role, @scope_type, @scope_id, @created_by)
ON CONFLICT (principal_id, role, scope_type, scope_id) DO UPDATE SET role = EXCLUDED.role
RETURNING *;

-- name: DeleteGrant :one
DELETE FROM grants WHERE id = @id RETURNING *;

-- name: ListGrants :many
SELECT * FROM grants
WHERE (sqlc.narg('principal_id')::uuid IS NULL OR principal_id = sqlc.narg('principal_id'))
ORDER BY created_at;

-- name: GrantsForPrincipalOverScopes :many
-- Grants held by the principal on any of the given scope ids (typically an ancestor chain).
-- Callers must still compare scope_type, because the filter here is on ids only.
SELECT * FROM grants
WHERE principal_id = @principal_id AND scope_id = ANY(@scope_ids::uuid[]);
