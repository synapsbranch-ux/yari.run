// Package fake is a scripted agent.Runtime for tests and the no-key demo.
//
// It plays the part of OpenHands without a model: it emits a realistic sequence of events and makes
// real calls to the yari MCP server with the session token and real commands in the sandbox.
// Script: read the spec, run the sample repo build and see it fail, fix the build with WriteFile,
// rerun it successfully, propose a plan, apply it on the environment.
package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// Runtime is the scripted runtime.
type Runtime struct {
	// MCPURL overrides SessionSpec.MCPURL. The spec URL is valid inside the sandbox network, but
	// the worker that runs this script is outside it.
	MCPURL string
	// Delay pauses between steps so a watcher sees the timeline grow. Zero in tests.
	Delay time.Duration
	// FailBuildForever keeps the build broken, to test sessions that cannot finish.
	FailBuildForever bool
	// ProposeDeleteDatabase makes the script try to delete a database, to show the deny list.
	ProposeDeleteDatabase bool
}

// Name implements agent.Runtime.
func (*Runtime) Name() string { return "fake" }

// Start implements agent.Runtime.
func (r *Runtime) Start(ctx context.Context, spec agent.SessionSpec, sb sandbox.Sandbox) (agent.Run, error) {
	ctx, cancel := context.WithCancel(ctx)
	run := &run{r: r, spec: spec, sb: sb, cancel: cancel, events: make(chan agent.Event, 256), done: make(chan error, 1), inbox: make(chan string, 16)}
	go run.play(ctx)
	return run, nil
}

type run struct {
	r      *Runtime
	spec   agent.SessionSpec
	sb     sandbox.Sandbox
	cancel context.CancelFunc
	events chan agent.Event
	done   chan error
	inbox  chan string

	mu  sync.Mutex
	seq int
}

func (r *run) Events() <-chan agent.Event { return r.events }
func (r *run) Done() <-chan error         { return r.done }

func (r *run) Send(_ context.Context, message string) error {
	select {
	case r.inbox <- message:
		return nil
	default:
		return errors.New("fake runtime: inbox full")
	}
}

func (r *run) Pause(context.Context) error { return nil }

func (r *run) Stop(context.Context) error {
	r.cancel()
	return nil
}

func (r *run) emit(kind agent.EventKind, tool, location, summary string, raw any) {
	r.mu.Lock()
	r.seq++
	id := fmt.Sprintf("fake-%s-%d", r.spec.SessionID, r.seq)
	r.mu.Unlock()
	b, _ := json.Marshal(raw)
	select {
	case r.events <- agent.Event{ExternalID: id, Kind: kind, Tool: tool, Location: location, Summary: summary, Raw: b, At: time.Now()}:
	default: // a slow consumer must not stall the script
	}
}

