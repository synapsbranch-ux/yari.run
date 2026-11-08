package jobs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// ModelGateway is the part of the LiteLLM admin client the session job uses.
type ModelGateway interface {
	CreateSessionKey(ctx context.Context, sessionID, model string, budgetUSD float64, ttl time.Duration) (gateway.SessionKey, error)
	Spend(ctx context.Context, alias string) (float64, error)
	Revoke(ctx context.Context, alias string) error
}

// SessionWorker runs one Agent Session: it owns the sandbox, the model key and the step log, and
// drives the harness through agent.Runtime. It contains no agent loop.
type SessionWorker struct {
	river.WorkerDefaults[RunAgentSessionArgs]

	Pool       *pgxpool.Pool
	Config     config.Config
	Sandboxes  sandbox.Manager
	Gateway    ModelGateway
	Runtime    agent.Runtime
	Live       *LiveSessions // registry the command listener dispatches to
	Prompt     string        // system prompt
	SampleRepo fs.FS         // copied into /workspace in dev; may be nil
	// SpendPoll is how often the budget is checked. Defaults to 5s.
	SpendPoll time.Duration
}

// LiveSessions routes API commands (send, stop) to running sessions in this process.
type LiveSessions struct {
	mu sync.Mutex
	m  map[uuid.UUID]*liveSession
}

type liveSession struct {
	messages chan string
	stop     chan struct{}
	once     sync.Once
}

// NewLiveSessions returns an empty registry.
func NewLiveSessions() *LiveSessions { return &LiveSessions{m: map[uuid.UUID]*liveSession{}} }

func (l *LiveSessions) add(id uuid.UUID) *liveSession {
	ls := &liveSession{messages: make(chan string, 16), stop: make(chan struct{})}
	l.mu.Lock()
	l.m[id] = ls
	l.mu.Unlock()
	return ls
}

func (l *LiveSessions) remove(id uuid.UUID) {
	l.mu.Lock()
	delete(l.m, id)
	l.mu.Unlock()
}

// Dispatch delivers a command to a session running here. Commands for other sessions are ignored.
func (l *LiveSessions) Dispatch(c Command) {
	l.mu.Lock()
	ls := l.m[c.SessionID]
	l.mu.Unlock()
	if ls == nil {
		return
	}
	switch c.Type {
	case CommandSend:
		select {
		case ls.messages <- c.Message:
		default:
			slog.Warn("session inbox full, dropping message", "session", c.SessionID)
		}
	case CommandStop:
		ls.once.Do(func() { close(ls.stop) })
	}
}

// ListenForCommands listens on the NOTIFY channel until ctx ends and dispatches commands.
func ListenForCommands(ctx context.Context, pool *pgxpool.Pool, live *LiveSessions) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+CommandChannel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		var c Command
		if json.Unmarshal([]byte(n.Payload), &c) == nil {
			live.Dispatch(c)
		}
	}
}

// outcome is how a session ended.
type outcome struct {
	status string // succeeded, waiting_approval, failed, stopped
	reason string
}

// Work implements river.Worker.
func (w *SessionWorker) Work(ctx context.Context, job *river.Job[RunAgentSessionArgs]) error {
	org, sid := job.Args.OrganizationID, job.Args.SessionID
	mcpToken := job.Args.MCPToken

	var sess dbsqlc.AgentSession
	if err := db.WithTenant(ctx, w.Pool, org, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		sess, err = dbsqlc.New(tx).GetAgentSession(ctx, sid)
		return err
	}); err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if sess.Status != "queued" {
		slog.Info("session is not queued, skipping", "session", sid, "status", sess.Status)
		return nil
	}

	s := &running{w: w, org: org, sid: sid, sess: sess, mcpToken: mcpToken}
	// Cleanup always runs, including on panic. The deferred function never lets a failure to clean
	// up hide the original outcome.
	defer s.cleanup()
	s.result = outcome{status: "failed", reason: "internal error"}

	err := s.run(ctx)
	if err != nil {
		s.result = outcome{status: "failed", reason: err.Error()}
		slog.Error("agent session failed", "session", sid, "error", err)
	}
	return nil // the session row is the outcome; River must not retry a session
}

