// Package fake is an in-memory sandbox.Manager for unit tests. It runs nothing: commands are
// answered by a pluggable function, and files live in a map.
package fake

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// ExecFunc answers a command run in a fake sandbox.
type ExecFunc func(s *Sandbox, cmd []string, opts sandbox.ExecOptions) (sandbox.ExecResult, error)

// Manager is an in-memory sandbox.Manager.
type Manager struct {
	// Exec answers commands. If nil, SampleRepoExec is used.
	Exec ExecFunc
	// Now is the clock. Defaults to time.Now.
	Now func() time.Time
	// CreateErr makes Create fail.
	CreateErr error
	// DestroyErr makes Destroy fail after recording the attempt.
	DestroyErr error

	mu        sync.Mutex
	seq       int
	sandboxes map[string]*Sandbox
	created   []string
	destroyed []string
}

// NewManager returns an empty manager.
func NewManager() *Manager { return &Manager{sandboxes: map[string]*Sandbox{}} }

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Create implements sandbox.Manager. It validates the environment like the real manager does.
func (m *Manager) Create(_ context.Context, spec sandbox.Spec) (sandbox.Sandbox, error) {
	spec = spec.WithDefaults()
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	if err := sandbox.ValidateEnv(spec.Env); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	s := &Sandbox{m: m, id: fmt.Sprintf("fake-sandbox-%d", m.seq), Spec: spec, files: map[string][]byte{}, expires: m.now().Add(spec.TTL)}
	m.sandboxes[s.id] = s
	m.created = append(m.created, s.id)
	return s, nil
}

// Get implements sandbox.Manager.
func (m *Manager) Get(_ context.Context, id string) (sandbox.Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sandboxes[id]
	if !ok {
		return nil, fmt.Errorf("fake sandbox %s not found", id)
	}
	return s, nil
}

// Reap implements sandbox.Manager.
func (m *Manager) Reap(ctx context.Context) (int, error) {
	m.mu.Lock()
	var expired []*Sandbox
	for _, s := range m.sandboxes {
		if !m.now().Before(s.expires) {
			expired = append(expired, s)
		}
	}
	m.mu.Unlock()
	for _, s := range expired {
		_ = s.Destroy(ctx)
	}
	return len(expired), nil
}

// Created returns the ids of every sandbox ever created.
func (m *Manager) Created() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.created...)
}

// Destroyed returns the ids of every sandbox destroyed.
func (m *Manager) Destroyed() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.destroyed...)
}

// Live returns how many sandboxes exist.
func (m *Manager) Live() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.sandboxes) }

// Sandbox is an in-memory sandbox.
type Sandbox struct {
	m       *Manager
	id      string
	Spec    sandbox.Spec
	expires time.Time

	mu    sync.Mutex
	files map[string][]byte
	execs [][]string
}

// ID implements sandbox.Sandbox.
func (s *Sandbox) ID() string { return s.id }

// Endpoint implements sandbox.Sandbox. Nothing listens here.
func (s *Sandbox) Endpoint() string { return "http://" + s.id + ".fake.invalid:8000" }

// Exec implements sandbox.Sandbox.
func (s *Sandbox) Exec(_ context.Context, cmd []string, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	s.mu.Lock()
	s.execs = append(s.execs, append([]string(nil), cmd...))
	s.mu.Unlock()
	fn := s.m.Exec
	if fn == nil {
		fn = SampleRepoExec
	}
	return fn(s, cmd, opts)
}

// WriteFile implements sandbox.Sandbox.
func (s *Sandbox) WriteFile(_ context.Context, path string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = append([]byte(nil), data...)
	return nil
}

// ReadFile implements sandbox.Sandbox.
func (s *Sandbox) ReadFile(_ context.Context, path string, maxBytes int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("fake sandbox: %s: no such file", path)
	}
	if maxBytes > 0 && len(b) > maxBytes {
		b = b[:maxBytes]
	}
	return append([]byte(nil), b...), nil
}

// Destroy implements sandbox.Sandbox. It is idempotent.
func (s *Sandbox) Destroy(context.Context) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	if _, ok := s.m.sandboxes[s.id]; ok {
		delete(s.m.sandboxes, s.id)
		s.m.destroyed = append(s.m.destroyed, s.id)
	}
	return s.m.DestroyErr
}

// Execs returns the commands run so far.
func (s *Sandbox) Execs() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]string(nil), s.execs...)
}

// SampleRepoExec simulates `make build` for agent/fixtures/sample-repo: the build fails until the
// Makefile creates the dist directory, which is the fix the scripted agent applies.
func SampleRepoExec(s *Sandbox, cmd []string, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	if len(cmd) >= 2 && cmd[0] == "make" && cmd[1] == "build" {
		mk, err := s.ReadFile(context.Background(), ProjectFile("Makefile"), 0)
		if err != nil {
			return sandbox.ExecResult{ExitCode: 2, Stderr: "make: *** No rule to make target 'build'.  Stop.\n"}, nil
		}
		if !bytes.Contains(mk, []byte("mkdir -p dist")) {
			return sandbox.ExecResult{ExitCode: 2, Stderr: "sh: 1: cannot create dist/out.txt: Directory nonexistent\nmake: *** [Makefile:2: build] Error 2\n"}, nil
		}
		return sandbox.ExecResult{Stdout: "mkdir -p dist\ncat src/main.txt > dist/out.txt\n"}, nil
	}
	if len(cmd) > 0 && cmd[0] == "cat" {
		b, err := s.ReadFile(context.Background(), strings.TrimSpace(cmd[len(cmd)-1]), 0)
		if err != nil {
			return sandbox.ExecResult{ExitCode: 1, Stderr: err.Error() + "\n"}, nil
		}
		return sandbox.ExecResult{Stdout: string(b)}, nil
	}
	return sandbox.ExecResult{ExitCode: 127, Stderr: fmt.Sprintf("fake sandbox: command not found: %s\n", strings.Join(cmd, " "))}, errors.New("unsupported command in fake sandbox")
}

// ProjectFile returns a path inside the sandbox project directory.
func ProjectFile(name string) string { return sandbox.ProjectDir + "/" + name }
