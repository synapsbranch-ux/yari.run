-- name: CreateAgentSession :one
INSERT INTO agent_sessions (id, organization_id, environment_id, agent_principal_id, started_by, goal, runtime, model,
                            status, budget_usd_micros, max_iterations)
VALUES (@id, @organization_id, @environment_id, @agent_principal_id, @started_by, @goal, @runtime, @model,
        'queued', @budget_usd_micros, @max_iterations)
RETURNING *;

-- name: GetAgentSession :one
SELECT * FROM agent_sessions WHERE id = @id;

-- name: ListAgentSessions :many
SELECT * FROM agent_sessions
WHERE (sqlc.narg('environment_id')::uuid IS NULL OR environment_id = sqlc.narg('environment_id'))
ORDER BY created_at DESC
LIMIT @page_size;

-- name: MarkAgentSessionRunning :one
UPDATE agent_sessions SET status = 'running', started_at = COALESCE(started_at, now()), gateway_key_alias = @gateway_key_alias
WHERE id = @id RETURNING *;

-- name: SetAgentSessionSandbox :exec
UPDATE agent_sessions SET sandbox_id = @sandbox_id WHERE id = @id;

-- name: SetAgentSessionConversation :exec
UPDATE agent_sessions SET external_conversation_id = @external_conversation_id WHERE id = @id;

-- name: SetAgentSessionStatus :one
UPDATE agent_sessions SET status = @status WHERE id = @id RETURNING *;

-- name: FinishAgentSession :one
UPDATE agent_sessions
SET status = @status, spend_usd_micros = @spend_usd_micros, ended_at = now()
WHERE id = @id RETURNING *;

-- name: AppendAgentStep :one
INSERT INTO agent_steps (organization_id, session_id, seq, kind, tool, location, summary, raw_event,
                         external_event_id, plan_id, prev_hash, hash)
VALUES (@organization_id, @session_id, @seq, @kind, sqlc.narg('tool'), @location, @summary, @raw_event,
        sqlc.narg('external_event_id'), sqlc.narg('plan_id'), @prev_hash, @hash)
RETURNING *;

-- name: LastAgentStep :one
SELECT * FROM agent_steps WHERE session_id = @session_id ORDER BY seq DESC LIMIT 1;

-- name: ListAgentSteps :many
SELECT * FROM agent_steps WHERE session_id = @session_id AND seq > @after_seq ORDER BY seq;
