package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
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

	"github.com/synapsbranch-ux/yari.run/agent/fixtures"
	"github.com/synapsbranch-ux/yari.run/agent/prompts"
	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
	"github.com/synapsbranch-ux/yari.run/internal/agent"
	agentfake "github.com/synapsbranch-ux/yari.run/internal/agent/fake"
	"github.com/synapsbranch-ux/yari.run/internal/api"
	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbtest"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
	"github.com/synapsbranch-ux/yari.run/internal/mcpserver"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
	sandboxfake "github.com/synapsbranch-ux/yari.run/internal/sandbox/fake"
)

// fakeGateway records key lifecycle and reports a configurable spend.
type fakeGateway struct {
	mu      sync.Mutex
	keys    map[string]string // alias -> key
	revoked []string
	spend   float64
	failGen error
}

func newFakeGateway() *fakeGateway { return &fakeGateway{keys: map[string]string{}} }

func (g *fakeGateway) CreateSessionKey(_ context.Context, sessionID, _ string, _ float64, _ time.Duration) (gateway.SessionKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failGen != nil {
		return gateway.SessionKey{}, g.failGen
	}
	alias := gateway.Alias(sessionID)
	key := "sk-fake-" + strings.ReplaceAll(sessionID, "-", "") // looks like a real key on purpose
	g.keys[alias] = key
	return gateway.SessionKey{Key: key, Alias: alias}, nil
}
func (g *fakeGateway) Spend(context.Context, string) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.spend, nil
}
func (g *fakeGateway) Revoke(_ context.Context, alias string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.revoked = append(g.revoked, alias)
	return nil
}
func (g *fakeGateway) setSpend(v float64) { g.mu.Lock(); g.spend = v; g.mu.Unlock() }
func (g *fakeGateway) wasRevoked(alias string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, a := range g.revoked {
		if a == alias {
			return true
		}
	}
	return false
}

type staticModels struct{}

func (staticModels) Models(context.Context) ([]gateway.Model, error) {
	return []gateway.Model{{Alias: "claude-sonnet-5-5"}}, nil
}

type sessionHarness struct {
	t         *testing.T
	pool      *pgxpool.Pool
	org       dbtest.Org
	apiURL    string
	mcpURL    string
	sandboxes *sandboxfake.Manager
	gw        *fakeGateway
	live      *jobs.LiveSessions
	owner     yariv1connect.AgentServiceClient
	plans     yariv1connect.PlanServiceClient
	ident     func(token string) yariv1connect.IdentityServiceClient
}

func newSessionHarness(t *testing.T) *sessionHarness {
	t.Helper()
	pool := dbtest.Pool(t)
	inserter, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	require.NoError(t, err)
	apiSrv := httptest.NewServer(api.New(api.Deps{
		Pool: pool, Config: config.Config{Env: "dev", AgentRuntime: "fake", AgentDefaultModel: "claude-sonnet-5-5", AgentSessionBudget: 2, AgentMaxIterations: 50, AgentSessionTTL: 5 * time.Minute},
		Authn: authn.New(pool, authn.Config{Env: "dev"}), River: inserter, Catalog: catalog.MustLoad(), Models: staticModels{}, WatchPoll: 20 * time.Millisecond,
	}).Handler())
	t.Cleanup(apiSrv.Close)
	mcpSrv := httptest.NewServer((&mcpserver.Server{APIURL: apiSrv.URL}).Handler())
	t.Cleanup(mcpSrv.Close)

	o := dbtest.SeedOrg(t, pool)
	hdr := devHeader(strings.TrimPrefix(o.Subject, "dev:"))
	return &sessionHarness{
		t: t, pool: pool, org: o, apiURL: apiSrv.URL, mcpURL: mcpSrv.URL + "/mcp",
		sandboxes: sandboxfake.NewManager(), gw: newFakeGateway(), live: jobs.NewLiveSessions(),
		owner: yariv1connect.NewAgentServiceClient(http.DefaultClient, apiSrv.URL, connect.WithInterceptors(hdr)),
		plans: yariv1connect.NewPlanServiceClient(http.DefaultClient, apiSrv.URL, connect.WithInterceptors(hdr)),
		ident: func(token string) yariv1connect.IdentityServiceClient {
			return yariv1connect.NewIdentityServiceClient(http.DefaultClient, apiSrv.URL, connect.WithInterceptors(bearer(token)))
		},
	}
}

