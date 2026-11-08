-- name: InsertAuditEvent :one
INSERT INTO audit_events (organization_id, principal_id, action, scope_type, scope_id, target, plan_id, agent_session_id, metadata)
VALUES (@organization_id, @principal_id, @action, @scope_type, @scope_id, @target, sqlc.narg('plan_id'), sqlc.narg('agent_session_id'), @metadata)
RETURNING *;

-- name: ListAuditEvents :many
-- Keyset pagination, newest first. before_id of 0 means the first page.
SELECT * FROM audit_events
WHERE (@before_id::bigint = 0 OR id < @before_id)
  AND (sqlc.narg('scope_type')::text IS NULL OR (scope_type = sqlc.narg('scope_type') AND scope_id = sqlc.narg('scope_id')::uuid))
ORDER BY id DESC
LIMIT @page_size;