type running struct {
	w        *SessionWorker
	org, sid uuid.UUID
	sess     dbsqlc.AgentSession
	mcpToken string

	key      gateway.SessionKey
	sb       sandbox.Sandbox
	redact   *agent.Redactor
	result   outcome
	lastPlan uuid.UUID
}

func (s *running) tenant(ctx context.Context, fn func(ctx context.Context, q *dbsqlc.Queries) error) error {
	return db.WithTenant(ctx, s.w.Pool, s.org, func(ctx context.Context, tx pgx.Tx) error { return fn(ctx, dbsqlc.New(tx)) })
}

func newSecret(prefix string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *running) step(ctx context.Context, in audit.StepInput) error {
	in.Summary = s.redact.String(in.Summary)
	in.RawEvent = s.redact.Bytes(in.RawEvent)
	return s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
		_, err := audit.AppendStep(ctx, q, s.org, s.sid, in)
		return err
	})
}

func (s *running) run(ctx context.Context) error {
	w := s.w
	ttl := w.Config.AgentSessionTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	budget := float64(s.sess.BudgetUsdMicros) / 1e6

	// 1. Model key: one per session, one model, a budget, an expiry. Only its alias is stored.
	key, err := w.Gateway.CreateSessionKey(ctx, s.sid.String(), s.sess.Model, budget, ttl)
	if err != nil {
		return fmt.Errorf("create the session model key: %w", err)
	}
	s.key = key
	agentKey, err := newSecret(sandbox.SessionKeyPrefix)
	if err != nil {
		return err
	}
	s.redact = agent.NewRedactor(agentKey, key.Key, s.mcpToken)

	if err := s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
		if _, err := q.MarkAgentSessionRunning(ctx, dbsqlc.MarkAgentSessionRunningParams{ID: s.sid, GatewayKeyAlias: pgText(key.Alias)}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Event{OrganizationID: s.org, PrincipalID: s.sess.StartedBy, Action: "agent.session.started",
			ScopeType: "environment", ScopeID: s.sess.EnvironmentID, Target: s.sid.String(), AgentSessionID: s.sid,
			Metadata: map[string]any{"model": s.sess.Model, "runtime": w.Runtime.Name(), "budget_usd": budget}})
	}); err != nil {
		return err
	}

	// 2. Sandbox, with the only secret it may hold: the agent server session key.
	sb, err := w.Sandboxes.Create(ctx, sandbox.Spec{
		SessionID: s.sid.String(), Image: w.Config.AgentImage, TTL: ttl + 5*time.Minute,
		Network: sandbox.NetworkAgentEgress, Env: map[string]string{sandbox.AgentServerKeyEnv: agentKey},
	})
	if err != nil {
		return fmt.Errorf("create the sandbox: %w", err)
	}
	s.sb = sb
	if err := s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
		return q.SetAgentSessionSandbox(ctx, dbsqlc.SetAgentSessionSandboxParams{ID: s.sid, SandboxID: pgText(sb.ID())})
	}); err != nil {
		return err
	}
	if w.Config.IsDev() && w.SampleRepo != nil {
		if err := copyTree(ctx, sb, w.SampleRepo, "/workspace"); err != nil {
			return fmt.Errorf("copy the sample repo: %w", err)
		}
	}

	// 3. Start the harness.
	runCtx, cancel := context.WithTimeout(ctx, ttl)
	defer cancel()
	spec := agent.SessionSpec{
		SessionID: s.sid.String(), EnvironmentID: s.sess.EnvironmentID.String(), Goal: s.sess.Goal, SystemPrompt: w.Prompt, Model: s.sess.Model,
		LLMBaseURL: w.Config.LLMBaseURL, LLMKey: key.Key, MCPURL: w.Config.MCPURL, MCPToken: s.mcpToken,
		AgentServerKey: agentKey, MaxIterations: int(s.sess.MaxIterations),
	}
	live := w.Live.add(s.sid)
	defer w.Live.remove(s.sid)
	r, err := w.Runtime.Start(runCtx, spec, sb)
	if err != nil {
		return fmt.Errorf("start the agent: %w", err)
	}
	if c, ok := r.(interface{ ConversationID() string }); ok && c.ConversationID() != "" {
		_ = s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
			return q.SetAgentSessionConversation(ctx, dbsqlc.SetAgentSessionConversationParams{ID: s.sid, ExternalConversationID: pgText(c.ConversationID())})
		})
	}

	// 4. Consume events. This is the only writer of the step log.
	poll := w.SpendPoll
	if poll <= 0 {
		poll = 5 * time.Second
	}
	spendTick := time.NewTicker(poll)
	defer spendTick.Stop()
	var (
		actions    int
		stopped    bool
		budgetGone bool
		iterLimit  bool
		stopping   bool
	)
	stopRun := func() {
		if !stopping {
			stopping = true
			_ = r.Stop(context.WithoutCancel(ctx))
		}
	}
	events := r.Events()