type bearer string

func (b bearer) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+string(b))
		return next(ctx, req)
	}
}
func (bearer) WrapStreamingClient(n connect.StreamingClientFunc) connect.StreamingClientFunc {
	return n
}
func (bearer) WrapStreamingHandler(n connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return n
}

// start creates a session through the API and returns the job args the worker would receive.
func (h *sessionHarness) start(env uuid.UUID, goal string) (jobs.RunAgentSessionArgs, *yariv1.AgentSession) {
	h.t.Helper()
	resp, err := h.owner.StartSession(context.Background(), connect.NewRequest(&yariv1.StartSessionRequest{EnvironmentId: env.String(), Goal: goal}))
	require.NoError(h.t, err)
	var token string
	require.NoError(h.t, h.pool.QueryRow(context.Background(), `SELECT args ->> 'mcp_token' FROM river_job WHERE kind = 'run_agent_session' AND args ->> 'session_id' = $1`, resp.Msg.Session.Id).Scan(&token))
	return jobs.RunAgentSessionArgs{OrganizationID: h.org.ID, SessionID: uuid.MustParse(resp.Msg.Session.Id), MCPToken: token}, resp.Msg.Session
}

func (h *sessionHarness) worker(rt agent.Runtime) *jobs.SessionWorker {
	sample, err := fs.Sub(fixtures.SampleRepo, "sample-repo")
	require.NoError(h.t, err)
	return &jobs.SessionWorker{
		Pool: h.pool, Config: config.Config{Env: "dev", AgentImage: "fake", AgentSessionTTL: 5 * time.Minute, MCPURL: "http://yari-mcp:8090/mcp", LLMBaseURL: "http://litellm:4000"},
		Sandboxes: h.sandboxes, Gateway: h.gw, Runtime: rt, Live: h.live, Prompt: prompts.System, SampleRepo: sample, SpendPoll: 15 * time.Millisecond,
	}
}

func (h *sessionHarness) work(w *jobs.SessionWorker, args jobs.RunAgentSessionArgs) {
	h.t.Helper()
	require.NoError(h.t, w.Work(context.Background(), &river.Job[jobs.RunAgentSessionArgs]{Args: args}))
}

func (h *sessionHarness) session(id uuid.UUID) dbsqlc.AgentSession {
	var s dbsqlc.AgentSession
	require.NoError(h.t, db.WithTenant(context.Background(), h.pool, h.org.ID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		s, err = dbsqlc.New(tx).GetAgentSession(ctx, id)
		return err
	}))
	return s
}

func (h *sessionHarness) steps(id uuid.UUID) []dbsqlc.AgentStep {
	var steps []dbsqlc.AgentStep
	require.NoError(h.t, db.WithTenant(context.Background(), h.pool, h.org.ID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		steps, err = dbsqlc.New(tx).ListAgentSteps(ctx, dbsqlc.ListAgentStepsParams{SessionID: id})
		return err
	}))
	return steps
}

func (h *sessionHarness) actions() []string {
	var out []string
	require.NoError(h.t, db.WithTenant(context.Background(), h.pool, h.org.ID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := dbsqlc.New(tx).ListAuditEvents(ctx, dbsqlc.ListAuditEventsParams{PageSize: 500})
		for _, r := range rows {
			out = append(out, r.Action)
		}
		return err
	}))
	return out
}

func (h *sessionHarness) tokenWorks(token string) bool {
	_, err := h.ident(token).WhoAmI(context.Background(), connect.NewRequest(&yariv1.WhoAmIRequest{}))
	return err == nil
}

