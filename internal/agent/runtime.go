// Package agent defines how yari.run talks to an agent harness. The harness (OpenHands) owns
// planning, tool calling, file editing, context management and the agent loop. yari.run owns the
// sandbox, the model gateway, the tools exposed over MCP, the event log and the approvals.
// There is no agent loop and no model client in this repository.
//
// This package must never import internal/db or the provider packages.
package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// SessionSpec is everything a Runtime needs to start one Agent Session.
type SessionSpec struct {
	SessionID string
	// EnvironmentID is the Environment the agent works on. The agent is told it in its first
	// message. It is not a secret.
	EnvironmentID string
	Goal          string
	SystemPrompt  string // yari.run operating rules for the agent, from agent/prompts/system.md
	Model         string // gateway alias
	LLMBaseURL    string // LiteLLM URL as seen from inside yari-agent-net
	LLMKey        string // session virtual key (secret)
	MCPURL        string // yari MCP URL as seen from inside yari-agent-net
	MCPToken      string // session yari API token (secret)
	// AgentServerKey is the session API key that protects the agent server inside the sandbox
	// (secret). The worker uses it to talk to the server; the container gets it as environment.
	AgentServerKey string
	MaxIterations  int
}

// Secrets returns the secret values of the spec, for redaction.
func (s SessionSpec) Secrets() []string {
	var out []string
	for _, v := range []string{s.AgentServerKey, s.LLMKey, s.MCPToken} {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// EventKind classifies a harness event.
type EventKind string

// Event kinds.
const (
	EventMessage     EventKind = "message"
	EventAction      EventKind = "action"
	EventObservation EventKind = "observation"
	EventError       EventKind = "error"
	EventStatus      EventKind = "status"
)

// Locations of an action.
const (
	LocationSandbox = "sandbox" // built-in harness tools running in the container
	LocationYari    = "yari"    // yari.run tools reached over MCP
	LocationNone    = "none"
)

// Event is one harness event in yari.run's own shape.
type Event struct {
	ExternalID string
	Kind       EventKind
	Tool       string
	Location   string
	Summary    string
	Raw        json.RawMessage
	At         time.Time
}

// Run is one running Agent Session.
type Run interface {
	// Events streams every event. It is closed when the run ends.
	Events() <-chan Event
	// Send gives the agent a follow-up instruction.
	Send(ctx context.Context, message string) error
	Pause(ctx context.Context) error
	Stop(ctx context.Context) error
	// Done receives exactly one value when the run ends: nil for a clean end or a stop.
	Done() <-chan error
}

// Runtime starts runs. Implementations: openhands (the real harness) and fake (scripted, for tests
// and the no-key demo).
type Runtime interface {
	Name() string
	Start(ctx context.Context, spec SessionSpec, sb sandbox.Sandbox) (Run, error)
}
