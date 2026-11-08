package db_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbtest"
)

func TestRLSIsolationBetweenOrganizations(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	a, b := dbtest.SeedOrg(t, pool), dbtest.SeedOrg(t, pool)

	// Each tenant sees only its own rows.
	for _, o := range []dbtest.Org{a, b} {
		require.NoError(t, db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
			q := dbsqlc.New(tx)
			orgs, err := q.ListOrganizations(ctx)
			require.NoError(t, err)
			require.Len(t, orgs, 1)
			require.Equal(t, o.ID, orgs[0].ID)
			ws, err := q.ListWorkspaces(ctx)
			require.NoError(t, err)
			require.Len(t, ws, 1)
			require.Equal(t, o.WorkspaceID, ws[0].ID)
			return nil
		}))
	}

	// Tenant A cannot read tenant B's rows by id.
	require.NoError(t, db.WithTenant(ctx, pool, a.ID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := dbsqlc.New(tx).GetWorkspace(ctx, b.WorkspaceID)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		return nil
	}))

	// Tenant A cannot write a row that belongs to tenant B (WITH CHECK).
	err := db.WithTenant(ctx, pool, a.ID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := dbsqlc.New(tx).CreateWorkspace(ctx, dbsqlc.CreateWorkspaceParams{ID: db.NewID(), OrganizationID: b.ID, Slug: "evil", Name: "Evil"})
		return err
	})
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, "42501", pgErr.Code, "row-level security violation expected")

	// Tenant A cannot update tenant B's rows: zero rows affected.
	require.NoError(t, db.WithTenant(ctx, pool, a.ID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE workspaces SET name = 'hijacked' WHERE id = $1`, b.WorkspaceID)
		require.NoError(t, err)
		require.Zero(t, tag.RowsAffected())
		return nil
	}))

	// Without a tenant context nothing is visible, and the setting does not leak between transactions.
	for i := 0; i < 3; i++ {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM workspaces`).Scan(&n))
		require.Zero(t, n)
	}
}

func TestAppendOnlyTables(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, pool)

	sessionID := db.NewID()
	var auditID, stepID int64
	require.NoError(t, db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		agent, err := q.CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{ID: db.NewID(), OrganizationID: o.ID, Kind: "agent", DisplayName: "yari-agent/main"})
		require.NoError(t, err)
		_, err = q.CreateAgentSession(ctx, dbsqlc.CreateAgentSessionParams{ID: sessionID, OrganizationID: o.ID, EnvironmentID: o.ProdEnvID, AgentPrincipalID: agent.ID, StartedBy: o.PrincipalID, Goal: "test", Runtime: "fake", Model: "m", BudgetUsdMicros: 1, MaxIterations: 1})
		require.NoError(t, err)
		ev, err := q.InsertAuditEvent(ctx, dbsqlc.InsertAuditEventParams{OrganizationID: o.ID, PrincipalID: o.PrincipalID, Action: "test.insert", ScopeType: "organization", ScopeID: o.ID, Metadata: []byte(`{}`)})
		require.NoError(t, err)
		auditID = ev.ID
		h := sha256.Sum256([]byte("x"))
		st, err := q.AppendAgentStep(ctx, dbsqlc.AppendAgentStepParams{OrganizationID: o.ID, SessionID: sessionID, Seq: 1, Kind: "message", Location: "none", Summary: "hi", RawEvent: []byte(`{}`), PrevHash: []byte{}, Hash: h[:]})
		require.NoError(t, err)
		stepID = st.ID
		return nil
	}))

	cases := []struct {
		name, sql string
		arg       any
	}{
		{"audit update", `UPDATE audit_events SET action = 'x' WHERE id = $1`, auditID},
		{"audit delete", `DELETE FROM audit_events WHERE id = $1`, auditID},
		{"steps update", `UPDATE agent_steps SET summary = 'x' WHERE id = $1`, stepID},
		{"steps delete", `DELETE FROM agent_steps WHERE id = $1`, stepID},
	}
	for _, c := range cases {
		t.Run("app role: "+c.name, func(t *testing.T) {
			err := db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, c.sql, c.arg)
				return err
			})
			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "expected a database error, got %v", err)
			require.Equal(t, "42501", pgErr.Code, "permission denied expected")
		})
	}

	// The triggers stop everyone else too, including the owner. Optional: needs the owner URL.
	t.Run("owner role: triggers", func(t *testing.T) {
		owner := dbtest.OwnerPool(t)
		for _, c := range cases {
			_, err := owner.Exec(ctx, c.sql, c.arg)
			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "%s: expected error, got %v", c.name, err)
			require.Equal(t, "23001", pgErr.Code, "%s: restrict_violation from trigger expected", c.name)
		}
		_, err := owner.Exec(ctx, `TRUNCATE audit_events`)
		require.Error(t, err)
	})
}

func TestSecurityDefinerFunctions(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	a, b := dbtest.SeedOrg(t, pool), dbtest.SeedOrg(t, pool)

	t.Run("organizations_for_user", func(t *testing.T) {
		orgs, err := db.OrganizationsForUser(ctx, pool, a.UserID)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{a.ID}, orgs)

		// The same user joins a second organization.
		require.NoError(t, db.WithTenant(ctx, pool, b.ID, func(ctx context.Context, tx pgx.Tx) error {
			_, err := dbsqlc.New(tx).CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{ID: db.NewID(), OrganizationID: b.ID, Kind: "human", UserID: uuid.NullUUID{UUID: a.UserID, Valid: true}, DisplayName: "Fake User"})
			return err
		}))
		orgs, err = db.OrganizationsForUser(ctx, pool, a.UserID)
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{a.ID, b.ID}, orgs)

		orgs, err = db.OrganizationsForUser(ctx, pool, db.NewID())
		require.NoError(t, err)
		require.Empty(t, orgs)
	})

	t.Run("principal_for_token", func(t *testing.T) {
		mk := func(label string, expires time.Time, revoke bool) []byte {
			sum := sha256.Sum256([]byte("fake-token-" + label + a.ID.String()))
			require.NoError(t, db.WithTenant(ctx, pool, a.ID, func(ctx context.Context, tx pgx.Tx) error {
				q := dbsqlc.New(tx)
				tok, err := q.CreateAPIToken(ctx, dbsqlc.CreateAPITokenParams{ID: db.NewID(), OrganizationID: a.ID, PrincipalID: a.PrincipalID, TokenHash: sum[:], Prefix: "yari_fake", Label: label, ExpiresAt: expires})
				require.NoError(t, err)
				if revoke {
					_, err = q.RevokeAPIToken(ctx, tok.ID)
				}
				return err
			}))
			return sum[:]
		}

		owner, err := db.PrincipalForToken(ctx, pool, mk("valid", time.Now().Add(time.Hour), false))
		require.NoError(t, err)
		require.Equal(t, a.PrincipalID, owner.PrincipalID)
		require.Equal(t, a.ID, owner.OrganizationID)

		_, err = db.PrincipalForToken(ctx, pool, mk("expired", time.Now().Add(-time.Minute), false))
		require.ErrorIs(t, err, pgx.ErrNoRows)
		_, err = db.PrincipalForToken(ctx, pool, mk("revoked", time.Now().Add(time.Hour), true))
		require.ErrorIs(t, err, pgx.ErrNoRows)
		_, err = db.PrincipalForToken(ctx, pool, []byte("unknown-hash"))
		require.ErrorIs(t, err, pgx.ErrNoRows)
	})

	t.Run("app role cannot read principals across tenants directly", func(t *testing.T) {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM principals`).Scan(&n))
		require.Zero(t, n)
	})
}
