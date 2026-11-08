package jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/stretchr/testify/require"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
	"github.com/synapsbranch-ux/yari.run/internal/api"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbtest"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
	"github.com/synapsbranch-ux/yari.run/internal/provider/all"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

type models struct{}

func (models) Models(context.Context) ([]gateway.Model, error) {
	return []gateway.Model{{Alias: "claude-sonnet-5-5"}}, nil
}

type devHeader string

func (d devHeader) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set(authn.DevSubjectHeader, string(d))
		return next(ctx, req)
	}
}
func (devHeader) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (devHeader) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// End to end: create the hierarchy through the API, create a plan, apply it, and watch the real
// River worker (with the fake registry) take it to applied and record the services.
func TestPlanAppliesEndToEndThroughTheAPIAndTheWorker(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cat := catalog.MustLoad()
	registry, err := all.Fake(cat)
	require.NoError(t, err)
	workers, err := jobs.NewWorkers(jobs.Deps{Pool: pool, Registry: registry, Leaser: jobs.FakeLeaser{}})
	require.NoError(t, err)
	worker, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 5}}, Workers: workers, FetchCooldown: 20 * time.Millisecond, FetchPollInterval: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NoError(t, worker.Start(ctx))
	t.Cleanup(func() { _ = worker.Stop(context.Background()) })

	inserter, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	require.NoError(t, err)
	srv := httptest.NewServer(api.New(api.Deps{
		Pool: pool, Config: config.Config{Env: "dev", AgentRuntime: "fake"}, Authn: authn.New(pool, authn.Config{Env: "dev"}),
		River: inserter, Catalog: cat, Models: models{},
	}).Handler())
	defer srv.Close()

	subject := "e2e-" + db.NewID().String()
	opt := connect.WithInterceptors(devHeader(subject))
	hier := yariv1connect.NewHierarchyServiceClient(http.DefaultClient, srv.URL, opt)
	planSvc := yariv1connect.NewPlanServiceClient(http.DefaultClient, srv.URL, opt)

	org, err := hier.CreateOrganization(ctx, connect.NewRequest(&yariv1.CreateOrganizationRequest{Slug: "e2e-" + db.NewID().String()[24:], Name: "E2E"}))
	require.NoError(t, err)
	ws, err := hier.CreateWorkspace(ctx, connect.NewRequest(&yariv1.CreateWorkspaceRequest{OrganizationId: org.Msg.Organization.Id, Slug: "main", Name: "Main"}))
	require.NoError(t, err)
	proj, err := hier.CreateProject(ctx, connect.NewRequest(&yariv1.CreateProjectRequest{WorkspaceId: ws.Msg.Workspace.Id, Slug: "shop", Name: "Shop"}))
	require.NoError(t, err)
	env, err := hier.CreateEnvironment(ctx, connect.NewRequest(&yariv1.CreateEnvironmentRequest{ProjectId: proj.Msg.Project.Id, Slug: "staging", Name: "Staging"}))
	require.NoError(t, err)

	created, err := planSvc.CreatePlan(ctx, connect.NewRequest(&yariv1.CreatePlanRequest{
		EnvironmentId: env.Msg.Environment.Id,
		Changes: []*yariv1.PlanChange{
			{Service: "web", Action: yariv1.ChangeAction_CHANGE_ACTION_CREATE, After: &yariv1.ServiceSpec{
				Name: "web", Kind: "web", Provider: "vercel",
				Env: map[string]*yariv1.EnvValue{"DATABASE_URL": {FromOutput: &yariv1.OutputRef{Service: "db", Key: "DATABASE_URL"}}},
			}},
			{Service: "db", Action: yariv1.ChangeAction_CHANGE_ACTION_CREATE, After: &yariv1.ServiceSpec{Name: "db", Kind: "postgres", Provider: "supabase"}},
		},
	}))
	require.NoError(t, err)

	applied, err := planSvc.ApplyPlan(ctx, connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: created.Msg.Plan.Id}))
	require.NoError(t, err)
	require.Equal(t, yariv1.PlanStatus_PLAN_STATUS_APPLYING, applied.Msg.Plan.Status, "staging is not protected, so no approval is needed")

	var final *yariv1.Plan
	require.Eventually(t, func() bool {
		got, err := planSvc.GetPlan(ctx, connect.NewRequest(&yariv1.GetPlanRequest{PlanId: created.Msg.Plan.Id}))
		require.NoError(t, err)
		final = got.Msg.Plan
		return final.Status == yariv1.PlanStatus_PLAN_STATUS_APPLIED || final.Status == yariv1.PlanStatus_PLAN_STATUS_FAILED
	}, 15*time.Second, 100*time.Millisecond)
	require.Equal(t, yariv1.PlanStatus_PLAN_STATUS_APPLIED, final.Status)

	// The services were recorded for the environment, and the audit log has the outcome.
	services, err := hier.ListServices(ctx, connect.NewRequest(&yariv1.ListServicesRequest{EnvironmentId: env.Msg.Environment.Id}))
	require.NoError(t, err)
	got := map[string]string{}
	for _, s := range services.Msg.Services {
		got[s.Name] = s.Provider + "/" + s.Kind + "/" + s.Status
	}
	require.Equal(t, map[string]string{"web": "vercel/web/online", "db": "supabase/postgres/online"}, got)

	audit := yariv1connect.NewAuditServiceClient(http.DefaultClient, srv.URL, opt)
	events, err := audit.ListAuditEvents(ctx, connect.NewRequest(&yariv1.ListAuditEventsRequest{OrganizationId: org.Msg.Organization.Id}))
	require.NoError(t, err)
	var actions []string
	for _, e := range events.Msg.Events {
		actions = append(actions, e.Action)
	}
	require.Contains(t, actions, "plan.applied")
	require.Contains(t, actions, "plan.apply")
}
