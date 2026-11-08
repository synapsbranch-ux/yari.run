// Package docker implements sandbox.Manager with the Docker Engine Go SDK.
//
// Every sandbox is a hardened container: non-root user, read-only root filesystem with tmpfs
// for the writable paths, all capabilities dropped, no-new-privileges, CPU, memory and pids
// limits, attached only to the internal yari-agent-net network (or to no network), no host
// mounts and never the Docker socket. The gVisor runtime (runsc) is used when the daemon has it.
package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// Labels set on every sandbox container.
const (
	LabelSession = "yari.session"
	LabelExpires = "yari.expires"
)

// Config configures the manager.
type Config struct {
	// Runtime is the preferred OCI runtime, normally "runsc" (gVisor).
	Runtime string
	// RequireRuntime refuses to start sandboxes when Runtime is not installed. When false (dev
	// only) the manager falls back to the default runtime and logs a loud warning.
	RequireRuntime bool
	// AgentPort is the port the agent server listens on inside the sandbox.
	AgentPort int
	// User overrides the image's user, as uid:gid. Empty keeps the image's non-root user.
	User string
}

// Manager is the Docker implementation of sandbox.Manager.
type Manager struct {
	cli     *client.Client
	cfg     Config
	runtime string // resolved: runsc, or "" for the daemon default
}

// New connects to the Docker daemon from the environment and resolves the OCI runtime.
func New(ctx context.Context, cfg Config) (*Manager, error) {
	cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("sandbox: docker client: %w", err)
	}
	return NewWithClient(ctx, cli, cfg)
}

// NewWithClient is New with an existing client.
func NewWithClient(ctx context.Context, cli *client.Client, cfg Config) (*Manager, error) {
	if cfg.AgentPort == 0 {
		cfg.AgentPort = 8000
	}
	info, err := cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return nil, fmt.Errorf("sandbox: reach the docker daemon: %w", err)
	}
	m := &Manager{cli: cli, cfg: cfg}
	if cfg.Runtime != "" {
		if _, ok := info.Info.Runtimes[cfg.Runtime]; ok {
			m.runtime = cfg.Runtime
		} else if cfg.RequireRuntime {
			return nil, fmt.Errorf("sandbox: OCI runtime %q is not installed and is required", cfg.Runtime)
		} else {
			slog.Warn("SANDBOX ISOLATION REDUCED: the OCI runtime is not installed, falling back to the daemon default (runc). "+
				"Containers share the host kernel without gVisor. Development only.", "wanted", cfg.Runtime)
		}
	}
	return m, nil
}

// Runtime returns the OCI runtime sandboxes use, or "" for the daemon default.
func (m *Manager) Runtime() string { return m.runtime }

// Client returns the underlying Docker client, for tests.
func (m *Manager) Client() *client.Client { return m.cli }

func containerName(session string) string {
	s := strings.NewReplacer("/", "-", ":", "-").Replace(session)
	return "yari-sandbox-" + s
}