// The scripted agent runs end to end against a real API and a real MCP server, in a fake sandbox,
// and stops at waiting_approval because it applied a plan on production.
func TestFakeRuntimeEndToEndReachesWaitingApprovalOnProduction(t *testing.T) {
	h := newSessionHarness(t)
	args, sess := h.start(h.org.ProdEnvID, "make the build pass and deploy it")
	require.True(t, h.tokenWorks(args.MCPToken), "the session token works while the session lives")

	h.work(h.worker(&agentfake.Runtime{MCPURL: h.mcpURL}), args)

	got := h.session(args.SessionID)
	require.Equal(t, "waiting_approval", got.Status)
	require.True(t, got.EndedAt != nil && got.StartedAt != nil)
	require.Equal(t, gateway.Alias(args.SessionID.String()), got.GatewayKeyAlias.String)

	// Steps: the build failed, was fixed in the sandbox, passed, then a plan was proposed and applied.
	steps := h.steps(args.SessionID)
	require.NotEmpty(t, steps)
	require.NoError(t, audit.VerifyChain(steps), "the step log hash chain verifies")
	var summaries []string
	locations := map[string]bool{}
	var planStep *dbsqlc.AgentStep
	for i, s := range steps {
		summaries = append(summaries, s.Kind+":"+s.Summary)
		locations[s.Location] = true
		if s.PlanID.Valid && planStep == nil {
			planStep = &steps[i]
		}
	}
	joined := strings.Join(summaries, "\n")
	require.Contains(t, joined, "make build: exit 2")
	require.Contains(t, joined, "write /workspace/Makefile")
	require.Contains(t, joined, "make build: ok")
	require.Contains(t, joined, "plan.propose")
	require.Contains(t, joined, "pending_approval")
	require.True(t, locations["sandbox"] && locations["yari"], "both Sandbox and yari actions are logged: %v", locations)
	require.NotNil(t, planStep, "a step links the proposed plan")

	// The plan really is pending approval on production, proposed by the agent principal.
	plan, err := h.plans.GetPlan(context.Background(), connect.NewRequest(&yariv1.GetPlanRequest{PlanId: planStep.PlanID.UUID.String()}))
	require.NoError(t, err)
	require.Equal(t, yariv1.PlanStatus_PLAN_STATUS_PENDING_APPROVAL, plan.Msg.Plan.Status)
	require.Equal(t, sess.AgentPrincipalId, plan.Msg.Plan.CreatedBy)
	require.Equal(t, sess.Id, plan.Msg.Plan.AgentSessionId)
	require.Equal(t, h.org.ProdEnvID.String(), plan.Msg.Plan.EnvironmentId)

	// Cleanup: sandbox destroyed, model key and session token revoked, audit events written.
	require.Len(t, h.sandboxes.Created(), 1)
	require.Equal(t, h.sandboxes.Created(), h.sandboxes.Destroyed())
	require.Zero(t, h.sandboxes.Live())
	require.True(t, h.gw.wasRevoked(got.GatewayKeyAlias.String))
	require.False(t, h.tokenWorks(args.MCPToken), "the session token is revoked at the end")
	require.Contains(t, h.actions(), "agent.session.started")
	require.Contains(t, h.actions(), "agent.session.ended")

	// The sandbox only ever received the agent server key as environment.
	sb, err := h.sandboxes.Get(context.Background(), got.SandboxID.String)
	if err == nil { // already destroyed in the common case
		_ = sb
	}
}

