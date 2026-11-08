package authz_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbtest"
)

var allPerms = []authz.Permission{
	authz.OrganizationRead, authz.WorkspaceRead, authz.ProjectRead, authz.EnvironmentRead, authz.ServiceRead,
	authz.PlanRead, authz.LogsRead, authz.AgentRead,
	authz.ServiceWrite, authz.PlanCreate, authz.ConnectionUse, authz.AgentRun,
	authz.PlanApply, authz.SecretRead,
	authz.PlanApprove, authz.ConnectionManage, authz.MemberManage, authz.EnvironmentManage, authz.AuditRead, authz.AgentManage,
	authz.OrganizationManage,
}

// The matrix from the product spec: which roles hold which permission on an unprotected environment.
var matrix = map[authz.Permission][4]bool{ // viewer, developer, admin, owner
	authz.OrganizationRead: {true, true, true, true}, authz.WorkspaceRead: {true, true, true, true},
	authz.ProjectRead: {true, true, true, true}, authz.EnvironmentRead: {true, true, true, true},
	authz.ServiceRead: {true, true, true, true}, authz.PlanRead: {true, true, true, true},
	authz.LogsRead: {true, true, true, true}, authz.AgentRead: {true, true, true, true},
	authz.ServiceWrite: {false, true, true, true}, authz.PlanCreate: {false, true, true, true},
	authz.ConnectionUse: {false, true, true, true}, authz.AgentRun: {false, true, true, true},
	authz.PlanApply: {false, true, true, true}, authz.SecretRead: {false, true, true, true},
	authz.PlanApprove: {false, false, true, true}, authz.ConnectionManage: {false, false, true, true},
	authz.MemberManage: {false, false, true, true}, authz.EnvironmentManage: {false, false, true, true},
	authz.AuditRead: {false, false, true, true}, authz.AgentManage: {false, false, true, true},
	authz.OrganizationManage: {false, false, false, true},
}

func TestRoleMatrixIsComplete(t *testing.T) {
	require.Len(t, matrix, len(allPerms), "every permission must appear in the matrix")
	roles := []authz.Role{authz.RoleViewer, authz.RoleDeveloper, authz.RoleAdmin, authz.RoleOwner}
	for _, perm := range allPerms {
		for i, role := range roles {
			d := authz.Evaluate(authz.KindHuman, role, perm, &authz.EnvInfo{})
			require.Equal(t, matrix[perm][i], d.Allowed, "%s on %s", role, perm)
			require.False(t, d.RequiresApproval, "%s on %s must not need approval on an unprotected environment", role, perm)
		}
	}
}

func TestEnvironmentAndAgentRules(t *testing.T) {
	prot := &authz.EnvInfo{Protected: true}
	second := &authz.EnvInfo{Protected: true, RequireSecondApprover: true}
	plain := &authz.EnvInfo{}
	type c struct {
		name         string
		kind         authz.PrincipalKind
		role         authz.Role
		perm         authz.Permission
		env          *authz.EnvInfo
		allowed, req bool
	}
	cases := []c{
		// Rule 1: secret.read on protected environments is admin and owner only.
		{"dev reads secret on plain env", authz.KindHuman, authz.RoleDeveloper, authz.SecretRead, plain, true, false},
		{"dev denied secret on protected", authz.KindHuman, authz.RoleDeveloper, authz.SecretRead, prot, false, false},
		{"admin reads secret on protected", authz.KindHuman, authz.RoleAdmin, authz.SecretRead, prot, true, false},
		{"owner reads secret on protected", authz.KindHuman, authz.RoleOwner, authz.SecretRead, prot, true, false},
		{"viewer denied secret", authz.KindHuman, authz.RoleViewer, authz.SecretRead, plain, false, false},
		{"api key dev denied secret on protected", authz.KindAPIKey, authz.RoleDeveloper, authz.SecretRead, prot, false, false},
		// Rule 2: developer apply on protected needs approval.
		{"dev apply plain", authz.KindHuman, authz.RoleDeveloper, authz.PlanApply, plain, true, false},
		{"dev apply protected", authz.KindHuman, authz.RoleDeveloper, authz.PlanApply, prot, true, true},
		{"ci dev apply protected", authz.KindCI, authz.RoleDeveloper, authz.PlanApply, prot, true, true},
		// Rule 3: admin and owner apply directly, unless a second approver is required.
		{"admin apply protected", authz.KindHuman, authz.RoleAdmin, authz.PlanApply, prot, true, false},
		{"owner apply protected", authz.KindHuman, authz.RoleOwner, authz.PlanApply, prot, true, false},
		{"admin apply second approver", authz.KindHuman, authz.RoleAdmin, authz.PlanApply, second, true, true},
		{"owner apply second approver", authz.KindHuman, authz.RoleOwner, authz.PlanApply, second, true, true},
		// Rule 4: agents.
		{"agent never reads secret (dev)", authz.KindAgent, authz.RoleDeveloper, authz.SecretRead, plain, false, false},
		{"agent never reads secret (owner)", authz.KindAgent, authz.RoleOwner, authz.SecretRead, plain, false, false},
		{"agent never reads secret (owner, protected)", authz.KindAgent, authz.RoleOwner, authz.SecretRead, prot, false, false},
		{"agent dev apply protected", authz.KindAgent, authz.RoleDeveloper, authz.PlanApply, prot, true, true},
		{"agent admin apply protected", authz.KindAgent, authz.RoleAdmin, authz.PlanApply, prot, true, true},
		{"agent owner apply protected", authz.KindAgent, authz.RoleOwner, authz.PlanApply, prot, true, true},
		{"agent dev apply plain", authz.KindAgent, authz.RoleDeveloper, authz.PlanApply, plain, true, false},
		{"agent cannot approve", authz.KindAgent, authz.RoleOwner, authz.PlanApprove, prot, false, false},
		{"agent can propose", authz.KindAgent, authz.RoleDeveloper, authz.PlanCreate, prot, true, false},
		// No environment in the chain (for example a project scope): flags do not apply.
		{"dev apply without env", authz.KindHuman, authz.RoleDeveloper, authz.PlanApply, nil, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := authz.Evaluate(tc.kind, tc.role, tc.perm, tc.env)
			require.Equal(t, tc.allowed, d.Allowed, d.Reason)
			require.Equal(t, tc.req, d.RequiresApproval, d.Reason)
			require.NotEmpty(t, d.Reason)
		})
	}
}

