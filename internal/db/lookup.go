package db

import (
	"context"
	"fmt"

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