loop:
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				break loop
			}
			in := s.classify(ctx, ev)
			if err := s.step(ctx, in); err != nil {
				slog.Error("append step", "session", s.sid, "error", err)
			}
			if in.PlanID != uuid.Nil {
				s.lastPlan = in.PlanID
			}
			if ev.Kind == agent.EventAction {
				actions++
				if s.sess.MaxIterations > 0 && actions > int(s.sess.MaxIterations) && !iterLimit {
					iterLimit = true
					_ = s.step(ctx, audit.StepInput{Kind: "guardrail", Location: "none", Summary: fmt.Sprintf("iteration limit of %d reached, stopping", s.sess.MaxIterations), RawEvent: []byte(`{"guardrail":"iteration-limit"}`)})
					stopRun()
				}
			}
			if ev.Kind == agent.EventError && gateway.IsBudgetExhausted(ev.Summary) && !budgetGone {
				budgetGone = true
				stopRun()
			}
		case msg := <-live.messages:
			if err := r.Send(ctx, msg); err != nil {
				slog.Warn("send to agent", "session", s.sid, "error", err)
				continue
			}
			_ = s.step(ctx, audit.StepInput{Kind: "message", Location: "none", Summary: "follow-up from a person: " + msg, RawEvent: mustJSON(map[string]any{"role": "user", "content": msg})})
		case <-live.stop:
			stopped = true
			stopRun()
		case <-spendTick.C:
			spend, err := w.Gateway.Spend(ctx, key.Alias)
			if err == nil && spend >= budget && budget > 0 && !budgetGone {
				budgetGone = true
				stopRun()
			}
		case <-runCtx.Done():
			stopRun()
			events = nil // stop reading; wait for Done
			break loop
		}
	}
	runErr := <-r.Done()

	switch {
	case budgetGone:
		_ = s.step(ctx, audit.StepInput{Kind: "guardrail", Location: "none", Summary: "budget exhausted: the model gateway refuses further calls", RawEvent: []byte(`{"guardrail":"budget"}`)})
		s.result = outcome{"failed", "budget exhausted"}
	case iterLimit:
		s.result = outcome{"failed", "iteration limit reached"}
	case stopped:
		s.result = outcome{"stopped", "stopped by a person"}
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		_ = s.step(ctx, audit.StepInput{Kind: "guardrail", Location: "none", Summary: "session time limit reached", RawEvent: []byte(`{"guardrail":"ttl"}`)})
		s.result = outcome{"failed", "time limit reached"}
	case runErr != nil:
		_ = s.step(ctx, audit.StepInput{Kind: "error", Location: "none", Summary: "agent failed: " + runErr.Error(), RawEvent: mustJSON(map[string]any{"error": runErr.Error()})})
		s.result = outcome{"failed", runErr.Error()}
	default:
		s.result = outcome{"succeeded", ""}
		if s.pendingPlan(ctx) {
			s.result = outcome{"waiting_approval", "a plan is waiting for approval"}
		}
	}
	return nil
}