// fakeLoader serves a fixed hierarchy: org > ws > project > env.
type fakeLoader struct {
	chain  []authz.Scope
	grants []authz.Grant
	env    authz.EnvInfo
}

func (f fakeLoader) ScopeChain(context.Context, authz.Scope) ([]authz.Scope, error) {
	return f.chain, nil
}
func (f fakeLoader) Grants(_ context.Context, _ uuid.UUID, chain []authz.Scope) ([]authz.Grant, error) {
	var out []authz.Grant
	for _, g := range f.grants {
		for _, s := range chain {
			if g.Scope == s {
				out = append(out, g)
			}
		}
	}
	return out, nil
}
func (f fakeLoader) Environment(context.Context, uuid.UUID) (authz.EnvInfo, error) { return f.env, nil }

func TestCanHighestRoleOnChainWinsAndNoGrantDenies(t *testing.T) {
	org, ws, pr, env := authz.Scope{Type: authz.ScopeOrganization, ID: db.NewID()}, authz.Scope{Type: authz.ScopeWorkspace, ID: db.NewID()},
		authz.Scope{Type: authz.ScopeProject, ID: db.NewID()}, authz.Scope{Type: authz.ScopeEnvironment, ID: db.NewID()}
	chain := []authz.Scope{env, pr, ws, org}
	p := authz.Principal{ID: db.NewID(), Kind: authz.KindHuman}
	ctx := context.Background()

	l := fakeLoader{chain: chain, grants: []authz.Grant{{Role: authz.RoleViewer, Scope: org}, {Role: authz.RoleAdmin, Scope: pr}, {Role: authz.RoleDeveloper, Scope: env}}}
	d, err := authz.New(l).Can(ctx, p, authz.PlanApprove, env)
	require.NoError(t, err)
	require.True(t, d.Allowed, "admin on the project carries down and beats developer on the environment")

	d, err = authz.New(fakeLoader{chain: chain}).Can(ctx, p, authz.PlanRead, env)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "no grant")

	d, err = authz.New(fakeLoader{}).Can(ctx, p, authz.PlanRead, env)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "not found")

	d, err = authz.New(l).Can(ctx, p, authz.PlanRead, authz.Scope{Type: "bogus", ID: db.NewID()})
	require.NoError(t, err)
	require.False(t, d.Allowed)
}

func TestCanApproveForbidsSelfApproval(t *testing.T) {
	env := authz.Scope{Type: authz.ScopeEnvironment, ID: db.NewID()}
	p := authz.Principal{ID: db.NewID(), Kind: authz.KindHuman}
	a := authz.New(fakeLoader{chain: []authz.Scope{env}, grants: []authz.Grant{{Role: authz.RoleOwner, Scope: env}}})
	d, err := a.CanApprove(context.Background(), p, env, p.ID)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "created")

	d, err = a.CanApprove(context.Background(), p, env, db.NewID())
	require.NoError(t, err)
	require.True(t, d.Allowed)
}