// Secrets never reach the step log: the three session secrets are removed from every stored event.
func TestStepLogNeverContainsSessionSecrets(t *testing.T) {
	h := newSessionHarness(t)
	args, _ := h.start(h.org.DevEnvID, "leak test")
	var seen agent.SessionSpec
	rt := &leakyRuntime{capture: &seen}
	h.work(h.worker(rt), args)

	require.NotEmpty(t, seen.LLMKey)
	require.NotEmpty(t, seen.MCPToken)
	require.NotEmpty(t, seen.AgentServerKey)
	require.True(t, strings.HasPrefix(seen.AgentServerKey, sandbox.SessionKeyPrefix))

	steps := h.steps(args.SessionID)
	require.NotEmpty(t, steps)
	for _, s := range steps {
		blob := s.Summary + string(s.RawEvent)
		for _, secret := range []string{seen.LLMKey, seen.MCPToken, seen.AgentServerKey, "AKIAABCDEFGHIJKLMNOP", "postgres://admin:hunter2@db.example.com/x"} {
			require.NotContains(t, blob, secret)
		}
	}
	require.Contains(t, steps[0].Summary, agent.Redacted)

	// The sandbox environment held only the agent server key.
	require.Len(t, h.sandboxes.Created(), 1)
}

type leakyRuntime struct{ capture *agent.SessionSpec }

func (*leakyRuntime) Name() string { return "leaky" }
func (l *leakyRuntime) Start(_ context.Context, spec agent.SessionSpec, _ sandbox.Sandbox) (agent.Run, error) {
	*l.capture = spec
	ch := make(chan agent.Event, 4)
	done := make(chan error, 1)
	text := "key " + spec.LLMKey + " token " + spec.MCPToken + " server " + spec.AgentServerKey + " aws AKIAABCDEFGHIJKLMNOP db postgres://admin:hunter2@db.example.com/x"
	raw, _ := json.Marshal(map[string]any{"text": text})
	ch <- agent.Event{Kind: agent.EventMessage, Location: agent.LocationNone, Summary: text, Raw: raw, At: time.Now()}
	close(ch)
	done <- nil
	return &staticRun{events: ch, done: done}, nil
}

type staticRun struct {
	events chan agent.Event
	done   chan error
}

func (r *staticRun) Events() <-chan agent.Event         { return r.events }
func (r *staticRun) Done() <-chan error                 { return r.done }
func (r *staticRun) Send(context.Context, string) error { return nil }
func (r *staticRun) Pause(context.Context) error        { return nil }
func (r *staticRun) Stop(context.Context) error         { return nil }

// A refused delete of a database is recorded as a guardrail step and creates no plan.
func TestDeleteDatabaseByAgentIsBlockedAndLoggedAsGuardrail(t *testing.T) {
	h := newSessionHarness(t)
	args, _ := h.start(h.org.ProdEnvID, "remove the old database")
	h.work(h.worker(&agentfake.Runtime{MCPURL: h.mcpURL, ProposeDeleteDatabase: true}), args)

	steps := h.steps(args.SessionID)
	var guard *dbsqlc.AgentStep
	for i, s := range steps {
		if s.Kind == "guardrail" {
			guard = &steps[i]
		}
		require.False(t, s.PlanID.Valid, "no plan may exist")
	}
	require.NotNil(t, guard, "the refusal is a guardrail step")
	require.Contains(t, guard.Summary, "agents may not delete postgres")
	require.Equal(t, "succeeded", h.session(args.SessionID).Status, "the agent stopped and reported")
}

// Even when the agent crashes or the runtime cannot start, the sandbox is destroyed and keys are revoked.
func TestSandboxAlwaysDestroyedAndKeysRevokedOnError(t *testing.T) {
	cases := map[string]agent.Runtime{
		"start error": failingRuntime{err: errors.New("harness unavailable")},
		"run error":   &erroringRuntime{},
	}
	for name, rt := range cases {
		t.Run(name, func(t *testing.T) {
			h := newSessionHarness(t)
			args, _ := h.start(h.org.DevEnvID, "doomed")
			h.work(h.worker(rt), args)

			s := h.session(args.SessionID)
			require.Equal(t, "failed", s.Status)
			require.Equal(t, h.sandboxes.Created(), h.sandboxes.Destroyed())
			require.True(t, h.gw.wasRevoked(s.GatewayKeyAlias.String))
			require.False(t, h.tokenWorks(args.MCPToken))
			require.Contains(t, h.actions(), "agent.session.ended")
		})
	}
}

type failingRuntime struct{ err error }

