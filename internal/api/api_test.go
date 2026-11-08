package api_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/stretchr/testify/require"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
	"github.com/synapsbranch-ux/yari.run/internal/api"
	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbtest"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

type models struct{}

func (models) Models(context.Context) ([]gateway.Model, error) {
	return []gateway.Model{{Alias: "claude-sonnet-5-5", DisplayName: "Claude Sonnet 5.5", Provider: "anthropic"}}, nil
}

type harness struct {
	t    *testing.T
	pool *pgxpool.Pool
	url  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := dbtest.Pool(t)
	rc, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	require.NoError(t, err)
	cfg := config.Config{Env: "dev", AgentRuntime: "fake", AgentDefaultModel: "claude-sonnet-5-5", AgentSessionBudget: 2, AgentMaxIterations: 20, AgentSessionTTL: 30 * time.Minute}
	srv := api.New(api.Deps{
		Pool: pool, Config: cfg, Authn: authn.New(pool, authn.Config{Env: "dev"}), River: rc,
		Catalog: catalog.MustLoad(), Models: models{}, WatchPoll: 20 * time.Millisecond,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &harness{t: t, pool: pool, url: ts.URL}
}

// auth adds either a dev subject or a bearer token to every call.
type auth struct{ headers map[string]string }

type authInterceptor struct{ headers map[string]string }

func (a auth) intercept() connect.Interceptor { return authInterceptor{a.headers} }

func (i authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		for k, v := range i.headers {
			req.Header().Set(k, v)
		}
		return next(ctx, req)
	}
}

func (i authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		for k, v := range i.headers {
			conn.RequestHeader().Set(k, v)
		}
		return conn
	}
}

