package docker_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
	sandboxdocker "github.com/synapsbranch-ux/yari.run/internal/sandbox/docker"
)

// Integration tests: need a Docker daemon, the yari-agent-net network with the litellm and
// yari-mcp containers on it (docker compose --profile agent up -d), and YARI_TEST_DOCKER=1.
// YARI_TEST_SANDBOX_IMAGE overrides the image (default: a tiny non-root alpine built on the fly).

func setup(t *testing.T) (*sandboxdocker.Manager, string) {
	t.Helper()
	if os.Getenv("YARI_TEST_DOCKER") != "1" {
		t.Skip("YARI_TEST_DOCKER=1 is not set")
	}
	image := os.Getenv("YARI_TEST_SANDBOX_IMAGE")
	if image == "" {
		image = "yari-sandbox-test:dev"
		base := os.Getenv("YARI_TEST_BASE_IMAGE")
		if base == "" {
			base = "alpine:3.22"
		}
		cmd := exec.Command("docker", "build", "-q", "-t", image, "-")
		cmd.Stdin = strings.NewReader("FROM " + base + "\nUSER 10001:10001\nCMD [\"sleep\", \"3600\"]\n")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	m, err := sandboxdocker.New(context.Background(), sandboxdocker.Config{Runtime: os.Getenv("YARI_SANDBOX_RUNTIME"), AgentPort: 8000})
	require.NoError(t, err)
	return m, image
}

func create(t *testing.T, m *sandboxdocker.Manager, image, session string, spec sandbox.Spec) sandbox.Sandbox {
	t.Helper()
	spec.SessionID, spec.Image = session, image
	sb, err := m.Create(context.Background(), spec)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Destroy(context.Background()) })
	return sb
}

func run(t *testing.T, sb sandbox.Sandbox, cmd ...string) sandbox.ExecResult {
	t.Helper()
	res, err := sb.Exec(context.Background(), cmd, sandbox.ExecOptions{Timeout: 20 * time.Second})
	require.NoError(t, err)
	return res
}

func TestSandboxIsHardened(t *testing.T) {
	m, image := setup(t)
	sb := create(t, m, image, "itest-hardening", sandbox.Spec{CPUs: 1, MemoryMB: 256, PidsLimit: 64, DiskMB: 64})

	insp, err := m.Client().ContainerInspect(context.Background(), sb.ID(), client.ContainerInspectOptions{})
	require.NoError(t, err)
	hc := insp.Container.HostConfig
	require.True(t, hc.ReadonlyRootfs, "read-only root filesystem")
	require.Equal(t, []string{"ALL"}, hc.CapDrop, "all capabilities dropped")
	require.Contains(t, hc.SecurityOpt, "no-new-privileges")
	require.False(t, hc.Privileged)
	require.Empty(t, hc.Binds, "no host mounts")
	require.Empty(t, hc.Mounts, "no host mounts")
	require.Empty(t, hc.PortBindings, "no published ports")
	require.EqualValues(t, 256<<20, hc.Memory)
	require.EqualValues(t, 256<<20, hc.MemorySwap, "no swap")
	require.EqualValues(t, 1_000_000_000, hc.NanoCPUs)
	require.NotNil(t, hc.PidsLimit)
	require.EqualValues(t, 64, *hc.PidsLimit)
	require.Equal(t, sandbox.AgentNetworkName, string(hc.NetworkMode))
	require.Len(t, insp.Container.NetworkSettings.Networks, 1, "attached only to yari-agent-net")
	require.Contains(t, insp.Container.NetworkSettings.Networks, sandbox.AgentNetworkName)
	require.Equal(t, "itest-hardening", insp.Container.Config.Labels[sandboxdocker.LabelSession])
	require.NotEmpty(t, insp.Container.Config.Labels[sandboxdocker.LabelExpires])
	for _, mnt := range insp.Container.Mounts {
		require.NotContains(t, mnt.Source, "docker.sock", "never the docker socket")
		require.NotEqual(t, "bind", string(mnt.Type), "no bind mounts")
	}
	if rt := m.Runtime(); rt != "" {
		require.Equal(t, rt, hc.Runtime)
	}
	require.NotEmpty(t, sb.Endpoint())
}

func TestSandboxRuntimeBehavior(t *testing.T) {
	m, image := setup(t)
	sb := create(t, m, image, "itest-behavior", sandbox.Spec{})

	// Non-root, read-only root, writable workspace and tmp.
	require.NotEqual(t, "0", strings.TrimSpace(run(t, sb, "id", "-u").Stdout), "runs as a non-root user")
	require.NotZero(t, run(t, sb, "touch", "/etc/yari-should-fail").ExitCode, "root filesystem is read-only")
	require.Zero(t, run(t, sb, "touch", "/workspace/ok").ExitCode)
	require.Zero(t, run(t, sb, "touch", "/tmp/ok").ExitCode)

	// Files in and out.
	require.NoError(t, sb.WriteFile(context.Background(), "/workspace/src/nested/hello.txt", []byte("hello")))
	got, err := sb.ReadFile(context.Background(), "/workspace/src/nested/hello.txt", 0)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
	require.Equal(t, "hello", run(t, sb, "cat", "src/nested/hello.txt").Stdout, "exec starts in /workspace")

	// Output is capped.
	res, err := sb.Exec(context.Background(), []string{"sh", "-c", "yes | head -c 100000"}, sandbox.ExecOptions{MaxBytes: 1000})
	require.NoError(t, err)
	require.True(t, res.Truncated)
	require.Len(t, res.Stdout, 1000)

	// Exit codes come through.
	require.Equal(t, 7, run(t, sb, "sh", "-c", "exit 7").ExitCode)

	// Timeouts stop long commands.
	_, err = sb.Exec(context.Background(), []string{"sleep", "30"}, sandbox.ExecOptions{Timeout: time.Second})
	require.Error(t, err)
}