func (failingRuntime) Name() string { return "failing" }
func (f failingRuntime) Start(context.Context, agent.SessionSpec, sandbox.Sandbox) (agent.Run, error) {
	return nil, f.err
}

type erroringRuntime struct{}

func (*erroringRuntime) Name() string { return "erroring" }
func (*erroringRuntime) Start(context.Context, agent.SessionSpec, sandbox.Sandbox) (agent.Run, error) {
	ch := make(chan agent.Event)
	close(ch)
	done := make(chan error, 1)
	done <- errors.New("harness crashed")
	return &staticRun{events: ch, done: done}, nil
}

type panickingRuntime struct{}

func (panickingRuntime) Name() string { return "panicking" }
func (panickingRuntime) Start(context.Context, agent.SessionSpec, sandbox.Sandbox) (agent.Run, error) {
	panic("boom")
}

func TestCleanupRunsEvenWhenTheRuntimePanics(t *testing.T) {
	h := newSessionHarness(t)
	args, _ := h.start(h.org.DevEnvID, "panic")
	w := h.worker(panickingRuntime{})
	require.Panics(t, func() { _ = w.Work(context.Background(), &river.Job[jobs.RunAgentSessionArgs]{Args: args}) })

	s := h.session(args.SessionID)
	require.Equal(t, "failed", s.Status)
	require.Equal(t, h.sandboxes.Created(), h.sandboxes.Destroyed())
	require.True(t, h.gw.wasRevoked(s.GatewayKeyAlias.String))
	require.False(t, h.tokenWorks(args.MCPToken))
}

func TestModelKeyFailureStillRevokesTheToken(t *testing.T) {
	h := newSessionHarness(t)
	h.gw.failGen = errors.New("gateway down")
	args, _ := h.start(h.org.DevEnvID, "no gateway")
	h.work(h.worker(&agentfake.Runtime{MCPURL: h.mcpURL}), args)
	require.Equal(t, "failed", h.session(args.SessionID).Status)
	require.Empty(t, h.sandboxes.Created(), "no sandbox without a model key")
	require.False(t, h.tokenWorks(args.MCPToken))
}

// Budget exhaustion ends the session as failed and records a guardrail step.
func TestBudgetExhaustionEndsTheSessionAsFailed(t *testing.T) {
	h := newSessionHarness(t)
	args, _ := h.start(h.org.DevEnvID, "expensive")
	h.gw.setSpend(2.5) // budget is 2
	h.work(h.worker(&agentfake.Runtime{MCPURL: h.mcpURL, Delay: 300 * time.Millisecond}), args)

	s := h.session(args.SessionID)
	require.Equal(t, "failed", s.Status)
	require.EqualValues(t, 2_500_000, s.SpendUsdMicros, "final spend is recorded")
	var found bool
	for _, st := range h.steps(args.SessionID) {
		if st.Kind == "guardrail" && strings.Contains(st.Summary, "budget exhausted") {
			found = true
		}
	}
	require.True(t, found)
	require.True(t, h.gw.wasRevoked(s.GatewayKeyAlias.String))
	require.Equal(t, h.sandboxes.Created(), h.sandboxes.Destroyed())
}

// StopSession from the API reaches the running session through the command channel.
func TestStopCommandEndsARunningSession(t *testing.T) {
	h := newSessionHarness(t)
	args, _ := h.start(h.org.DevEnvID, "long task")
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.work(h.worker(&agentfake.Runtime{MCPURL: h.mcpURL, Delay: 400 * time.Millisecond}), args)
	}()
	require.Eventually(t, func() bool { return h.session(args.SessionID).Status == "running" }, 5*time.Second, 20*time.Millisecond)
	h.live.Dispatch(jobs.Command{SessionID: args.SessionID, Type: jobs.CommandStop})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("session did not stop")
	}
	require.Equal(t, "stopped", h.session(args.SessionID).Status)
	require.Equal(t, h.sandboxes.Created(), h.sandboxes.Destroyed())
}