func (i authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func asUser(subject string) auth {
	return auth{headers: map[string]string{authn.DevSubjectHeader: strings.TrimPrefix(subject, "dev:")}}
}
func asToken(tok string) auth {
	return auth{headers: map[string]string{"Authorization": "Bearer " + tok}}
}

type clients struct {
	Identity  yariv1connect.IdentityServiceClient
	Hierarchy yariv1connect.HierarchyServiceClient
	Access    yariv1connect.AccessServiceClient
	Plan      yariv1connect.PlanServiceClient
	Audit     yariv1connect.AuditServiceClient
	Catalog   yariv1connect.CatalogServiceClient
	Agent     yariv1connect.AgentServiceClient
}

func (h *harness) as(a auth) clients {
	opt := connect.WithInterceptors(a.intercept())
	c := http.DefaultClient
	return clients{
		Identity:  yariv1connect.NewIdentityServiceClient(c, h.url, opt),
		Hierarchy: yariv1connect.NewHierarchyServiceClient(c, h.url, opt),
		Access:    yariv1connect.NewAccessServiceClient(c, h.url, opt),
		Plan:      yariv1connect.NewPlanServiceClient(c, h.url, opt),
		Audit:     yariv1connect.NewAuditServiceClient(c, h.url, opt),
		Catalog:   yariv1connect.NewCatalogServiceClient(c, h.url, opt),
		Agent:     yariv1connect.NewAgentServiceClient(c, h.url, opt),
	}
}

func code(err error) connect.Code { return connect.CodeOf(err) }

func webChange(name, provider string) *yariv1.PlanChange {
	return &yariv1.PlanChange{Service: name, Action: yariv1.ChangeAction_CHANGE_ACTION_CREATE,
		After: &yariv1.ServiceSpec{Name: name, Kind: "web", Provider: provider}}
}

func countJobs(t *testing.T, pool *pgxpool.Pool, kind, argKey, argVal string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM river_job WHERE kind = $1 AND args ->> $2 = $3`, kind, argKey, argVal).Scan(&n))
	return n
}

func auditActions(t *testing.T, pool *pgxpool.Pool, org uuid.UUID) []string {
	t.Helper()
	var out []string
	require.NoError(t, db.WithTenant(context.Background(), pool, org, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := dbsqlc.New(tx).ListAuditEvents(ctx, dbsqlc.ListAuditEventsParams{PageSize: 500})
		for _, r := range rows {
			out = append(out, r.Action)
		}
		return err
	}))
	return out
}

func TestCreateOrganizationGrantsOwnerAndAudits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	subject := "dev:owner-" + db.NewID().String()
	c := h.as(asUser(subject))

	slug := "org-" + db.NewID().String()[24:]
	resp, err := c.Hierarchy.CreateOrganization(ctx, connect.NewRequest(&yariv1.CreateOrganizationRequest{Slug: slug, Name: "Acme"}))
	require.NoError(t, err)
	org := uuid.MustParse(resp.Msg.Organization.Id)

	// The creator is an owner: it can check organization.manage.
	chk, err := c.Access.CheckPermission(ctx, connect.NewRequest(&yariv1.CheckPermissionRequest{
		Permission: "organization.manage", Scope: &yariv1.Scope{Type: yariv1.ScopeType_SCOPE_TYPE_ORGANIZATION, Id: org.String()}}))
	require.NoError(t, err)
	require.True(t, chk.Msg.Decision.Allowed, chk.Msg.Decision.Reason)

	grants, err := c.Access.ListGrants(ctx, connect.NewRequest(&yariv1.ListGrantsRequest{OrganizationId: org.String()}))
	require.NoError(t, err)
	require.Len(t, grants.Msg.Grants, 1)
	require.Equal(t, yariv1.Role_ROLE_OWNER, grants.Msg.Grants[0].Role)

	require.Contains(t, auditActions(t, h.pool, org), "organization.create")

	// Duplicate slug is rejected, and invalid slugs are too.
	_, err = c.Hierarchy.CreateOrganization(ctx, connect.NewRequest(&yariv1.CreateOrganizationRequest{Slug: slug, Name: "Again"}))
	require.Equal(t, connect.CodeAlreadyExists, code(err))
	_, err = c.Hierarchy.CreateOrganization(ctx, connect.NewRequest(&yariv1.CreateOrganizationRequest{Slug: "Bad Slug", Name: "x"}))
	require.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestUnauthenticatedAndPermissionDenied(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	viewer := dbtest.AddMember(t, h.pool, o, "viewer", "organization", o.ID)

	_, err := h.as(auth{}).Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, code(err))

	// A viewer can read but not create.
	vc := h.as(asUser(viewer.Subject))
	_, err = vc.Hierarchy.ListWorkspaces(ctx, connect.NewRequest(&yariv1.ListWorkspacesRequest{OrganizationId: o.ID.String()}))
	require.NoError(t, err)
	_, err = vc.Hierarchy.CreateWorkspace(ctx, connect.NewRequest(&yariv1.CreateWorkspaceRequest{OrganizationId: o.ID.String(), Slug: "x", Name: "X"}))
	require.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = vc.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{EnvironmentId: o.DevEnvID.String(), Changes: []*yariv1.PlanChange{webChange("web", "railway")}}))
	require.Equal(t, connect.CodePermissionDenied, code(err))

	// A stranger is not a member of the organization at all.
	stranger := h.as(asUser("dev:stranger-" + db.NewID().String()))
	_, err = stranger.Hierarchy.ListWorkspaces(ctx, connect.NewRequest(&yariv1.ListWorkspacesRequest{OrganizationId: o.ID.String()}))
	require.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = stranger.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{EnvironmentId: o.DevEnvID.String(), Changes: []*yariv1.PlanChange{webChange("web", "railway")}}))
	require.Equal(t, connect.CodeNotFound, code(err), "ids of other tenants must not be revealed")
}

func TestDeveloperApplyOnProductionGoesToPendingApprovalWithoutJob(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	dev := dbtest.AddMember(t, h.pool, o, "developer", "organization", o.ID)
	dc := h.as(asUser(dev.Subject))
	owner := h.as(asUser(o.Subject))

	created, err := dc.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{
		EnvironmentId: o.ProdEnvID.String(),
		Changes:       []*yariv1.PlanChange{webChange("web", "railway")},
	}))
	require.NoError(t, err)
	planID := created.Msg.Plan.Id
	require.Equal(t, yariv1.PlanStatus_PLAN_STATUS_DRAFT, created.Msg.Plan.Status)
	require.Equal(t, int64(500), created.Msg.Plan.CostDeltaCents)
	require.Equal(t, yariv1.Risk_RISK_LOW, created.Msg.Plan.Risk)

	applied, err := dc.Plan.ApplyPlan(ctx, connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: planID}))
	require.NoError(t, err)
	require.True(t, applied.Msg.RequiresApproval)
	require.Equal(t, yariv1.PlanStatus_PLAN_STATUS_PENDING_APPROVAL, applied.Msg.Plan.Status)
	require.Zero(t, countJobs(t, h.pool, "apply_plan", "plan_id", planID), "no job before approval")

	// The developer cannot approve, and it does not see the plan in the approvals inbox.
	_, err = dc.Plan.ApprovePlan(ctx, connect.NewRequest(&yariv1.ApprovePlanRequest{PlanId: planID}))
	require.Equal(t, connect.CodePermissionDenied, code(err))
	pending, err := dc.Plan.ListPendingApprovals(ctx, connect.NewRequest(&yariv1.ListPendingApprovalsRequest{}))
	require.NoError(t, err)
	require.Empty(t, pending.Msg.Plans)

	// The owner sees it and approves.
	pending, err = owner.Plan.ListPendingApprovals(ctx, connect.NewRequest(&yariv1.ListPendingApprovalsRequest{}))
	require.NoError(t, err)
	require.Len(t, pending.Msg.Plans, 1)
	_, err = owner.Plan.ApprovePlan(ctx, connect.NewRequest(&yariv1.ApprovePlanRequest{PlanId: planID}))
	require.NoError(t, err)
	require.Zero(t, countJobs(t, h.pool, "apply_plan", "plan_id", planID), "approval alone does not apply")

	applied, err = dc.Plan.ApplyPlan(ctx, connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: planID}))
	require.NoError(t, err)
	require.False(t, applied.Msg.RequiresApproval)
	require.Equal(t, yariv1.PlanStatus_PLAN_STATUS_APPLYING, applied.Msg.Plan.Status)
	require.Equal(t, 1, countJobs(t, h.pool, "apply_plan", "plan_id", planID))

	acts := auditActions(t, h.pool, o.ID)
	for _, want := range []string{"plan.create", "plan.apply.requested", "plan.approve", "plan.apply"} {
		require.Contains(t, acts, want)
	}
}

func TestPlanCannotBeApprovedByItsCreator(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	dev := dbtest.AddMember(t, h.pool, o, "developer", "organization", o.ID)
	admin := dbtest.AddMember(t, h.pool, o, "admin", "organization", o.ID)
	dc, ac := h.as(asUser(dev.Subject)), h.as(asUser(admin.Subject))

	// An admin creates a plan on an environment that needs a second approver.
	env, err := ac.Hierarchy.CreateEnvironment(ctx, connect.NewRequest(&yariv1.CreateEnvironmentRequest{ProjectId: o.ProjectID.String(), Slug: "audited", Name: "Audited", RequireSecondApprover: true}))
	require.NoError(t, err)
	cp, err := ac.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{EnvironmentId: env.Msg.Environment.Id, Changes: []*yariv1.PlanChange{webChange("web", "vercel")}}))
	require.NoError(t, err)
	ap, err := ac.Plan.ApplyPlan(ctx, connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: cp.Msg.Plan.Id}))
	require.NoError(t, err)
	require.True(t, ap.Msg.RequiresApproval, "require_second_approver applies to admins too")

	_, err = ac.Plan.ApprovePlan(ctx, connect.NewRequest(&yariv1.ApprovePlanRequest{PlanId: cp.Msg.Plan.Id}))
	require.Equal(t, connect.CodePermissionDenied, code(err), "own plan")
	_, err = dc.Plan.ApprovePlan(ctx, connect.NewRequest(&yariv1.ApprovePlanRequest{PlanId: cp.Msg.Plan.Id}))
	require.Equal(t, connect.CodePermissionDenied, code(err), "developers cannot approve")
	_, err = h.as(asUser(o.Subject)).Plan.ApprovePlan(ctx, connect.NewRequest(&yariv1.ApprovePlanRequest{PlanId: cp.Msg.Plan.Id}))
	require.NoError(t, err, "a second principal with plan.approve can")
}

func TestProductionEnvironmentIsProtectedAutomatically(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	c := h.as(asUser(o.Subject))
	prod, err := c.Hierarchy.CreateEnvironment(ctx, connect.NewRequest(&yariv1.CreateEnvironmentRequest{ProjectId: o.ProjectID.String(), Slug: "production", Name: "Prod", Protected: false}))
	require.Error(t, err, "the seeded project already has production")
	require.Nil(t, prod)
	p2, err := c.Hierarchy.CreateProject(ctx, connect.NewRequest(&yariv1.CreateProjectRequest{WorkspaceId: o.WorkspaceID.String(), Slug: "second", Name: "Second"}))
	require.NoError(t, err)
	prod, err = c.Hierarchy.CreateEnvironment(ctx, connect.NewRequest(&yariv1.CreateEnvironmentRequest{ProjectId: p2.Msg.Project.Id, Slug: "production", Name: "Prod"}))
	require.NoError(t, err)
	require.True(t, prod.Msg.Environment.Protected)
}

func TestPlanValidation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	c := h.as(asUser(o.Subject))
	create := func(changes ...*yariv1.PlanChange) error {
		_, err := c.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{EnvironmentId: o.DevEnvID.String(), Changes: changes}))
		return err
	}
	require.Equal(t, connect.CodeInvalidArgument, code(create()), "empty plan")
	require.Equal(t, connect.CodeInvalidArgument, code(create(webChange("web", "no-such-provider"))))
	bad := webChange("db", "railway")
	bad.After.Kind = "bucket" // railway cannot run buckets
	require.Equal(t, connect.CodeInvalidArgument, code(create(bad)))

	secret := webChange("web", "railway")
	secret.After.Env = map[string]*yariv1.EnvValue{"API_KEY": {Value: "super-secret", Secret: true}}
	require.Equal(t, connect.CodeInvalidArgument, code(create(secret)), "secret literals are refused")

	cycle1, cycle2 := webChange("a", "railway"), webChange("b", "railway")
	cycle1.After.Env = map[string]*yariv1.EnvValue{"B": {FromOutput: &yariv1.OutputRef{Service: "b", Key: "URL"}}}
	cycle2.After.Env = map[string]*yariv1.EnvValue{"A": {FromOutput: &yariv1.OutputRef{Service: "a", Key: "URL"}}}
	require.Equal(t, connect.CodeInvalidArgument, code(create(cycle1, cycle2)), "cycles are refused")

	ref := webChange("web", "railway")
	ref.After.Env = map[string]*yariv1.EnvValue{"DB": {FromOutput: &yariv1.OutputRef{Service: "ghost", Key: "URL"}}}
	require.Equal(t, connect.CodeInvalidArgument, code(create(ref)), "unknown refs are refused")
}

func TestStartSessionCreatesEverythingInOneTransaction(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	c := h.as(asUser(o.Subject))

	_, err := c.Agent.StartSession(ctx, connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: o.ProdEnvID.String(), Goal: "deploy", Model: "not-a-model"}))
	require.Equal(t, connect.CodeInvalidArgument, code(err), "only gateway models are accepted")

	resp, err := c.Agent.StartSession(ctx, connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: o.ProdEnvID.String(), Goal: "make the build pass"}))
	require.NoError(t, err)
	s := resp.Msg.Session
	require.Equal(t, yariv1.SessionStatus_SESSION_STATUS_QUEUED, s.Status)
	require.Equal(t, "fake", s.Runtime)
	require.Equal(t, "claude-sonnet-5-5", s.Model)
	require.Equal(t, int64(2_000_000), s.BudgetUsdMicros)
	sessionID := uuid.MustParse(s.Id)

	// Session, agent principal, developer grant, token and job all exist.
	require.NoError(t, db.WithTenant(ctx, h.pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		sess, err := q.GetAgentSession(ctx, sessionID)
		require.NoError(t, err)
		agent, err := q.GetPrincipal(ctx, sess.AgentPrincipalID)
		require.NoError(t, err)
		require.Equal(t, "agent", agent.Kind)
		require.Equal(t, o.PrincipalID, sess.StartedBy)

		grants, err := q.ListGrants(ctx, uuid.NullUUID{UUID: agent.ID, Valid: true})
		require.NoError(t, err)
		require.Len(t, grants, 1)
		require.Equal(t, "developer", grants[0].Role)
		require.Equal(t, "environment", grants[0].ScopeType)
		require.Equal(t, o.ProdEnvID, grants[0].ScopeID)

		var label string
		var expires time.Time
		require.NoError(t, tx.QueryRow(ctx, `SELECT label, expires_at FROM api_tokens WHERE principal_id = $1`, agent.ID).Scan(&label, &expires))
		require.Equal(t, api.SessionTokenLabel(sessionID), label)
		require.WithinDuration(t, time.Now().Add(30*time.Minute), expires, time.Minute, "token expiry equals the session time limit")
		return nil
	}))
	require.Equal(t, 1, countJobs(t, h.pool, "run_agent_session", "session_id", sessionID.String()))

	// The token plaintext lives only in the job args and is a real, working yari token.
	var token string
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT args ->> 'mcp_token' FROM river_job WHERE kind = 'run_agent_session' AND args ->> 'session_id' = $1`, sessionID.String()).Scan(&token))
	require.True(t, strings.HasPrefix(token, authn.TokenPrefix))
	who, err := h.as(asToken(token)).Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.NoError(t, err)
	require.Equal(t, yariv1.PrincipalKind_PRINCIPAL_KIND_AGENT, who.Msg.Memberships[0].PrincipalKind)

	// Starting a second session reuses the workspace's agent principal.
	resp2, err := c.Agent.StartSession(ctx, connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: o.DevEnvID.String(), Goal: "second"}))
	require.NoError(t, err)
	require.Equal(t, s.AgentPrincipalId, resp2.Msg.Session.AgentPrincipalId)

	// An agent cannot start agent sessions.
	_, err = h.as(asToken(token)).Agent.StartSession(ctx, connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: o.ProdEnvID.String(), Goal: "recursion"}))
	require.Equal(t, connect.CodePermissionDenied, code(err))

	// Stopping a queued session ends it at once and kills its token.
	stopped, err := c.Agent.StopSession(ctx, connect.NewRequest(&yariv1.StopSessionRequest{SessionId: s.Id}))
	require.NoError(t, err)
	require.Equal(t, yariv1.SessionStatus_SESSION_STATUS_STOPPED, stopped.Msg.Session.Status)
	_, err = h.as(asToken(token)).Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, code(err))
}

