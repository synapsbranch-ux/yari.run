-- name: CreatePlan :one
INSERT INTO plans (id, organization_id, environment_id, created_by, agent_session_id, status, changes, cost_delta_cents, risk)
VALUES (@id, @organization_id, @environment_id, @created_by, sqlc.narg('agent_session_id'), @status, @changes, @cost_delta_cents, @risk)
RETURNING *;

-- name: GetPlan :one
SELECT * FROM plans WHERE id = @id;

-- name: GetPlanForUpdate :one
SELECT * FROM plans WHERE id = @id FOR UPDATE;

-- name: ListPendingPlans :many
SELECT * FROM plans WHERE status = 'pending_approval' ORDER BY created_at;

-- name: ListPlansForEnvironment :many
SELECT * FROM plans WHERE environment_id = @environment_id ORDER BY created_at DESC LIMIT 100;

-- name: SetPlanStatus :one
UPDATE plans
SET status = @status,
    approved_by = COALESCE(sqlc.narg('approved_by'), approved_by),
    updated_at = now()
WHERE id = @id
RETURNING *;