// pendingPlan reports whether the session's last plan is waiting for approval.
func (s *running) pendingPlan(ctx context.Context) bool {
	if s.lastPlan == uuid.Nil {
		return false
	}
	pending := false
	_ = s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
		p, err := q.GetPlan(ctx, s.lastPlan)
		pending = err == nil && p.Status == "pending_approval"
		return nil
	})
	return pending
}

// classify maps a harness event to a step, redacting secrets and linking plans.
func (s *running) classify(ctx context.Context, ev agent.Event) audit.StepInput {
	kind := string(ev.Kind)
	if ev.Kind == agent.EventObservation && strings.Contains(ev.Summary, "guardrail: ") {
		kind = "guardrail" // a guardrail refused the tool call
	}
	loc := ev.Location
	if loc == "" {
		loc = agent.LocationNone
	}
	in := audit.StepInput{Kind: kind, Tool: ev.Tool, Location: loc, Summary: ev.Summary, RawEvent: ev.Raw, ExternalEventID: ev.ExternalID}
	if strings.HasPrefix(ev.Tool, "plan.") && ev.Kind == agent.EventObservation {
		if id := planIDFrom(ev.Raw); id != uuid.Nil {
			in.PlanID = id
			// Only link plans of this organization.
			ok := false
			_ = s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
				_, err := q.GetPlan(ctx, id)
				ok = err == nil
				return nil
			})
			if !ok {
				in.PlanID = uuid.Nil
			}
		}
	}
	return in
}

func planIDFrom(raw json.RawMessage) uuid.UUID {
	var outer struct {
		Result string `json:"result"`
	}
	if json.Unmarshal(raw, &outer) != nil || outer.Result == "" {
		return uuid.Nil
	}
	var inner struct {
		PlanID string `json:"plan_id"`
	}
	if json.Unmarshal([]byte(outer.Result), &inner) != nil {
		return uuid.Nil
	}
	id, _ := uuid.Parse(inner.PlanID)
	return id
}

// cleanup always destroys the sandbox, revokes the model key and the session token, records the
// final spend and status, and writes the ended audit event. Each step is tried even if another fails.
func (s *running) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	w := s.w

	if s.sb != nil {
		if err := s.sb.Destroy(ctx); err != nil {
			slog.Error("destroy sandbox", "session", s.sid, "error", err)
		}
	}
	var spendMicros int64
	if s.key.Alias != "" {
		if spend, err := w.Gateway.Spend(ctx, s.key.Alias); err == nil {
			spendMicros = int64(spend * 1e6)
		}
		if err := w.Gateway.Revoke(ctx, s.key.Alias); err != nil {
			slog.Error("revoke model key", "session", s.sid, "error", err)
		}
	}
	err := s.tenant(ctx, func(ctx context.Context, q *dbsqlc.Queries) error {
		if _, err := q.RevokeAPITokensByLabel(ctx, sessionTokenLabel(s.sid)); err != nil {
			return err
		}
		if _, err := q.FinishAgentSession(ctx, dbsqlc.FinishAgentSessionParams{ID: s.sid, Status: s.result.status, SpendUsdMicros: spendMicros}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Event{OrganizationID: s.org, PrincipalID: s.sess.AgentPrincipalID, Action: "agent.session.ended",
			ScopeType: "environment", ScopeID: s.sess.EnvironmentID, Target: s.sid.String(), AgentSessionID: s.sid,
			Metadata: map[string]any{"status": s.result.status, "reason": s.result.reason, "spend_usd_micros": spendMicros}})
	})
	if err != nil {
		slog.Error("record session end", "session", s.sid, "error", err)
	}
}

// sessionTokenLabel must match api.SessionTokenLabel. It is duplicated to keep jobs free of an api import.
func sessionTokenLabel(session uuid.UUID) string { return "agent-session:" + session.String() }

func copyTree(ctx context.Context, sb sandbox.Sandbox, fsys fs.FS, dest string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return sb.WriteFile(ctx, dest+"/"+path, data)
	})
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