func TestAPITokenAuth(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	owner := h.as(asUser(o.Subject))

	created, err := owner.Identity.CreateApiToken(ctx, connect.NewRequest(&yariv1.CreateApiTokenRequest{OrganizationId: o.ID.String(), Label: "ci", TtlSeconds: 3600}))
	require.NoError(t, err)
	tok := created.Msg.Token
	require.True(t, strings.HasPrefix(tok, "yari_"))

	// Only the hash is stored.
	sum := sha256.Sum256([]byte(tok))
	var n int
	require.NoError(t, db.WithTenant(ctx, h.pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE token_hash = $1 AND prefix = $2`, sum[:], created.Msg.Prefix).Scan(&n)
	}))
	require.Equal(t, 1, n)

	tc := h.as(asToken(tok))
	who, err := tc.Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.NoError(t, err)
	require.Len(t, who.Msg.Memberships, 1)
	require.Equal(t, o.ID.String(), who.Msg.Memberships[0].OrganizationId)

	// A token only works in its own organization.
	other := dbtest.SeedOrg(t, h.pool)
	_, err = tc.Hierarchy.ListWorkspaces(ctx, connect.NewRequest(&yariv1.ListWorkspacesRequest{OrganizationId: other.ID.String()}))
	require.Equal(t, connect.CodePermissionDenied, code(err))

	// Revoked tokens fail.
	_, err = owner.Identity.RevokeApiToken(ctx, connect.NewRequest(&yariv1.RevokeApiTokenRequest{OrganizationId: o.ID.String(), TokenId: created.Msg.TokenId}))
	require.NoError(t, err)
	_, err = tc.Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, code(err))

	// Expired tokens fail.
	plain, prefix, hash, err := authn.GenerateToken()
	require.NoError(t, err)
	require.NoError(t, db.WithTenant(ctx, h.pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := dbsqlc.New(tx).CreateAPIToken(ctx, dbsqlc.CreateAPITokenParams{ID: db.NewID(), OrganizationID: o.ID, PrincipalID: o.PrincipalID, TokenHash: hash, Prefix: prefix, ExpiresAt: time.Now().Add(-time.Second)})
		return err
	}))
	_, err = h.as(asToken(plain)).Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, code(err))

	// An unknown token fails, and the dev header is ignored outside the dev environment.
	_, err = h.as(asToken("yari_nope")).Identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, code(err))
	prodAuthn := authn.New(h.pool, authn.Config{Env: "prod"})
	_, err = prodAuthn.Authenticate(ctx, http.Header{authn.DevSubjectHeader: []string{"someone"}})
	require.Error(t, err)
}

func TestAgentCannotDeleteDataStores(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	owner := h.as(asUser(o.Subject))
	_, err := owner.Agent.StartSession(ctx, connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: o.ProdEnvID.String(), Goal: "clean up"}))
	require.NoError(t, err)
	var token, sessionID string
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT args ->> 'mcp_token', args ->> 'session_id' FROM river_job WHERE kind = 'run_agent_session' AND args ->> 'organization_id' = $1`, o.ID.String()).Scan(&token, &sessionID))
	agent := h.as(asToken(token))
	require.NoError(t, db.WithTenant(ctx, h.pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := dbsqlc.New(tx).MarkAgentSessionRunning(ctx, dbsqlc.MarkAgentSessionRunningParams{ID: uuid.MustParse(sessionID), GatewayKeyAlias: pgText("alias")})
		return err
	}))

	for _, kind := range []string{"postgres", "mysql", "mongodb", "sqlite", "redis", "bucket"} {
		_, err := agent.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{
			EnvironmentId: o.ProdEnvID.String(), AgentSessionId: sessionID,
			Changes: []*yariv1.PlanChange{{Service: "data", Action: yariv1.ChangeAction_CHANGE_ACTION_DELETE, After: &yariv1.ServiceSpec{Name: "data", Kind: kind, Provider: "railway"}}},
		}))
		require.Equal(t, connect.CodePermissionDenied, code(err), kind)
		require.Contains(t, err.Error(), api.GuardrailPrefix)
	}
	require.Contains(t, auditActions(t, h.pool, o.ID), "plan.refused", "the refusal is recorded")

	// Deleting a stateless service is fine, and the human owner is not restricted by the deny list.
	_, err = agent.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{
		EnvironmentId: o.ProdEnvID.String(), AgentSessionId: sessionID,
		Changes: []*yariv1.PlanChange{{Service: "web", Action: yariv1.ChangeAction_CHANGE_ACTION_DELETE, After: &yariv1.ServiceSpec{Name: "web", Kind: "web", Provider: "railway"}}},
	}))
	require.NoError(t, err)
	_, err = owner.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{
		EnvironmentId: o.ProdEnvID.String(),
		Changes:       []*yariv1.PlanChange{{Service: "data", Action: yariv1.ChangeAction_CHANGE_ACTION_DELETE, After: &yariv1.ServiceSpec{Name: "data", Kind: "postgres", Provider: "railway"}}},
	}))
	require.NoError(t, err)

	// An agent applying on production only reaches pending_approval.
	plans, err := agent.Plan.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{EnvironmentId: o.ProdEnvID.String(), AgentSessionId: sessionID, Changes: []*yariv1.PlanChange{webChange("api", "railway")}}))
	require.NoError(t, err)
	applied, err := agent.Plan.ApplyPlan(ctx, connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: plans.Msg.Plan.Id}))
	require.NoError(t, err)
	require.True(t, applied.Msg.RequiresApproval)
	require.Zero(t, countJobs(t, h.pool, "apply_plan", "plan_id", plans.Msg.Plan.Id))
	_, err = agent.Plan.ApprovePlan(ctx, connect.NewRequest(&yariv1.ApprovePlanRequest{PlanId: plans.Msg.Plan.Id}))
	require.Equal(t, connect.CodePermissionDenied, code(err), "agents never approve")
}

