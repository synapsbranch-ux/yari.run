// Package dbtest provides helpers for tests that need a real PostgreSQL.
// Tests are skipped when YARI_TEST_DATABASE_URL (the application role URL) is unset.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

// Pool returns a pool for the application role, or skips the test.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("YARI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("YARI_TEST_DATABASE_URL is not set")
	}
	pool, err := db.NewPool(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// OwnerPool returns a pool for the owner role, or skips the test. Optional.
func OwnerPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("YARI_TEST_OWNER_DATABASE_URL")
	if url == "" {
		t.Skip("YARI_TEST_OWNER_DATABASE_URL is not set")
	}
	pool, err := db.NewPool(context.Background(), url)
	if err != nil {
		t.Fatalf("connect owner: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Org is a seeded Organization with one human principal and a full hierarchy.
type Org struct {
	ID          uuid.UUID
	Slug        string
	UserID      uuid.UUID
	Subject     string
	PrincipalID uuid.UUID // human, owner grant on the organization
	WorkspaceID uuid.UUID
	ProjectID   uuid.UUID
	ProdEnvID   uuid.UUID // protected
	DevEnvID    uuid.UUID // not protected
}

// SeedOrg creates an Organization with unique ids so tests never collide on a shared database.
func SeedOrg(t testing.TB, pool *pgxpool.Pool) Org {
	t.Helper()
	ctx := context.Background()
	o := Org{ID: db.NewID(), UserID: db.NewID(), PrincipalID: db.NewID(), WorkspaceID: db.NewID(), ProjectID: db.NewID(), ProdEnvID: db.NewID(), DevEnvID: db.NewID()}
	suffix := o.ID.String()[24:]
	o.Slug = "t-" + suffix
	o.Subject = "test-subject-" + o.ID.String()

	if _, err := dbsqlc.New(pool).UpsertUser(ctx, dbsqlc.UpsertUserParams{ID: o.UserID, ExternalSubject: o.Subject, Email: "fake@fake.invalid", DisplayName: "Fake User"}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	err := db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		if _, err := q.CreateOrganization(ctx, dbsqlc.CreateOrganizationParams{ID: o.ID, Slug: o.Slug, Name: "Test " + suffix}); err != nil {
			return err
		}
		if _, err := q.CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{ID: o.PrincipalID, OrganizationID: o.ID, Kind: "human", UserID: uuid.NullUUID{UUID: o.UserID, Valid: true}, DisplayName: "Fake User"}); err != nil {
			return err
		}
		if _, err := q.CreateGrant(ctx, dbsqlc.CreateGrantParams{ID: db.NewID(), OrganizationID: o.ID, PrincipalID: o.PrincipalID, Role: "owner", ScopeType: "organization", ScopeID: o.ID, CreatedBy: o.PrincipalID}); err != nil {
			return err
		}
		if _, err := q.CreateWorkspace(ctx, dbsqlc.CreateWorkspaceParams{ID: o.WorkspaceID, OrganizationID: o.ID, Slug: "main", Name: "Main"}); err != nil {
			return err
		}
		if _, err := q.CreateProject(ctx, dbsqlc.CreateProjectParams{ID: o.ProjectID, OrganizationID: o.ID, WorkspaceID: o.WorkspaceID, Slug: "shop", Name: "Shop"}); err != nil {
			return err
		}
		if _, err := q.CreateEnvironment(ctx, dbsqlc.CreateEnvironmentParams{ID: o.ProdEnvID, OrganizationID: o.ID, ProjectID: o.ProjectID, Slug: "production", Name: "Production", Protected: true}); err != nil {
			return err
		}
		_, err := q.CreateEnvironment(ctx, dbsqlc.CreateEnvironmentParams{ID: o.DevEnvID, OrganizationID: o.ID, ProjectID: o.ProjectID, Slug: "staging", Name: "Staging"})
		return err
	})
	if err != nil {
		t.Fatalf("seed org: %s", fmt.Sprint(err))
	}
	return o
}
