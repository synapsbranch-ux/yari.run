package client_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands/client"
)

// TestAgentServerIntegration drives a real, running agent server. It is skipped unless
// YARI_TEST_OPENHANDS=1. Required environment:
//
//	OPENHANDS_TEST_URL      base URL of the agent server, for example http://172.20.0.3:8000
//	OPENHANDS_TEST_API_KEY  the server's OH_SESSION_API_KEYS_0 value
//
// Optional:
//
//	OPENHANDS_TEST_LLM_URL  fake LLM base URL (default http://fake-llm.invalid:9999/v1).
//	                        It is never called by this test until RunConversation, and the
//	                        test accepts both an unreachable URL (conversation ends in
//	                        "error") and a working fake (conversation ends "finished").
//
// Example (see agent/image/README.md for the hardened docker run flags):
//
//	YARI_TEST_OPENHANDS=1 OPENHANDS_TEST_URL=http://172.20.0.3:8000 \
//	  OPENHANDS_TEST_API_KEY=placeholder-session-key go test ./internal/agent/openhands/client -run Integration -v
func TestAgentServerIntegration(t *testing.T) {
	if os.Getenv("YARI_TEST_OPENHANDS") != "1" {
		t.Skip("set YARI_TEST_OPENHANDS=1 (and OPENHANDS_TEST_URL, OPENHANDS_TEST_API_KEY) to run")
	}
	baseURL := os.Getenv("OPENHANDS_TEST_URL")
	apiKey := os.Getenv("OPENHANDS_TEST_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("OPENHANDS_TEST_URL and OPENHANDS_TEST_API_KEY are not set: the runtime test in internal/agent/openhands covers the real server")
	}
	llmURL := os.Getenv("OPENHANDS_TEST_LLM_URL")
	if llmURL == "" {
		llmURL = "http://fake-llm.invalid:9999/v1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := client.NewAuthedClient(baseURL, apiKey, nil)
	require.NoError(t, err)

	t.Run("health", func(t *testing.T) {
		h, err := c.HealthWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, h.StatusCode())
		require.NotNil(t, h.JSON200)
		require.Equal(t, "ok", h.JSON200.Status)

		si, err := c.ServerInfoWithResponse(ctx)
		require.NoError(t, err)
		require.NotNil(t, si.JSON200)
		t.Logf("agent server version=%v tools=%v", deref(si.JSON200.Version), si.JSON200.UsableTools)
	})

	t.Run("wrong session key is rejected", func(t *testing.T) {
		bad, err := client.NewAuthedClient(baseURL, "definitely-not-the-key", nil)
		require.NoError(t, err)
		r, err := bad.SearchEventsWithResponse(ctx, "00000000-0000-0000-0000-000000000000", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, r.StatusCode())
	})

	t.Run("websocket with wrong session key is closed with 4001", func(t *testing.T) {
		s, err := client.Dial(ctx, baseURL, "definitely-not-the-key", "00000000-0000-0000-0000-000000000000", client.StreamOptions{})
		require.NoError(t, err) // the upgrade is accepted first, the key is checked on the first frame
		defer s.Close()
		_, err = s.Next(ctx)
		require.Error(t, err)
		require.Equal(t, websocket.StatusCode(4001), websocket.CloseStatus(err), "got %v", err)
	})

	// Create. No initial message, so creation never calls the model.
	cfg := client.ConversationConfig{
		LLMModel:      "openai/yari-fake-model",
		LLMBaseURL:    llmURL,
		LLMAPIKey:     "sk-session-virtual-key-placeholder",
		MCPURL:        "http://fake-mcp.invalid:8080/mcp",
		MCPToken:      "yari-mcp-session-token-placeholder",
		WorkingDir:    "/workspace/project",
		MaxIterations: 25,
		NumRetries:    1,
	}
	created, err := c.StartConversationWithResponse(ctx, cfg.StartRequest())
	require.NoError(t, err)
	info := created.JSON201
	if info == nil {
		info = created.JSON200
	}
	require.NotNil(t, info, "status %d body %s", created.StatusCode(), created.Body)
	require.Equal(t, client.Idle, info.ExecutionStatus)
	require.NotEmpty(t, info.Id)
	require.NotNil(t, info.MaxIterations)
	require.Equal(t, 25, *info.MaxIterations)
	id := info.Id
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		_, _ = c.DeleteConversationWithResponse(cctx, id)
	})

	// Secrets must never be echoed back by the server.
	require.NotContains(t, string(created.Body), "sk-session-virtual-key-placeholder")
	require.NotContains(t, string(created.Body), "yari-mcp-session-token-placeholder")

	// Open the event stream before sending anything, replaying history.
	stream, err := client.Dial(ctx, baseURL, apiKey, id, client.StreamOptions{ResendAll: true})
	require.NoError(t, err)
	defer stream.Close()

	// First message with run=false: stored, but the agent loop (and the model) stay idle.
	sent, err := c.SendMessageWithResponse(ctx, id, client.TextMessage("Say hello.", false))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, sent.StatusCode())

	seen := map[string]int{}
	var sawUserMessage bool
	readUntil(ctx, t, stream, func(ev client.StreamEvent) bool {
		seen[ev.Kind]++
		if ev.Kind == client.KindMessage && ev.Source == "user" {
			sawUserMessage = true
		}
		return sawUserMessage
	})
	require.True(t, sawUserMessage)
	require.Positive(t, seen[client.KindSystemPrompt], "resend_mode=all must replay the system prompt event")

	// REST pagination: two events per page until exhausted.
	var all []client.Event
	var pageID *string
	for pages := 0; pages < 50; pages++ {
		page, err := c.SearchEventsWithResponse(ctx, id, &client.SearchEventsParams{Limit: ptr(2), PageId: pageID})
		require.NoError(t, err)
		require.NotNil(t, page.JSON200, "status %d", page.StatusCode())
		require.LessOrEqual(t, len(page.JSON200.Items), 2)
		all = append(all, page.JSON200.Items...)
		if page.JSON200.NextPageId == nil || *page.JSON200.NextPageId == "" {
			break
		}
		pageID = page.JSON200.NextPageId
	}
	require.GreaterOrEqual(t, len(all), 2)
	cnt, err := c.CountEventsWithResponse(ctx, id, nil)
	require.NoError(t, err)
	require.NotNil(t, cnt.JSON200)
	require.Equal(t, len(all), *cnt.JSON200)

	// Resume (start the loop). With an unreachable LLM the run must fail gracefully:
	// status error plus a ConversationErrorEvent, and the server stays healthy.
	run, err := c.RunConversationWithResponse(ctx, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, run.StatusCode())

	var final client.ConversationExecutionStatus
	var sawConvError bool
	readUntil(ctx, t, stream, func(ev client.StreamEvent) bool {
		if ev.Kind == client.KindConvError {
			sawConvError = true
		}
		if st, ok := ev.ExecutionStatus(); ok && (st == client.Finished || st == client.Error || st == client.Stuck) {
			final = st
		}
		// The ConversationErrorEvent follows the "error" status update by a few ms.
		return final == client.Finished || final == client.Stuck || (final == client.Error && sawConvError)
	})
	switch final {
	case client.Error:
		require.True(t, sawConvError)
	case client.Finished:
	default:
		t.Fatalf("unexpected final status %q", final)
	}

	got, err := c.GetConversationWithResponse(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got.JSON200)
	require.Equal(t, final, got.JSON200.ExecutionStatus)

	// Pause on a settled conversation is accepted (no-op); follow-up message works.
	p, err := c.PauseConversationWithResponse(ctx, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, p.StatusCode())
	f, err := c.SendMessageWithResponse(ctx, id, client.TextMessage("follow up", false))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, f.StatusCode())

	// Delete, then the conversation is gone.
	d, err := c.DeleteConversationWithResponse(ctx, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, d.StatusCode())
	g, err := c.GetConversationWithResponse(ctx, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, g.StatusCode())
}

// readUntil reads events until stop returns true or the context/timeout ends.
func readUntil(ctx context.Context, t *testing.T, s *client.EventStream, stop func(client.StreamEvent) bool) {
	t.Helper()
	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for {
		ev, err := s.Next(rctx)
		require.NoError(t, err)
		t.Logf("event %-30s source=%-11s %s", ev.Kind, ev.Source, ev.Timestamp)
		if stop(ev) {
			return
		}
	}
}

func ptr[T any](v T) *T { return &v }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
