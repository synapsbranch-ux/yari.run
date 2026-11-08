// Package sandbox defines the isolated container an Agent Session runs in.
//
// One Sandbox per Agent Session, created by yari.run's own sandbox manager. The Sandbox runs the
// OpenHands agent server; the worker drives it from outside. A Sandbox never receives a
// long-lived credential and has no route to the internet.
package sandbox

import (
	"context"
	"time"
)

// NetworkPolicy selects how a Sandbox is networked.
type NetworkPolicy string

const (
	// NetworkAgentEgress attaches the Sandbox only to the internal yari-agent-net network. It can
	// reach the model gateway and the MCP server, and nothing else. This is the default.
	NetworkAgentEgress NetworkPolicy = "agent-egress"
	// NetworkNone attaches no network at all.
	NetworkNone NetworkPolicy = "none"
)

// AgentNetworkName is the Docker network sandboxes join. It is created with internal: true.
const AgentNetworkName = "yari-agent-net"

// Spec describes a Sandbox to create.
type Spec struct {
	SessionID     string
	Image         string
	CPUs          float64
	MemoryMB      int
	PidsLimit     int
	DiskMB        int
	TTL           time.Duration
	Network       NetworkPolicy
	WritablePaths []string          // in addition to /workspace and /tmp, only what the agent server needs
	Env           map[string]string // validated by ValidateEnv
}

// ExecOptions controls a command run inside a Sandbox.
type ExecOptions struct {
	Dir      string
	Timeout  time.Duration
	MaxBytes int // per stream; output beyond it is dropped and Truncated is set
}

// ExecResult is the outcome of a command.
type ExecResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Truncated bool
}

// Sandbox is one running isolated container.
type Sandbox interface {
	ID() string
	// Endpoint is the base URL of the agent server, reachable by the worker only.
	Endpoint() string
	Exec(ctx context.Context, cmd []string, opts ExecOptions) (ExecResult, error)
	WriteFile(ctx context.Context, path string, data []byte) error
	ReadFile(ctx context.Context, path string, maxBytes int) ([]byte, error)
	Destroy(ctx context.Context) error
}

// Manager creates and finds Sandboxes.
type Manager interface {
	Create(ctx context.Context, spec Spec) (Sandbox, error)
	Get(ctx context.Context, id string) (Sandbox, error)
	// Reap destroys Sandboxes past their TTL and returns how many it destroyed.
	Reap(ctx context.Context) (int, error)
}

// Defaults for sessions in this scaffold.
const (
	DefaultCPUs      = 2.0
	DefaultMemoryMB  = 4096
	DefaultPidsLimit = 512
	DefaultDiskMB    = 4096
	DefaultTTL       = 45 * time.Minute
	DefaultMaxBytes  = 1 << 20
)

// WithDefaults fills zero values of the spec.
func (s Spec) WithDefaults() Spec {
	if s.CPUs == 0 {
		s.CPUs = DefaultCPUs
	}
	if s.MemoryMB == 0 {
		s.MemoryMB = DefaultMemoryMB
	}
	if s.PidsLimit == 0 {
		s.PidsLimit = DefaultPidsLimit
	}
	if s.DiskMB == 0 {
		s.DiskMB = DefaultDiskMB
	}
	if s.TTL == 0 {
		s.TTL = DefaultTTL
	}
	if s.Network == "" {
		s.Network = NetworkAgentEgress
	}
	return s
}
