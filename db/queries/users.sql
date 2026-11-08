-- name: UpsertUser :one
-- Creates the user for an unknown OIDC subject, or refreshes profile fields for a known one.
INSERT INTO users (id, external_subject, email, display_name)
VALUES (@id, @external_subject, @email, @display_name)
ON CONFLICT (external_subject) DO UPDATE
    SET email = CASE WHEN EXCLUDED.email <> '' THEN EXCLUDED.email ELSE users.email END,
        display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name ELSE users.display_name END
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = @id;

