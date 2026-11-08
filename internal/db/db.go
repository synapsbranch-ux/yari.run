// Package db owns the PostgreSQL connection pool and tenant-scoped transactions.
package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx pool for the application role.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// WithTenant runs fn inside a transaction whose row-level security context is the given
// Organization. The setting is transaction local, so it cannot leak to other users of the pooled
// connection. If fn returns an error the transaction is rolled back.
func WithTenant(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	if _, err := tx.Exec(ctx, `SELECT set_config('yari.organization_id', $1, true)`, orgID.String()); err != nil {
		return fmt.Errorf("db: set tenant: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// NewID returns a new UUIDv7, the id format used by every table.
func NewID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Errorf("db: generate uuid: %w", err)) // only fails if the system random source fails
	}
	return id
}