func TestInheritanceAgainstDatabase(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, pool)

	// A second workspace with its own project and environment.
	otherWs, otherProj, otherEnv := db.NewID(), db.NewID(), db.NewID()
	dev, viewer, wsAdmin, envDev := db.NewID(), db.NewID(), db.NewID(), db.NewID()
	require.NoError(t, db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		_, err := q.CreateWorkspace(ctx, dbsqlc.CreateWorkspaceParams{ID: otherWs, OrganizationID: o.ID, Slug: "other", Name: "Other"})
		require.NoError(t, err)
		_, err = q.CreateProject(ctx, dbsqlc.CreateProjectParams{ID: otherProj, OrganizationID: o.ID, WorkspaceID: otherWs, Slug: "p2", Name: "P2"})
		require.NoError(t, err)
		_, err = q.CreateEnvironment(ctx, dbsqlc.CreateEnvironmentParams{ID: otherEnv, OrganizationID: o.ID, ProjectID: otherProj, Slug: "production", Name: "Production", Protected: true})
		require.NoError(t, err)
		for _, pr := range []uuid.UUID{dev, viewer, wsAdmin, envDev} {
			_, err = q.CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{ID: pr, OrganizationID: o.ID, Kind: "api_key", DisplayName: pr.String()})
			require.NoError(t, err)
		}
		grant := func(p uuid.UUID, role, st string, sid uuid.UUID) {
			_, err := q.CreateGrant(ctx, dbsqlc.CreateGrantParams{ID: db.NewID(), OrganizationID: o.ID, PrincipalID: p, Role: role, ScopeType: st, ScopeID: sid, CreatedBy: o.PrincipalID})
			require.NoError(t, err)
		}
		grant(dev, "developer", "organization", o.ID)
		grant(viewer, "viewer", "project", o.ProjectID)
		grant(wsAdmin, "admin", "workspace", o.WorkspaceID)
		grant(envDev, "developer", "environment", o.DevEnvID)
		return nil
	}))

	can := func(p uuid.UUID, kind authz.PrincipalKind, perm authz.Permission, st authz.ScopeType, sid uuid.UUID) authz.Decision {
		var d authz.Decision
		require.NoError(t, db.WithTenant(ctx, pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
			var err error
			d, err = authz.New(authz.DBLoader{Q: dbsqlc.New(tx)}).Can(ctx, authz.Principal{ID: p, OrganizationID: o.ID, Kind: kind}, perm, authz.Scope{Type: st, ID: sid})
			return err
		}))
		return d
	}
	E := authz.ScopeEnvironment

	// Organization-level developer reaches every environment in the organization.
	require.True(t, can(dev, authz.KindHuman, authz.PlanCreate, E, o.DevEnvID).Allowed)
	require.True(t, can(dev, authz.KindHuman, authz.PlanCreate, E, otherEnv).Allowed)
	d := can(dev, authz.KindHuman, authz.PlanApply, E, o.ProdEnvID)
	require.True(t, d.Allowed)
	require.True(t, d.RequiresApproval, "developer apply on production needs approval")
	require.False(t, can(dev, authz.KindHuman, authz.SecretRead, E, o.ProdEnvID).Allowed)
	require.False(t, can(dev, authz.KindHuman, authz.PlanApprove, E, o.DevEnvID).Allowed)
	require.True(t, can(dev, authz.KindAgent, authz.PlanApply, E, o.ProdEnvID).RequiresApproval)

	// Project-level viewer: reads below the project, cannot write, has nothing in the other project.
	require.True(t, can(viewer, authz.KindHuman, authz.PlanRead, E, o.ProdEnvID).Allowed)
	require.False(t, can(viewer, authz.KindHuman, authz.PlanCreate, E, o.ProdEnvID).Allowed)
	require.False(t, can(viewer, authz.KindHuman, authz.PlanRead, E, otherEnv).Allowed)
	require.False(t, can(viewer, authz.KindHuman, authz.OrganizationRead, authz.ScopeOrganization, o.ID).Allowed, "grants carry down, never up")

	// Workspace-level admin: carries down into its workspace, not into the other one.
	require.True(t, can(wsAdmin, authz.KindHuman, authz.PlanApprove, E, o.ProdEnvID).Allowed)
	require.True(t, can(wsAdmin, authz.KindHuman, authz.SecretRead, E, o.ProdEnvID).Allowed)
	require.False(t, can(wsAdmin, authz.KindHuman, authz.PlanApprove, E, otherEnv).Allowed)

	// Environment-level developer: only that environment.
	require.True(t, can(envDev, authz.KindHuman, authz.ServiceWrite, E, o.DevEnvID).Allowed)
	require.False(t, can(envDev, authz.KindHuman, authz.ServiceWrite, E, o.ProdEnvID).Allowed)
	require.False(t, can(envDev, authz.KindHuman, authz.ProjectRead, authz.ScopeProject, o.ProjectID).Allowed)

	// A scope that does not exist in this tenant is denied, not an error.
	require.False(t, can(dev, authz.KindHuman, authz.PlanRead, E, db.NewID()).Allowed)
}