// Create starts a hardened container for the spec.
func (m *Manager) Create(ctx context.Context, spec sandbox.Spec) (sandbox.Sandbox, error) {
	spec = spec.WithDefaults()
	if spec.SessionID == "" || spec.Image == "" {
		return nil, errors.New("sandbox: session id and image are required")
	}
	if err := sandbox.ValidateEnv(spec.Env); err != nil {
		return nil, err
	}

	user, err := m.nonRootUser(ctx, spec.Image)
	if err != nil {
		return nil, err
	}

	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	pids := int64(spec.PidsLimit)
	init := true
	// Docker mounts tmpfs noexec by default. The agent server is a bundled binary that unpacks its
	// shared libraries into /tmp and maps them executable, and builds run in /workspace, so exec is
	// set explicitly. nosuid and nodev stay on. This is the one hardening item relaxed for the
	// agent server; see the README.
	tmpOpts := func(sizeMB int) string { return fmt.Sprintf("rw,exec,nosuid,nodev,size=%dm,mode=1777", sizeMB) }
	tmpfs := map[string]string{"/tmp": tmpOpts(512), "/workspace": tmpOpts(spec.DiskMB)}
	for _, p := range spec.WritablePaths {
		if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			return nil, fmt.Errorf("sandbox: invalid writable path %q", p)
		}
		tmpfs[p] = tmpOpts(512)
	}

	hc := &container.HostConfig{
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		Privileged:     false,
		Init:           &init,
		Tmpfs:          tmpfs,
		Runtime:        m.runtime,
		Resources: container.Resources{
			NanoCPUs:   int64(spec.CPUs * 1e9),
			Memory:     int64(spec.MemoryMB) << 20,
			MemorySwap: int64(spec.MemoryMB) << 20, // no swap
			PidsLimit:  &pids,
		},
		// No Binds, no Mounts, no port publishing, no docker socket.
	}
	var nc *network.NetworkingConfig
	switch spec.Network {
	case sandbox.NetworkNone:
		hc.NetworkMode = "none"
	default:
		hc.NetworkMode = container.NetworkMode(sandbox.AgentNetworkName)
		nc = &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{sandbox.AgentNetworkName: {}}}
	}

	cfg := &container.Config{
		Image:      spec.Image,
		Env:        env,
		User:       user,
		WorkingDir: "/workspace",
		Labels: map[string]string{
			LabelSession: spec.SessionID,
			LabelExpires: strconv.FormatInt(time.Now().Add(spec.TTL).Unix(), 10),
		},
	}
	created, err := m.cli.ContainerCreate(ctx, client.ContainerCreateOptions{Config: cfg, HostConfig: hc, NetworkingConfig: nc, Name: containerName(spec.SessionID)})
	if err != nil {
		return nil, fmt.Errorf("sandbox: create container: %w", err)
	}
	if _, err := m.cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		_, _ = m.cli.ContainerRemove(context.WithoutCancel(ctx), created.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		return nil, fmt.Errorf("sandbox: start container: %w", err)
	}
	return m.sandboxFor(ctx, created.ID)
}

// nonRootUser returns the user to run as, and refuses images that would run as root.
func (m *Manager) nonRootUser(ctx context.Context, image string) (string, error) {
	user := m.cfg.User
	if user == "" {
		res, err := m.cli.ImageInspect(ctx, image)
		if err != nil {
			return "", fmt.Errorf("sandbox: inspect image %q (build it with make agent-image): %w", image, err)
		}
		if res.Config != nil {
			user = res.Config.User
		}
	}
	name := strings.SplitN(user, ":", 2)[0]
	if name == "" || name == "0" || name == "root" {
		return "", fmt.Errorf("sandbox: image %q would run as root, refusing to start", image)
	}
	return user, nil
}

// Get finds a sandbox by container id.
func (m *Manager) Get(ctx context.Context, id string) (sandbox.Sandbox, error) {
	return m.sandboxFor(ctx, id)
}

func (m *Manager) sandboxFor(ctx context.Context, id string) (*Sandbox, error) {
	res, err := m.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("sandbox: inspect %s: %w", id, err)
	}
	s := &Sandbox{m: m, id: res.Container.ID}
	if ns := res.Container.NetworkSettings; ns != nil {
		if ep := ns.Networks[sandbox.AgentNetworkName]; ep != nil && ep.IPAddress.IsValid() {
			s.endpoint = fmt.Sprintf("http://%s:%d", ep.IPAddress, m.cfg.AgentPort)
		}
	}
	return s, nil
}

// Reap removes sandboxes whose yari.expires label is in the past.
func (m *Manager) Reap(ctx context.Context) (int, error) {
	list, err := m.cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", LabelSession)})
	if err != nil {
		return 0, fmt.Errorf("sandbox: list: %w", err)
	}
	n := 0
	for _, c := range list.Items {
		exp, err := strconv.ParseInt(c.Labels[LabelExpires], 10, 64)
		if err != nil || time.Now().Unix() < exp {
			continue
		}
		if _, err := m.cli.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !cerrdefs.IsNotFound(err) {
			return n, fmt.Errorf("sandbox: reap %s: %w", c.ID, err)
		}
		n++
	}
	return n, nil
}

// Sandbox is one running container.
type Sandbox struct {
	m        *Manager
	id       string
	endpoint string
}

// ID implements sandbox.Sandbox.
func (s *Sandbox) ID() string { return s.id }

// Endpoint implements sandbox.Sandbox: the agent server URL on yari-agent-net.
func (s *Sandbox) Endpoint() string { return s.endpoint }

// limitWriter keeps at most max bytes and notes when it dropped some.
type limitWriter struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	room := w.max - w.buf.Len()
	if room <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		w.buf.Write(p[:room])
		w.truncated = true
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