func (r *run) pause(ctx context.Context) error {
	d := r.r.Delay
	if d == 0 {
		return ctx.Err()
	}
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// play runs the script and ends the run.
func (r *run) play(ctx context.Context) {
	err := r.script(ctx)
	if errors.Is(err, context.Canceled) {
		r.emit(agent.EventStatus, "", agent.LocationNone, "stopped", map[string]any{"reason": "stop requested"})
		err = nil
	}
	if err != nil {
		r.emit(agent.EventError, "", agent.LocationNone, "script failed: "+err.Error(), map[string]any{"error": err.Error()})
	}
	close(r.events)
	r.done <- err
}

type headerTransport struct {
	token, session string
	base           http.RoundTripper
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("X-Yari-Session", t.session)
	return t.base.RoundTrip(req)
}

func (r *run) script(ctx context.Context) error {
	url := r.r.MCPURL
	if url == "" {
		url = r.spec.MCPURL
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "yari-fake-agent", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             url,
		HTTPClient:           &http.Client{Transport: headerTransport{token: r.spec.MCPToken, session: r.spec.SessionID, base: http.DefaultTransport}, Timeout: 30 * time.Second},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		return fmt.Errorf("connect to the yari MCP server: %w", err)
	}
	defer cs.Close()

	call := func(tool string, args map[string]any) (map[string]any, bool, error) {
		r.emit(agent.EventAction, tool, agent.LocationYari, "call "+tool, map[string]any{"tool": tool, "arguments": args})
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			return nil, false, err
		}
		var text string
		for _, c := range res.Content {
			if t, ok := c.(*mcp.TextContent); ok {
				text += t.Text
			}
		}
		r.emit(agent.EventObservation, tool, agent.LocationYari, summarize(tool, text, res.IsError), map[string]any{"tool": tool, "is_error": res.IsError, "result": text})
		var out map[string]any
		_ = json.Unmarshal([]byte(text), &out)
		return out, res.IsError, nil
	}
	exec := func(cmd ...string) (sandbox.ExecResult, error) {
		r.emit(agent.EventAction, "terminal", agent.LocationSandbox, "run "+join(cmd), map[string]any{"command": cmd})
		res, err := r.sb.Exec(ctx, cmd, sandbox.ExecOptions{Dir: "/workspace", Timeout: 60 * time.Second})
		if err != nil {
			return res, err
		}
		kind := "ok"
		if res.ExitCode != 0 {
			kind = fmt.Sprintf("exit %d", res.ExitCode)
		}
		r.emit(agent.EventObservation, "terminal", agent.LocationSandbox, fmt.Sprintf("%s: %s", join(cmd), kind), map[string]any{"exit_code": res.ExitCode, "stdout": res.Stdout, "stderr": res.Stderr})
		return res, nil
	}

	r.emit(agent.EventMessage, "", agent.LocationNone, "Goal received: "+r.spec.Goal, map[string]any{"role": "user", "content": r.spec.Goal})
	if err := r.pause(ctx); err != nil {
		return err
	}

	r.emit(agent.EventMessage, "", agent.LocationNone, "I will read the environment, get the build working in my sandbox, then propose a plan.", map[string]any{"role": "assistant"})
	if _, isErr, err := call("spec.read", map[string]any{"environment_id": r.spec.EnvironmentID}); err != nil || isErr {
		return orErr(err, "spec.read failed")
	}
	if err := r.pause(ctx); err != nil {
		return err
	}

	res, err := exec("make", "build")
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		if err := r.pause(ctx); err != nil {
			return err
		}
		r.emit(agent.EventMessage, "", agent.LocationNone, "The build fails because dist/ does not exist. I will make the build create it.", map[string]any{"role": "assistant"})
		if !r.r.FailBuildForever {
			r.emit(agent.EventAction, "file_editor", agent.LocationSandbox, "write /workspace/Makefile", map[string]any{"path": "/workspace/Makefile"})
			fixed := "build:\n\tmkdir -p dist\n\tcat src/main.txt > dist/out.txt\n"
			if err := r.sb.WriteFile(ctx, "/workspace/Makefile", []byte(fixed)); err != nil {
				return fmt.Errorf("write Makefile: %w", err)
			}
			r.emit(agent.EventObservation, "file_editor", agent.LocationSandbox, "Makefile updated", map[string]any{"path": "/workspace/Makefile"})
		}
		if res, err = exec("make", "build"); err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return errors.New("the build still fails")
		}
	}
	if err := r.pause(ctx); err != nil {
		return err
	}

	changes := []map[string]any{{"service": "api", "action": "create", "kind": "web", "provider": "railway",
		"env": map[string]any{"DATABASE_URL": map[string]any{"from_service": "db", "from_key": "DATABASE_URL", "secret": true}}},
		{"service": "db", "action": "create", "kind": "postgres", "provider": "supabase"}}
	if r.r.ProposeDeleteDatabase {
		changes = []map[string]any{{"service": "db", "action": "delete", "kind": "postgres", "provider": "supabase"}}
	}
	r.emit(agent.EventMessage, "", agent.LocationNone, "The build passes. I will propose a plan to deploy it.", map[string]any{"role": "assistant"})
	plan, isErr, err := call("plan.propose", map[string]any{"environment_id": r.spec.EnvironmentID, "changes": changes})
	if err != nil {
		return err
	}
	if isErr {
		r.emit(agent.EventMessage, "", agent.LocationNone, "My plan was refused. I will stop and report instead of trying another way.", map[string]any{"role": "assistant"})
		return nil
	}
	planID, _ := plan["plan_id"].(string)
	if err := r.pause(ctx); err != nil {
		return err
	}

	applied, isErr, err := call("plan.apply", map[string]any{"plan_id": planID})
	if err != nil || isErr {
		return orErr(err, "plan.apply failed")
	}
	if status, _ := applied["status"].(string); status == "pending_approval" {
		r.emit(agent.EventMessage, "", agent.LocationNone, "The plan is waiting for approval from a person. I am done until it is approved.", map[string]any{"role": "assistant", "plan_id": planID})
	} else {
		r.emit(agent.EventMessage, "", agent.LocationNone, "The plan is applying.", map[string]any{"role": "assistant", "plan_id": planID})
	}

	// Answer follow-ups until stopped: a person may still be talking to the session.
	select {
	case msg := <-r.inbox:
		r.emit(agent.EventMessage, "", agent.LocationNone, "Follow-up received: "+msg, map[string]any{"role": "user", "content": msg})
	default:
	}
	return nil
}

func join(cmd []string) string {
	s := ""
	for i, c := range cmd {
		if i > 0 {
			s += " "
		}
		s += c
	}
	return s
}

func orErr(err error, msg string) error {
	if err != nil {
		return err
	}
	return errors.New(msg)
}

func summarize(tool, text string, isErr bool) string {
	if isErr {
		return tool + " refused: " + text
	}
	var out map[string]any
	if json.Unmarshal([]byte(text), &out) == nil {
		if id, ok := out["plan_id"].(string); ok {
			return fmt.Sprintf("%s: plan %s is %v", tool, id, out["status"])
		}
	}
	return tool + " ok"
}
