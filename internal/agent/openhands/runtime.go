// Package openhands implements agent.Runtime on the OpenHands Software Agent SDK.
//
// The agent server runs inside the Sandbox. This package only drives it over its REST and
// WebSocket API with the generated client: it creates a conversation (default tool preset, the yari
// MCP server, the session LiteLLM key, the iteration limit), sends the goal, and maps the harness
// events to agent.Event. There is no agent loop and no model client here.
package openhands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands/client"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// Runtime starts conversations on the agent server inside a sandbox.
type Runtime struct {
	// HTTP is used for the REST calls. Defaults to a client with a 60 second timeout.
	HTTP *http.Client
	// ReadyTimeout is how long to wait for the agent server to answer /health. Default 120s.
	ReadyTimeout time.Duration
	// NumRetries is the LLM retry count inside the agent server. Default 2: the session budget and
	// the time limit already bound a run, and long backoffs hide failures.
	NumRetries int
}

// Name implements agent.Runtime.
func (*Runtime) Name() string { return "openhands" }

// WorkingDir is where the agent works inside the sandbox.
const WorkingDir = sandbox.ProjectDir

// firstMessage is what the agent is told first: the operating rules, where it works, and the goal.
func firstMessage(spec agent.SessionSpec) string {
	var b strings.Builder
	if spec.SystemPrompt != "" {
		b.WriteString(strings.TrimSpace(spec.SystemPrompt))
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "Your Environment id is %s. Pass it to the yari tools as environment_id.\n", spec.EnvironmentID)
	fmt.Fprintf(&b, "Your working directory is %s.\n\nGoal:\n%s\n", WorkingDir, spec.Goal)
	return b.String()
}

// Start implements agent.Runtime.
func (r *Runtime) Start(ctx context.Context, spec agent.SessionSpec, sb sandbox.Sandbox) (agent.Run, error) {
	if spec.AgentServerKey == "" {
		return nil, errors.New("openhands: no agent server key")
	}
	api, err := client.NewAuthedClient(sb.Endpoint(), spec.AgentServerKey, r.HTTP)
	if err != nil {
		return nil, err
	}
	if err := r.waitReady(ctx, api); err != nil {
		return nil, err
	}

	retries := r.NumRetries
	if retries <= 0 {
		retries = 2
	}
	cfg := client.ConversationConfig{
		// LiteLLM speaks the OpenAI protocol, so the harness addresses every model as openai/<alias>.
		LLMModel: "openai/" + spec.Model, LLMBaseURL: spec.LLMBaseURL, LLMAPIKey: spec.LLMKey,
		MCPURL: spec.MCPURL, MCPToken: spec.MCPToken,
		WorkingDir: WorkingDir, MaxIterations: spec.MaxIterations, NumRetries: retries,
	}
	resp, err := api.StartConversationWithResponse(ctx, cfg.StartRequest())
	if err != nil {
		return nil, fmt.Errorf("openhands: create conversation: %w", err)
	}
	info := resp.JSON201
	if info == nil {
		info = resp.JSON200
	}
	if info == nil {
		return nil, fmt.Errorf("openhands: create conversation: unexpected HTTP status %d", resp.HTTPResponse.StatusCode)
	}

	// Subscribe before the first message so no event is missed.
	stream, err := client.Dial(ctx, sb.Endpoint(), spec.AgentServerKey, info.Id, client.StreamOptions{ResendAll: true})
	if err != nil {
		return nil, fmt.Errorf("openhands: %w", err)
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	run := &run{
		api: api, id: info.Id, cancel: cancel, endpoint: sb.Endpoint(), key: spec.AgentServerKey,
		events: make(chan agent.Event, 256), done: make(chan error, 1), seen: map[string]bool{},
	}
	if err := run.send(ctx, firstMessage(spec), false); err != nil {
		cancel()
		_ = stream.Close()
		return nil, err
	}
	rr, err := api.RunConversationWithResponse(ctx, info.Id)
	if err != nil {
		cancel()
		_ = stream.Close()
		return nil, fmt.Errorf("openhands: run: %w", err)
	}
	if err := client.CheckStatus("run", rr.HTTPResponse, http.StatusOK); err != nil {
		cancel()
		_ = stream.Close()
		return nil, err
	}
	go run.consume(runCtx, stream)
	return run, nil
}

func (r *Runtime) waitReady(ctx context.Context, api *client.ClientWithResponses) error {
	timeout := r.ReadyTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		resp, err := api.HealthWithResponse(ctx)
		if err == nil && resp.HTTPResponse.StatusCode == http.StatusOK {
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("openhands: the agent server did not become ready within %s: %v", timeout, last)
}

type run struct {
	api      *client.ClientWithResponses
	id       string
	endpoint string
	key      string
	cancel   context.CancelFunc
	events   chan agent.Event
	done     chan error

	mu         sync.Mutex
	seen       map[string]bool
	stopped    bool
	lastErr    string
	mcpChecked bool
	mcpMissing bool
}

// ConversationID is stored on the session row.
func (r *run) ConversationID() string { return r.id }

func (r *run) Events() <-chan agent.Event { return r.events }
func (r *run) Done() <-chan error         { return r.done }

func (r *run) send(ctx context.Context, text string, run bool) error {
	resp, err := r.api.SendMessageWithResponse(ctx, r.id, client.TextMessage(text, run))
	if err != nil {
		return fmt.Errorf("openhands: send message: %w", err)
	}
	return client.CheckStatus("send message", resp.HTTPResponse, http.StatusOK)
}

// Send implements agent.Run: a follow-up instruction that restarts the loop if it is idle.
func (r *run) Send(ctx context.Context, message string) error { return r.send(ctx, message, true) }

// Pause implements agent.Run.
func (r *run) Pause(ctx context.Context) error {
	resp, err := r.api.PauseConversationWithResponse(ctx, r.id)
	if err != nil {
		return err
	}
	return client.CheckStatus("pause", resp.HTTPResponse, http.StatusOK)
}

// Stop implements agent.Run. It interrupts the loop at once, cancelling an in-flight model call.
func (r *run) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.stopped = true
	r.mu.Unlock()
	resp, err := r.api.InterruptConversationWithResponse(ctx, r.id)
	if err != nil {
		r.cancel()
		return err
	}
	// The consumer ends the run when it sees the paused status. A bounded wait covers a lost event.
	go func() {
		select {
		case <-time.After(10 * time.Second):
			r.cancel()
		case <-r.done:
		}
	}()
	return client.CheckStatus("interrupt", resp.HTTPResponse, http.StatusOK)
}

// consume reads the stream until the run ends and reconnects when the socket drops.
func (r *run) consume(ctx context.Context, stream *client.EventStream) {
	defer func() { _ = stream.Close() }()
	var finalErr error
	defer func() {
		close(r.events)
		r.done <- finalErr
	}()

	reconnects := 0
	for {
		ev, err := stream.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// The socket dropped. Replay everything and skip what was already seen.
			if reconnects >= 5 {
				finalErr = fmt.Errorf("openhands: event stream lost: %w", err)
				return
			}
			reconnects++
			time.Sleep(time.Duration(reconnects) * time.Second)
			_ = stream.Close()
			ns, derr := client.Dial(ctx, r.endpoint, r.key, r.id, client.StreamOptions{ResendAll: true})
			if derr != nil {
				continue
			}
			stream = ns
			continue
		}
		reconnects = 0

		if status, ok := ev.ExecutionStatus(); ok {
			if end, err := r.onStatus(status); end {
				finalErr = err
				// Flush the status event too, then stop.
				if me, ok := Map(ev); ok {
					r.emit(me)
				}
				return
			}
		}
		if ev.Kind == client.KindSystemPrompt {
			r.checkMCP(ev)
		}
		me, ok := Map(ev)
		if !ok {
			continue
		}
		if me.Kind == agent.EventError {
			r.mu.Lock()
			r.lastErr = me.Summary
			r.mu.Unlock()
		}
		r.emit(me)
	}
}