// Exec implements sandbox.Sandbox.
func (s *Sandbox) Exec(ctx context.Context, cmd []string, opts sandbox.ExecOptions) (sandbox.ExecResult, error) {
	return s.execWithStdin(ctx, cmd, opts, nil)
}

func (s *Sandbox) execWithStdin(ctx context.Context, cmd []string, opts sandbox.ExecOptions, stdin []byte) (sandbox.ExecResult, error) {
	if len(cmd) == 0 {
		return sandbox.ExecResult{}, errors.New("sandbox: empty command")
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	max := opts.MaxBytes
	if max <= 0 {
		max = sandbox.DefaultMaxBytes
	}
	dir := opts.Dir
	if dir == "" {
		dir = "/workspace"
	}
	created, err := s.m.cli.ExecCreate(ctx, s.id, client.ExecCreateOptions{Cmd: cmd, WorkingDir: dir, AttachStdin: stdin != nil, AttachStdout: true, AttachStderr: true})
	if err != nil {
		return sandbox.ExecResult{}, fmt.Errorf("sandbox: exec create: %w", err)
	}
	att, err := s.m.cli.ExecAttach(ctx, created.ID, client.ExecAttachOptions{})
	if err != nil {
		return sandbox.ExecResult{}, fmt.Errorf("sandbox: exec attach: %w", err)
	}
	defer att.Close()

	if stdin != nil {
		go func() {
			_, _ = att.Conn.Write(stdin)
			_ = att.CloseWrite()
		}()
	}
	out, errOut := &limitWriter{max: max}, &limitWriter{max: max}
	done := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(out, errOut, att.Reader)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) {
			return sandbox.ExecResult{}, fmt.Errorf("sandbox: exec read: %w", err)
		}
	case <-ctx.Done():
		return sandbox.ExecResult{}, fmt.Errorf("sandbox: exec %q: %w", cmd[0], ctx.Err())
	}
	insp, err := s.m.cli.ExecInspect(ctx, created.ID, client.ExecInspectOptions{})
	if err != nil {
		return sandbox.ExecResult{}, fmt.Errorf("sandbox: exec inspect: %w", err)
	}
	return sandbox.ExecResult{ExitCode: insp.ExitCode, Stdout: out.buf.String(), Stderr: errOut.buf.String(), Truncated: out.truncated || errOut.truncated}, nil
}

// WriteFile implements sandbox.Sandbox.
//
// It streams the data through an exec instead of the archive API, because docker cp cannot reach
// tmpfs mounts, which is where the writable paths of a read-only sandbox live. The file is owned
// by the container user, and parent directories are created.
func (s *Sandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return fmt.Errorf("sandbox: path %q must be absolute without ..", path)
	}
	if data == nil {
		data = []byte{}
	}
	res, err := s.execWithStdin(ctx, []string{"sh", "-c", `mkdir -p "$(dirname "$1")" && cat > "$1"`, "sh", path},
		sandbox.ExecOptions{Dir: "/", Timeout: 30 * time.Second, MaxBytes: 4096}, data)
	if err != nil {
		return fmt.Errorf("sandbox: write %s: %w", path, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("sandbox: write %s: %s", path, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// ReadFile implements sandbox.Sandbox.
func (s *Sandbox) ReadFile(ctx context.Context, path string, maxBytes int) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return nil, fmt.Errorf("sandbox: path %q must be absolute without ..", path)
	}
	if maxBytes <= 0 {
		maxBytes = sandbox.DefaultMaxBytes
	}
	res, err := s.Exec(ctx, []string{"cat", path}, sandbox.ExecOptions{Dir: "/", Timeout: 30 * time.Second, MaxBytes: maxBytes})
	if err != nil {
		return nil, fmt.Errorf("sandbox: read %s: %w", path, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("sandbox: read %s: %s", path, strings.TrimSpace(res.Stderr))
	}
	return []byte(res.Stdout), nil
}

// Destroy implements sandbox.Sandbox. It is idempotent.
func (s *Sandbox) Destroy(ctx context.Context) error {
	_, err := s.m.cli.ContainerRemove(context.WithoutCancel(ctx), s.id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("sandbox: destroy %s: %w", s.id, err)
	}
	return nil
}
