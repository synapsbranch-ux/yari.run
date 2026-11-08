package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrganizationsForUser returns the organizations a user belongs to. It calls a SECURITY DEFINER
// function, so it works before the caller's tenant context is known.
func OrganizationsForUser(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `SELECT organizations_for_user($1)`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: organizations for user: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("db: organizations for user: %w", err)
	}
	return ids, nil
}

// TokenOwner identifies who a valid API token belongs to.
type TokenOwner struct {
	PrincipalID    uuid.UUID
	OrganizationID uuid.UUID
}

// PrincipalForToken resolves a token hash to its principal. It returns pgx.ErrNoRows
// when the token is unknown, expired or revoked.
func PrincipalForToken(ctx context.Context, pool *pgxpool.Pool, tokenHash []byte) (TokenOwner, error) {
	var o TokenOwner
	err := pool.QueryRow(ctx, `SELECT principal_id, organization_id FROM principal_for_token($1)`, tokenHash).
		Scan(&o.PrincipalID, &o.OrganizationID)
	if err != nil {
		return TokenOwner{}, err
	}
	return o, nil
}

// StuckSession is an Agent Session whose worker is presumed dead.
type StuckSession struct {
	OrganizationID uuid.UUID
	SessionID      uuid.UUID
	GatewayAlias   string
	SandboxID      string
}

// StuckAgentSessions lists queued or running sessions created before the cutoff, across
// Organizations, through a SECURITY DEFINER function.
func StuckAgentSessions(ctx context.Context, pool *pgxpool.Pool, createdBefore time.Time) ([]StuckSession, error) {
	rows, err := pool.Query(ctx, `SELECT organization_id, session_id, COALESCE(gateway_key_alias, ''), COALESCE(sandbox_id, '') FROM stuck_agent_sessions($1)`, createdBefore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StuckSession
	for rows.Next() {
		var s StuckSession
		if err := rows.Scan(&s.OrganizationID, &s.SessionID, &s.GatewayAlias, &s.SandboxID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