func TestCatalogAndModels(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	c := h.as(asUser("dev:reader-" + db.NewID().String()))
	all, err := c.Catalog.ListProviders(ctx, connect.NewRequest(&yariv1.ListProvidersRequest{}))
	require.NoError(t, err)
	require.NotEmpty(t, all.Msg.Providers)
	pg, err := c.Catalog.ListProviders(ctx, connect.NewRequest(&yariv1.ListProvidersRequest{Kind: "postgres"}))
	require.NoError(t, err)
	require.NotEmpty(t, pg.Msg.Providers)
	for _, p := range pg.Msg.Providers {
		require.Contains(t, p.Kinds, "postgres")
		require.True(t, strings.HasPrefix(p.DocsUrl, "https://"))
	}
	_, err = c.Catalog.ListProviders(ctx, connect.NewRequest(&yariv1.ListProvidersRequest{Kind: "bogus"}))
	require.Equal(t, connect.CodeInvalidArgument, code(err))
	m, err := c.Agent.ListModels(ctx, connect.NewRequest(&yariv1.ListModelsRequest{}))
	require.NoError(t, err)
	require.Len(t, m.Msg.Models, 1)
}

func TestAuditPagination(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	c := h.as(asUser(o.Subject))
	for i := 0; i < 5; i++ {
		_, err := c.Hierarchy.CreateProject(ctx, connect.NewRequest(&yariv1.CreateProjectRequest{WorkspaceId: o.WorkspaceID.String(), Slug: "p-" + strings.Repeat("x", i+1), Name: "P"}))
		require.NoError(t, err)
	}
	page1, err := c.Audit.ListAuditEvents(ctx, connect.NewRequest(&yariv1.ListAuditEventsRequest{OrganizationId: o.ID.String(), PageSize: 3}))
	require.NoError(t, err)
	require.Len(t, page1.Msg.Events, 3)
	require.NotEmpty(t, page1.Msg.NextPageToken)
	page2, err := c.Audit.ListAuditEvents(ctx, connect.NewRequest(&yariv1.ListAuditEventsRequest{OrganizationId: o.ID.String(), PageSize: 3, PageToken: page1.Msg.NextPageToken}))
	require.NoError(t, err)
	require.Len(t, page2.Msg.Events, 2)
	require.Empty(t, page2.Msg.NextPageToken)
	require.Greater(t, page1.Msg.Events[2].Id, page2.Msg.Events[0].Id, "newest first")

	// Developers cannot read the audit log.
	dev := dbtest.AddMember(t, h.pool, o, "developer", "organization", o.ID)
	_, err = h.as(asUser(dev.Subject)).Audit.ListAuditEvents(ctx, connect.NewRequest(&yariv1.ListAuditEventsRequest{OrganizationId: o.ID.String()}))
	require.Equal(t, connect.CodePermissionDenied, code(err))
}

