package openhands_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/agent/prompts"
	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
	sandboxdocker "github.com/synapsbranch-ux/yari.run/internal/sandbox/docker"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Needs YARI_TEST_OPENHANDS=1, Docker, the agent image (make agent-image) and the compose agent
// profile (litellm and yari-mcp on yari-agent-net): docker compose --profile agent up -d --wait.
//
// Without a model key configured in LiteLLM the test stops after verifying that the conversation is
// created, the event stream works, the yari MCP tools load in the agent, and Stop ends the run
// cleanly. With YARI_TEST_OPENHANDS_MODEL=1 (a real provider key in LiteLLM) it also waits for the
// agent to answer.
func TestRuntimeStartsAConversationInAHardenedSandboxAndStopsCleanly(t *testing.T) {
	if os.Getenv("YARI_TEST_OPENHANDS") != "1" {
		t.Skip("YARI_TEST_OPENHANDS=1 is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	gw := &gateway.Client{BaseURL: env("LITELLM_URL", "http://localhost:4000"), MasterKey: env("LITELLM_MASTER_KEY", "change-me")}
	sessionID := "itest-openhands-" + time.Now().Format("150405")
	key, err := gw.CreateSessionKey(ctx, sessionID, "claude-sonnet-5-5", 0.5, 10*time.Minute)
	require.NoError(t, err, "start litellm: docker compose --profile agent up -d --wait")
	t.Cleanup(func() { _ = gw.Revoke(context.Background(), key.Alias) })

	mgr, err := sandboxdocker.New(ctx, sandboxdocker.Config{Runtime: os.Getenv("YARI_SANDBOX_RUNTIME")})
	require.NoError(t, err)
	agentKey := sandbox.SessionKeyPrefix + "itest0123456789abcdefghijkl"
	sb, err := mgr.Create(ctx, sandbox.Spec{
		SessionID: sessionID, Image: env("YARI_AGENT_IMAGE", "yari-agent:dev"), TTL: 10 * time.Minute,
		WritablePaths: sandbox.AgentServerWritablePaths, Env: map[string]string{sandbox.AgentServerKeyEnv: agentKey},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Destroy(context.Background()) })

	spec := agent.SessionSpec{
		SessionID: sessionID, EnvironmentID: "00000000-0000-0000-0000-000000000000", Goal: "Run `echo hello` in the terminal and then say done.",
		SystemPrompt: prompts.System, Model: "claude-sonnet-5-5", LLMBaseURL: env("YARI_LLM_BASE_URL", "http://litellm:4000"), LLMKey: key.Key,
		MCPURL: env("YARI_MCP_URL", "http://yari-mcp:8090/mcp"), MCPToken: "yari_itesttokenitesttoken", AgentServerKey: agentKey, MaxIterations: 5,
	}
	rt := &openhands.Runtime{NumRetries: 1}
	run, err := rt.Start(ctx, spec, sb)
	require.NoError(t, err, "the conversation is created")
	if c, ok := run.(interface{ ConversationID() string }); ok {
		require.NotEmpty(t, c.ConversationID())
	}

	var summaries []string
	var sawInit, sawMCPError bool
	deadline := time.After(90 * time.Second)
collect:
	for {
		select {
		case ev, ok := <-run.Events():
			if !ok {
				break collect
			}
			summaries = append(summaries, string(ev.Kind)+": "+ev.Summary)
			if ev.Summary == "agent initialized" {
				sawInit = true
			}
			if strings.Contains(ev.Summary, "MCP tools did not load") {
				sawMCPError = true
			}
			// Without a model key the run ends with an error soon after; with one it answers.
			if ev.Kind == agent.EventError && !strings.Contains(ev.Summary, "MCP") {
				break collect
			}
			if os.Getenv("YARI_TEST_OPENHANDS_MODEL") != "1" && sawInit {
				break collect
			}
		case <-deadline:
			break collect
		}
	}
	t.Logf("events seen: %v", summaries)
	require.True(t, sawInit, "the event stream delivered the agent's system prompt")
	require.False(t, sawMCPError, "the yari MCP tools loaded in the agent")

	stopCtx, stopCancel := context.WithTimeout(ctx, 30*time.Second)
	defer stopCancel()
	require.NoError(t, run.Stop(stopCtx))
	select {
	case err := <-run.Done():
		// A stop is a clean end. A run that already failed on the missing model key reports that error.
		if err != nil {
			require.NotContains(t, err.Error(), "stream lost")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the run did not end after Stop")
	}
	require.NoError(t, sb.Destroy(ctx))
}