func (r *run) emit(e agent.Event) {
	r.mu.Lock()
	if e.ExternalID != "" {
		if r.seen[e.ExternalID] {
			r.mu.Unlock()
			return
		}
		r.seen[e.ExternalID] = true
	}
	r.mu.Unlock()
	r.events <- e
}

// onStatus decides whether an execution status ends the run.
func (r *run) onStatus(s client.ConversationExecutionStatus) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch s {
	case "finished":
		return true, nil
	case "error":
		if r.lastErr != "" {
			return true, errors.New(r.lastErr)
		}
		return true, errors.New("the agent ended with an error")
	case "stuck":
		return true, errors.New("the agent got stuck repeating itself")
	case "paused":
		if r.stopped {
			if r.mcpMissing {
				return true, errors.New(r.lastErr)
			}
			return true, nil
		}
	case "deleting":
		return true, nil
	}
	return false, nil
}

// checkMCP flags a session whose yari tools did not load: the harness skips an unreachable MCP
// server without failing, and an agent without the yari tools must not be trusted to proceed.
func (r *run) checkMCP(ev client.StreamEvent) {
	r.mu.Lock()
	if r.mcpChecked {
		r.mu.Unlock()
		return
	}
	r.mcpChecked = true
	r.mu.Unlock()
	if systemPromptHasYariTools(ev.Raw) {
		return
	}
	r.mu.Lock()
	r.lastErr = "the yari MCP tools did not load in the agent"
	r.mcpMissing = true
	r.mu.Unlock()
	r.emit(agent.Event{ExternalID: ev.ID + "-mcp", Kind: agent.EventError, Location: agent.LocationNone,
		Summary: "the yari MCP tools did not load in the agent (is the MCP server reachable from the sandbox?)", At: time.Now()})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = r.Stop(ctx)
	}()
}
