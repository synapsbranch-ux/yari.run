package openhands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/synapsbranch-ux/yari.run/internal/agent"
	"github.com/synapsbranch-ux/yari.run/internal/agent/openhands/client"
)

// builtinTools are the harness tools that run inside the Sandbox. Every other tool an agent calls
// is an MCP tool, which means a yari.run tool.
var builtinTools = map[string]bool{
	"terminal": true, "file_editor": true, "task_tracker": true, "think": true, "finish": true,
	"browser": true, "browser_use": true, "str_replace_editor": true, "execute_bash": true,
}

// yariTools maps the names the harness may give MCP tools to the names yari.run documents.
var yariTools = []string{"spec.read", "deployments.status", "logs.query", "metrics.query", "plan.propose", "plan.apply", "plan.status"}

// NormalizeTool turns whatever name the harness uses for an MCP tool (a server prefix, underscores
// instead of dots) into the documented yari tool name, so steps and plan links are stable.
func NormalizeTool(name string) string {
	n := strings.ToLower(name)
	for _, y := range yariTools {
		flat := strings.ReplaceAll(y, ".", "_")
		if n == y || n == flat || strings.HasSuffix(n, "_"+flat) || strings.HasSuffix(n, "."+y) || strings.HasSuffix(n, "-"+flat) {
			return y
		}
	}
	return name
}

func locationOf(tool string) string {
	if tool == "" {
		return agent.LocationNone
	}
	if builtinTools[tool] {
		return agent.LocationSandbox
	}
	return agent.LocationYari
}

type observationText struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	IsError  bool   `json:"is_error"`
	Command  string `json:"command"`
	ExitCode *int   `json:"exit_code"`
}

type eventEnvelope struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`

	ToolName string `json:"tool_name"`
	Summary  string `json:"summary"`
	Action   struct {
		Command string `json:"command"`
		Path    string `json:"path"`
		Kind    string `json:"kind"`
	} `json:"action"`
	ToolCall struct {
		Arguments string `json:"arguments"`
	} `json:"tool_call"`
	Observation observationText `json:"observation"`
	LLMMessage  struct {
		Role    string `json:"role"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"llm_message"`

	Code   string `json:"code"`
	Detail string `json:"detail"`
	Error  string `json:"error"`
}

const (
	maxSummary = 300
	maxRawText = 8 << 10
)

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// Map converts a harness event to a yari event. The second result is false for events that carry
// nothing to show, such as per-token statistics.
//
// Raw is a compact JSON object. For observations it carries the tool text under "result", which is
// where the session job looks for plan ids; the full harness event is not stored because it can
// be very large.
func Map(ev client.StreamEvent) (agent.Event, bool) {
	var e eventEnvelope
	if err := json.Unmarshal(ev.Raw, &e); err != nil {
		return agent.Event{}, false
	}
	at := parseTime(e.Timestamp)
	out := agent.Event{ExternalID: e.ID, At: at, Location: agent.LocationNone}
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

	switch e.Kind {
	case client.KindMessage:
		var text strings.Builder
		for _, c := range e.LLMMessage.Content {
			text.WriteString(c.Text)
		}
		role := e.Source
		out.Kind = agent.EventMessage
		out.Summary = clip(role+": "+text.String(), maxSummary)
		out.Raw = raw(map[string]any{"role": role, "content": clip(text.String(), maxRawText)})
	case client.KindAction:
		tool := NormalizeTool(e.ToolName)
		out.Kind, out.Tool, out.Location = agent.EventAction, tool, locationOf(e.ToolName)
		switch {
		case e.Summary != "":
			out.Summary = clip(e.Summary, maxSummary)
		case e.Action.Command != "" && tool == "terminal":
			out.Summary = clip("run "+firstLine(e.Action.Command), maxSummary)
		case e.Action.Path != "":
			out.Summary = clip(fmt.Sprintf("%s %s", tool, e.Action.Path), maxSummary)
		default:
			out.Summary = "call " + tool
		}
		out.Raw = raw(map[string]any{"tool": tool, "arguments": clip(e.ToolCall.Arguments, maxRawText)})
	case client.KindObservation:
		tool := NormalizeTool(e.ToolName)
		var text strings.Builder
		for _, c := range e.Observation.Content {
			text.WriteString(c.Text)
		}
		out.Kind, out.Tool, out.Location = agent.EventObservation, tool, locationOf(e.ToolName)
		switch {
		case e.Observation.Command != "" && e.Observation.ExitCode != nil:
			out.Summary = clip(fmt.Sprintf("%s: exit %d", firstLine(e.Observation.Command), *e.Observation.ExitCode), maxSummary)
		case e.Observation.IsError:
			out.Summary = clip(tool+" refused: "+firstLine(text.String()), maxSummary)
		default:
			out.Summary = clip(tool+": "+firstLine(text.String()), maxSummary)
		}
		out.Raw = raw(map[string]any{"tool": tool, "is_error": e.Observation.IsError, "result": clip(text.String(), maxRawText)})
	case client.KindAgentError, client.KindConvError, client.KindServerError:
		detail := e.Detail
		if detail == "" {
			detail = e.Error
		}
		out.Kind = agent.EventError
		out.Summary = clip(strings.TrimSpace(e.Code+": "+detail), maxSummary)
		out.Raw = raw(map[string]any{"code": e.Code, "detail": clip(detail, maxRawText)})
	case client.KindPause, client.KindInterrupt:
		out.Kind = agent.EventStatus
		out.Summary = "agent paused"
		out.Raw = raw(map[string]any{"kind": e.Kind})
	case client.KindSystemPrompt:
		out.Kind = agent.EventStatus
		out.Summary = "agent initialized"
		out.Raw = raw(map[string]any{"kind": e.Kind})
	case client.KindStateUpdate:
		if st, ok := ev.ExecutionStatus(); ok {
			out.Kind = agent.EventStatus
			out.Summary = "execution status: " + string(st)
			out.Raw = raw(map[string]any{"execution_status": string(st)})
			return out, true
		}
		return agent.Event{}, false
	default:
		return agent.Event{}, false
	}
	return out, true
}

// parseTime reads the harness's naive ISO timestamps, which are UTC.
func parseTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02T15:04:05.999999", time.RFC3339Nano} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t
		}
	}
	return time.Now()
}

// systemPromptHasYariTools reports whether the agent was started with the yari MCP tools. An MCP
// server that cannot be reached is skipped silently by the harness, so the worker checks.
func systemPromptHasYariTools(raw []byte) bool {
	s := strings.ToLower(string(raw))
	for _, y := range yariTools {
		if strings.Contains(s, y) || strings.Contains(s, strings.ReplaceAll(y, ".", "_")) {
			return true
		}
	}
	return false
}
