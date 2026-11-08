package openhands_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands"
	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands/client"
)

// The events in client/testdata were captured from a real agent server.
func load(t *testing.T, name string) client.StreamEvent {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("client", "testdata", name))
	require.NoError(t, err)
	ev := client.StreamEvent{Raw: raw}
	require.NoError(t, jsonUnmarshal(raw, &ev))
	return ev
}

func TestMapRealEvents(t *testing.T) {
	cases := []struct {
		file     string
		kind     agent.EventKind
		tool     string
		location string
		summary  string
	}{
		{"event_action_terminal.json", agent.EventAction, "terminal", agent.LocationSandbox, "print identity"},
		{"event_observation_terminal.json", agent.EventObservation, "terminal", agent.LocationSandbox, ": exit 0"},
		{"event_message_agent.json", agent.EventMessage, "", agent.LocationNone, "agent: Hello from the fake LLM."},
		{"event_message_user.json", agent.EventMessage, "", agent.LocationNone, "user: "},
		{"event_conversation_error_llm_unreachable.json", agent.EventError, "", agent.LocationNone, "LLMServiceUnavailableError"},
		{"event_conversation_error_max_iterations.json", agent.EventError, "", agent.LocationNone, "MaxIterationsReached"},
		{"event_pause.json", agent.EventStatus, "", agent.LocationNone, "paused"},
		{"event_state_finished.json", agent.EventStatus, "", agent.LocationNone, "execution status: finished"},
		{"event_state_running.json", agent.EventStatus, "", agent.LocationNone, "execution status: running"},
		{"event_system_prompt.json", agent.EventStatus, "", agent.LocationNone, "agent initialized"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			ev, ok := openhands.Map(load(t, c.file))
			require.True(t, ok)
			require.Equal(t, c.kind, ev.Kind)
			require.Equal(t, c.tool, ev.Tool)
			require.Equal(t, c.location, ev.Location)
			require.Contains(t, ev.Summary, c.summary)
			require.NotEmpty(t, ev.ExternalID)
			require.True(t, json.Valid(ev.Raw))
			require.LessOrEqual(t, len(ev.Summary), 310)
		})
	}
}

func TestStatisticsUpdatesAreDropped(t *testing.T) {
	_, ok := openhands.Map(load(t, "event_state_stats.json"))
	require.False(t, ok)
	_, ok = openhands.Map(load(t, "event_state_full_state.json"))
	require.False(t, ok)
}

func TestYariToolNamesAreNormalized(t *testing.T) {
	for in, want := range map[string]string{
		"plan.propose": "plan.propose", "plan_propose": "plan.propose", "yari_plan_propose": "plan.propose",
		"yari-plan_apply": "plan.apply", "spec_read": "spec.read", "yari_deployments_status": "deployments.status",
		"terminal": "terminal", "file_editor": "file_editor", "something_else": "something_else",
	} {
		require.Equal(t, want, openhands.NormalizeTool(in), in)
	}
}

func TestToolLocation(t *testing.T) {
	obs := client.StreamEvent{Raw: []byte(`{"id":"1","kind":"ObservationEvent","source":"environment","tool_name":"yari_plan_propose","observation":{"content":[{"type":"text","text":"{\"plan_id\":\"p1\"}"}],"is_error":false}}`)}
	ev, ok := openhands.Map(obs)
	require.True(t, ok)
	require.Equal(t, "plan.propose", ev.Tool)
	require.Equal(t, agent.LocationYari, ev.Location, "MCP tools are yari tools")
	require.Contains(t, string(ev.Raw), `"result"`, "the session job reads the plan id from result")
}

func TestGuardrailRefusalReachesTheSummary(t *testing.T) {
	obs := client.StreamEvent{Raw: []byte(`{"id":"2","kind":"ObservationEvent","source":"environment","tool_name":"plan_propose","observation":{"content":[{"type":"text","text":"guardrail: agents may not delete postgres services"}],"is_error":true}}`)}
	ev, ok := openhands.Map(obs)
	require.True(t, ok)
	require.Contains(t, ev.Summary, "guardrail: agents may not delete postgres", "the session job turns this into a guardrail step")
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