func TestSandboxHasNoInternetButReachesGatewayAndMCP(t *testing.T) {
	m, image := setup(t)
	sb := create(t, m, image, "itest-network", sandbox.Spec{})

	get := func(url string) sandbox.ExecResult { return run(t, sb, "wget", "-q", "-T", "4", "-O", "-", url) }
	require.Zero(t, get("http://litellm:4000/health/liveliness").ExitCode, "the model gateway is reachable")
	mcp := get("http://yari-mcp:8090/healthz")
	require.Zero(t, mcp.ExitCode, "the MCP server is reachable")
	require.Equal(t, "ok", mcp.Stdout)

	for _, url := range []string{"http://example.com", "https://example.com", "http://1.1.1.1", "http://8.8.8.8"} {
		require.NotZero(t, get(url).ExitCode, "outbound request to %s must fail", url)
	}
	require.NotZero(t, run(t, sb, "nslookup", "example.com").ExitCode, "no public DNS")
}

func TestNetworkNoneHasNoNetworkAtAll(t *testing.T) {
	m, image := setup(t)
	sb := create(t, m, image, "itest-none", sandbox.Spec{Network: sandbox.NetworkNone})
	require.NotZero(t, run(t, sb, "wget", "-q", "-T", "3", "-O", "-", "http://litellm:4000/health/liveliness").ExitCode)
}

func TestEnvValidationRejectsPlantedCredentials(t *testing.T) {
	m, image := setup(t)
	for name, env := range map[string]map[string]string{
		"anthropic": {"ANTHROPIC_API_KEY": "sk-ant-xxxxxxxxxxxx"},
		"database":  {"DATABASE_URL": "postgres://u:p@h/db"},
		"value":     {"HARMLESS": "AKIAABCDEFGHIJKLMNOP"},
	} {
		_, err := m.Create(context.Background(), sandbox.Spec{SessionID: "itest-env-" + name, Image: image, Env: env})
		require.Error(t, err, name)
	}
	list, err := m.Client().ContainerList(context.Background(), client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", sandboxdocker.LabelSession+"=itest-env-anthropic")})
	require.NoError(t, err)
	require.Empty(t, list.Items, "no container is created when validation fails")

	// The agent server key is the one allowed secret.
	sb := create(t, m, image, "itest-env-ok", sandbox.Spec{Env: map[string]string{sandbox.AgentServerKeyEnv: sandbox.SessionKeyPrefix + "abcdefghijklmnopqrstuvwx"}})
	require.Contains(t, run(t, sb, "env").Stdout, sandbox.AgentServerKeyEnv+"=")
}

func TestRootImagesAreRefused(t *testing.T) {
	if os.Getenv("YARI_TEST_DOCKER") != "1" {
		t.Skip("YARI_TEST_DOCKER=1 is not set")
	}
	m, err := sandboxdocker.New(context.Background(), sandboxdocker.Config{})
	require.NoError(t, err)
	cmd := exec.Command("docker", "build", "-q", "-t", "yari-sandbox-root:dev", "-")
	base := os.Getenv("YARI_TEST_BASE_IMAGE")
	if base == "" {
		base = "alpine:3.22"
	}
	cmd.Stdin = strings.NewReader("FROM " + base + "\nCMD [\"sleep\", \"3600\"]\n")
	out, err2 := cmd.CombinedOutput()
	require.NoError(t, err2, string(out))
	_, err = m.Create(context.Background(), sandbox.Spec{SessionID: "itest-root", Image: "yari-sandbox-root:dev"})
	require.ErrorContains(t, err, "root")
}

func TestDestroyAndReapCleanUp(t *testing.T) {
	m, image := setup(t)
	ctx := context.Background()
	count := func(session string) int {
		list, err := m.Client().ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", sandboxdocker.LabelSession+"="+session)})
		require.NoError(t, err)
		return len(list.Items)
	}

	sb := create(t, m, image, "itest-destroy", sandbox.Spec{})
	require.Equal(t, 1, count("itest-destroy"))
	require.NoError(t, sb.Destroy(ctx))
	require.Equal(t, 0, count("itest-destroy"))
	require.NoError(t, sb.Destroy(ctx), "destroy is idempotent")

	// Reap removes sandboxes past their TTL and leaves the others.
	create(t, m, image, "itest-expired", sandbox.Spec{TTL: time.Second})
	create(t, m, image, "itest-alive", sandbox.Spec{TTL: time.Hour})
	time.Sleep(2500 * time.Millisecond)
	n, err := m.Reap(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)
	require.Equal(t, 0, count("itest-expired"))
	require.Equal(t, 1, count("itest-alive"))
}
