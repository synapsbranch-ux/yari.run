package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands/client"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}

// The request body below is the exact JSON verified against agent-server 1.60.1
// (see testdata/start_conversation_request.json and agent/image/README.md).
func TestStartRequestMatchesVerifiedBody(t *testing.T) {
	cfg := client.ConversationConfig{
		LLMModel:      "openai/yari-default",
		LLMBaseURL:    "http://litellm.internal:4000/v1",
		LLMAPIKey:     "sk-session-virtual-key-placeholder",
		MCPURL:        "http://yari-mcp.internal:8080/mcp",
		MCPToken:      "yari-mcp-session-token-placeholder",
		WorkingDir:    "/workspace/project",
		MaxIterations: 25,
		NumRetries:    2,
	}
	got, err := json.Marshal(cfg.StartRequest())
	require.NoError(t, err)
	require.JSONEq(t, string(readTestdata(t, "start_conversation_request.json")), string(got))
}

func TestEventTestdataDecodes(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "event_*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	wantKind := map[string]string{
		"event_system_prompt.json":                      client.KindSystemPrompt,
		"event_message_user.json":                       client.KindMessage,
		"event_message_agent.json":                      client.KindMessage,
		"event_action_terminal.json":                    client.KindAction,
		"event_observation_terminal.json":               client.KindObservation,
		"event_conversation_error_llm_unreachable.json": client.KindConvError,
		"event_conversation_error_max_iterations.json":  client.KindConvError,
		"event_pause.json":                              client.KindPause,
		"event_state_running.json":                      client.KindStateUpdate,
		"event_state_finished.json":                     client.KindStateUpdate,
		"event_state_error.json":                        client.KindStateUpdate,
		"event_state_paused.json":                       client.KindStateUpdate,
		"event_state_stats.json":                        client.KindStateUpdate,
		"event_state_full_state.json":                   client.KindStateUpdate,
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			raw := readTestdata(t, name)
			var ev client.StreamEvent
			require.NoError(t, json.Unmarshal(raw, &ev))
			ev.Raw = raw
			require.NotEmpty(t, ev.ID)
			require.NotEmpty(t, ev.Timestamp)
			require.Equal(t, wantKind[name], ev.Kind)

			var generic client.Event
			require.NoError(t, json.Unmarshal(raw, &generic))
			require.Equal(t, ev.Kind, generic.Kind)
			require.NotEmpty(t, generic.AdditionalProperties, "extra fields must be preserved")
		})
	}

	status := map[string]client.ConversationExecutionStatus{
		"event_state_running.json":  client.Running,
		"event_state_finished.json": client.Finished,
		"event_state_error.json":    client.Error,
		"event_state_paused.json":   client.Paused,
	}
	for name, want := range status {
		raw := readTestdata(t, name)
		var ev client.StreamEvent
		require.NoError(t, json.Unmarshal(raw, &ev))
		ev.Raw = raw
		got, ok := ev.ExecutionStatus()
		require.True(t, ok, name)
		require.Equal(t, want, got, name)
	}
}

func TestEventsPageAndConversationInfoDecode(t *testing.T) {
	var page client.EventPage
	require.NoError(t, json.Unmarshal(readTestdata(t, "events_page.json"), &page))
	require.Greater(t, len(page.Items), 3)
	require.Equal(t, client.KindSystemPrompt, page.Items[0].Kind)

	var info client.ConversationInfo
	require.NoError(t, json.Unmarshal(readTestdata(t, "conversation_info_created.json"), &info))
	require.NotEmpty(t, info.Id)
	require.Equal(t, client.Idle, info.ExecutionStatus)
	// The server never echoes the LLM key or MCP headers back.
	require.NotContains(t, string(readTestdata(t, "conversation_info_created.json")), "sk-")
}

func TestAuthedClientSendsSessionKey(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(client.SessionAPIKeyHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	c, err := client.NewAuthedClient(srv.URL+"/", "test-session-key-placeholder", nil)
	require.NoError(t, err)
	resp, err := c.HealthWithResponse(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ok", resp.JSON200.Status)
	require.Equal(t, "test-session-key-placeholder", gotKey)
}

// A hermetic stand-in for the agent server's /sockets/events/{id} endpoint: it checks the
// first-message auth frame, replays one stored event and then streams one live event.
func TestStreamAuthFrameAndEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/sockets/events/11111111-1111-1111-1111-111111111111", r.URL.Path)
		require.Equal(t, "all", r.URL.Query().Get("resend_mode"))
		require.Empty(t, r.URL.Query().Get("session_api_key"), "the key must not be put in the URL")
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		ctx := r.Context()
		_, first, err := conn.Read(ctx)
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"auth","session_api_key":"test-session-key-placeholder"}`, string(first))
		require.NoError(t, conn.Write(ctx, websocket.MessageText, readTestdata(t, "event_message_user.json")))
		require.NoError(t, conn.Write(ctx, websocket.MessageText, readTestdata(t, "event_state_finished.json")))
		conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := client.Dial(ctx, srv.URL,
		"test-session-key-placeholder", "11111111-1111-1111-1111-111111111111", client.StreamOptions{ResendAll: true})
	require.NoError(t, err)
	defer s.Close()

	ev, err := s.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, client.KindMessage, ev.Kind)
	require.Equal(t, "user", ev.Source)

	ev, err = s.Next(ctx)
	require.NoError(t, err)
	st, ok := ev.ExecutionStatus()
	require.True(t, ok)
	require.Equal(t, client.Finished, st)
	require.True(t, st.Settled())

	_, err = s.Next(ctx)
	require.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err))
}
