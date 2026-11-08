package jobs_test

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/agent/fixtures"
	"github.com/synapsbranch-ux/yari.run/agent/prompts"
	agentfake "github.com/synapsbranch-ux/yari.run/internal/agent/fake"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
	sandboxdocker "github.com/synapsbranch-ux/yari.run/internal/sandbox/docker"
)

// The scripted agent in a real hardened container: it builds the sample repo with real commands,
// calls the MCP server and ends at waiting_approval. Needs YARI_TEST_DOCKER=1 and the agent image
// (make agent-image). The LiteLLM gateway is faked here, so no model key and no proxy is needed.
func TestFakeRuntimeInARealHardenedContainer(t *testing.T) {
	if os.Getenv("YARI_TEST_DOCKER") != "1" {
		t.Skip("YARI_TEST_DOCKER=1 is not set")
	}
	image := os.Getenv("YARI_AGENT_IMAGE")
	if image == "" {
		image = "yari-agent:dev"
	}
	h := newSessionHarness(t)
	m, err := sandboxdocker.New(context.Background(), sandboxdocker.Config{Runtime: os.Getenv("YARI_SANDBOX_RUNTIME")})
	require.NoError(t, err)
	sample, err := fs.Sub(fixtures.SampleRepo, "sample-repo")
	require.NoError(t, err)

	args, _ := h.start(h.org.ProdEnvID, "make the build pass and deploy it")
	w := &jobs.SessionWorker{
		Pool: h.pool, Config: config.Config{Env: "dev", AgentImage: image, AgentSessionTTL: 5 * time.Minute},
		Sandboxes: m, Gateway: h.gw, Runtime: &agentfake.Runtime{MCPURL: h.mcpURL}, Live: h.live, Prompt: prompts.System, SampleRepo: sample,
	}
	require.NoError(t, w.Work(context.Background(), &river.Job[jobs.RunAgentSessionArgs]{Args: args}))

	s := h.session(args.SessionID)
	require.Equal(t, "waiting_approval", s.Status)
	var sawFailure, sawFix bool
	for _, st := range h.steps(args.SessionID) {
		sawFailure = sawFailure || st.Summary == "make build: exit 2"
		sawFix = sawFix || st.Summary == "make build: ok"
	}
	require.True(t, sawFailure && sawFix, "the real build failed first and passed after the fix")

	list, err := m.Client().ContainerList(context.Background(), client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "yari.session="+args.SessionID.String())})
	require.NoError(t, err)
	require.Empty(t, list.Items, "the sandbox container is gone")
	require.True(t, h.gw.wasRevoked(s.GatewayKeyAlias.String))
}