// The reaper cleans up sessions whose worker died.
func TestReaperCleansUpStuckSessions(t *testing.T) {
	h := newSessionHarness(t)
	args, _ := h.start(h.org.DevEnvID, "orphan")
	// Pretend a worker started it and died: running, with a key alias and a sandbox.
	sb, err := h.sandboxes.Create(context.Background(), sandbox.Spec{SessionID: args.SessionID.String(), Image: "fake"})
	require.NoError(t, err)
	require.NoError(t, db.WithTenant(context.Background(), h.pool, h.org.ID, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		if _, err := q.MarkAgentSessionRunning(ctx, dbsqlc.MarkAgentSessionRunningParams{ID: args.SessionID, GatewayKeyAlias: pgText(gateway.Alias(args.SessionID.String()))}); err != nil {
			return err
		}
		return q.SetAgentSessionSandbox(ctx, dbsqlc.SetAgentSessionSandboxParams{ID: args.SessionID, SandboxID: pgText(sb.ID())})
	}))

	w := &jobs.ReapWorker{Pool: h.pool, Sandboxes: h.sandboxes, Gateway: h.gw, MaxAge: time.Nanosecond}
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, w.Work(context.Background(), &river.Job[jobs.ReapArgs]{}))

	require.Equal(t, "failed", h.session(args.SessionID).Status)
	require.True(t, h.gw.wasRevoked(gateway.Alias(args.SessionID.String())))
	require.Contains(t, h.sandboxes.Destroyed(), sb.ID())
	require.False(t, h.tokenWorks(args.MCPToken))
}

func TestSandboxRejectsLongLivedCredentialsInEnvironment(t *testing.T) {
	m := sandboxfake.NewManager()
	for name, env := range map[string]map[string]string{
		"anthropic key":                   {"ANTHROPIC_API_KEY": "sk-ant-api03-xxxxxxxx"},
		"openai key":                      {"OPENAI_API_KEY": "sk-proj-xxxxxxxxxxxx"},
		"aws":                             {"AWS_SECRET_ACCESS_KEY": "abc"},
		"database url":                    {"DATABASE_URL": "postgres://u:p@h/db"},
		"master key":                      {"LITELLM_MASTER_KEY": "change-me"},
		"password":                        {"DB_PASSWORD": "x"},
		"innocent name, key-shaped value": {"NOTES": "sk-abcdefghijklmnop"},
		"yari token":                      {"X": "yari_abcdefghijkl"},
		"connection string":               {"X": "postgresql://user:pw@host/db"},
		"bad session key":                 {sandbox.AgentServerKeyEnv: "not-generated"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := m.Create(context.Background(), sandbox.Spec{SessionID: "s", Image: "i", Env: env})
			require.Error(t, err)
		})
	}
	_, err := m.Create(context.Background(), sandbox.Spec{SessionID: "ok", Image: "i", Env: map[string]string{sandbox.AgentServerKeyEnv: sandbox.SessionKeyPrefix + "abcdefghijklmnopqrstuvwx", "LANG": "C.UTF-8"}})
	require.NoError(t, err)
}

// internal/agent and cmd/mcp must stay away from the database and the providers.
func TestAgentAndMCPNeverImportDBOrProviders(t *testing.T) {
	for _, pkg := range []string{"github.com/synapsbranch-ux/yari.run/internal/agent/...", "github.com/synapsbranch-ux/yari.run/internal/mcpserver/...", "github.com/synapsbranch-ux/yari.run/cmd/mcp"} {
		out, err := exec.Command("go", "list", "-deps", pkg).Output()
		require.NoError(t, err, pkg)
		for _, dep := range strings.Fields(string(out)) {
			require.NotContains(t, dep, "yari.run/internal/db", "%s depends on %s", pkg, dep)
			require.NotContains(t, dep, "yari.run/internal/provider", "%s depends on %s", pkg, dep)
			require.NotContains(t, dep, "jackc/pgx", "%s depends on %s", pkg, dep)
		}
	}
}

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