func TestWatchSessionStreamsStepsUntilTheSessionEnds(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	o := dbtest.SeedOrg(t, h.pool)
	c := h.as(asUser(o.Subject))
	resp, err := c.Agent.StartSession(ctx, connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: o.DevEnvID.String(), Goal: "watch me"}))
	require.NoError(t, err)
	sid := uuid.MustParse(resp.Msg.Session.Id)

	go func() { // plays the worker: writes steps, then ends the session
		time.Sleep(60 * time.Millisecond)
		for i := 0; i < 3; i++ {
			_ = db.WithTenant(ctx, h.pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
				_, err := audit.AppendStep(ctx, dbsqlc.New(tx), o.ID, sid, audit.StepInput{Kind: "action", Tool: "terminal", Location: "sandbox", Summary: "step", RawEvent: []byte(`{}`)})
				return err
			})
			time.Sleep(30 * time.Millisecond)
		}
		_ = db.WithTenant(ctx, h.pool, o.ID, func(ctx context.Context, tx pgx.Tx) error {
			_, err := dbsqlc.New(tx).FinishAgentSession(ctx, dbsqlc.FinishAgentSessionParams{ID: sid, Status: "succeeded"})
			return err
		})
	}()

	stream, err := c.Agent.WatchSession(ctx, connect.NewRequest(&yariv1.WatchSessionRequest{SessionId: sid.String()}))
	require.NoError(t, err)
	var seqs []int32
	var last yariv1.SessionStatus
	for stream.Receive() {
		if st := stream.Msg().Step; st != nil {
			seqs = append(seqs, st.Seq)
		}
		last = stream.Msg().Status
	}
	require.NoError(t, stream.Err())
	require.Equal(t, []int32{1, 2, 3}, seqs)
	require.Equal(t, yariv1.SessionStatus_SESSION_STATUS_SUCCEEDED, last)
}

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// A user who belongs to several organizations can still address resources by id: the API has to
// look through the user's organizations until it finds the owner of the id.
func TestResourcesAreFoundAcrossAUsersOrganizations(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	subject := "dev:multi-" + db.NewID().String()
	c := h.as(asUser(subject))
	var projects []string
	for i := 0; i < 3; i++ {
		org, err := c.Hierarchy.CreateOrganization(ctx, connect.NewRequest(&yariv1.CreateOrganizationRequest{Slug: "multi-" + db.NewID().String()[24:], Name: "Multi"}))
		require.NoError(t, err)
		ws, err := c.Hierarchy.CreateWorkspace(ctx, connect.NewRequest(&yariv1.CreateWorkspaceRequest{OrganizationId: org.Msg.Organization.Id, Slug: "main", Name: "Main"}))
		require.NoError(t, err)
		pr, err := c.Hierarchy.CreateProject(ctx, connect.NewRequest(&yariv1.CreateProjectRequest{WorkspaceId: ws.Msg.Workspace.Id, Slug: "shop", Name: "Shop"}))
		require.NoError(t, err, "organization %d: the workspace is not in the first organization searched", i)
		projects = append(projects, pr.Msg.Project.Id)
	}
	// Every project can be reached again by id, wherever it lives.
	for _, id := range projects {
		_, err := c.Hierarchy.CreateEnvironment(ctx, connect.NewRequest(&yariv1.CreateEnvironmentRequest{ProjectId: id, Slug: "staging", Name: "Staging"}))
		require.NoError(t, err)
	}
	_, err := c.Hierarchy.CreateEnvironment(ctx, connect.NewRequest(&yariv1.CreateEnvironmentRequest{ProjectId: db.NewID().String(), Slug: "x", Name: "X"}))
	require.Equal(t, connect.CodeNotFound, code(err), "an id that exists nowhere is NotFound, not an internal error")
}
