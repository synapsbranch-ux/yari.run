-- name: CreateOrganization :one
INSERT INTO organizations (id, slug, name) VALUES (@id, @slug, @name) RETURNING *;

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = @id;

-- name: ListOrganizations :many
-- Row-level security limits this to the current tenant.
SELECT * FROM organizations ORDER BY created_at;

-- name: CreatePrincipal :one
INSERT INTO principals (id, organization_id, kind, user_id, display_name)
VALUES (@id, @organization_id, @kind, sqlc.narg('user_id'), @display_name)
RETURNING *;

-- name: GetPrincipal :one
SELECT * FROM principals WHERE id = @id;

-- name: GetHumanPrincipal :one
SELECT * FROM principals WHERE user_id = @user_id;

-- name: GetAgentPrincipalByName :one
SELECT * FROM principals WHERE kind = 'agent' AND display_name = @display_name;

-- name: ListPrincipals :many
SELECT * FROM principals ORDER BY created_at;

-- name: CreateAPIToken :one
INSERT INTO api_tokens (id, organization_id, principal_id, token_hash, prefix, label, expires_at)
VALUES (@id, @organization_id, @principal_id, @token_hash, @prefix, @label, @expires_at)
RETURNING *;

-- name: RevokeAPIToken :execrows
UPDATE api_tokens SET revoked_at = now() WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeAPITokensForPrincipal :execrows
UPDATE api_tokens SET revoked_at = now() WHERE principal_id = @principal_id AND revoked_at IS NULL;

-- name: GetAPIToken :one
SELECT * FROM api_tokens WHERE id = @id;

-- name: RevokeAPITokensByLabel :execrows
UPDATE api_tokens SET revoked_at = now() WHERE label = @label AND revoked_at IS NULL;

-- name: GetServiceByName :one
SELECT * FROM services WHERE project_id = @project_id AND name = @name;
