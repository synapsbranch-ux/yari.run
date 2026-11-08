package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SessionAPIKeyHeader is the HTTP header the agent server reads its session API key from
// (REST and, for non-browser WebSocket clients, the upgrade request).
const SessionAPIKeyHeader = "X-Session-API-Key"

// Default tool names registered by the agent server (GET /server_info "usable_tools").
// Note: the tool registry uses these snake_case names. The CamelCase class names in the
// OpenAPI examples ("TerminalTool", ...) are rejected with HTTP 500 "not registered".
const (
	ToolTerminal    = "terminal"
	ToolFileEditor  = "file_editor"
	ToolTaskTracker = "task_tracker"
)

// NewAuthedClient returns a generated client that sends the session API key on every
// request. baseURL is the agent server root, for example "http://10.0.0.5:8000".
func NewAuthedClient(baseURL, sessionAPIKey string, httpClient *http.Client) (*ClientWithResponses, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return NewClientWithResponses(strings.TrimRight(baseURL, "/"),
		WithHTTPClient(httpClient),
		WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			if sessionAPIKey != "" {
				req.Header.Set(SessionAPIKeyHeader, sessionAPIKey)
			}
			return nil
		}),
	)
}

// ConversationConfig is everything yari sets when it creates an agent conversation.
// The LLM key and the MCP bearer token are session scoped and are sent through the
// agent server API only (never as container environment).
type ConversationConfig struct {
	// LLM: model is the LiteLLM model string, for example "openai/yari-default" when the
	// base URL is an OpenAI-compatible LiteLLM proxy.
	LLMModel   string
	LLMBaseURL string
	LLMAPIKey  string // LiteLLM virtual key for this session

	// MCP: optional yari MCP server (streamable HTTP) with a bearer session token.
	MCPURL   string
	MCPToken string

	WorkingDir    string // agent working directory, for example "/workspace/project"
	MaxIterations int    // hard cap on agent iterations (server default is 500)
	NumRetries    int    // LLM retries inside the agent server (server default is 5)

	// ConversationID optionally fixes the id (a UUID). Empty lets the server pick one.
	ConversationID string
}

// StartRequest builds the request body for POST /api/conversations.
//
// It deliberately has no initial message: with an initial message the server starts the
// agent loop (and so calls the model) as part of creation, even with run=false. Send
// the first message with SendMessage (run=false) and start the loop with
// RunConversation when the caller is ready.
func (c ConversationConfig) StartRequest() StartConversationRequest {
	kind := AgentKindAgent
	tools := []Tool{{Name: ToolTerminal}, {Name: ToolFileEditor}, {Name: ToolTaskTracker}}
	llm := LLM{
		Model:   ptr(c.LLMModel),
		BaseUrl: ptr(c.LLMBaseURL),
		ApiKey:  ptr(c.LLMAPIKey),
		UsageId: ptr("yari-session"),
	}
	if c.NumRetries > 0 {
		llm.NumRetries = ptr(c.NumRetries)
	}
	agent := Agent{Kind: &kind, Llm: llm, Tools: &tools}
	if c.MCPURL != "" {
		transport := MCPServerTransport("streamable-http")
		servers := map[string]MCPServer{"yari": {
			Url:       ptr(c.MCPURL),
			Transport: &transport,
			Headers:   &map[string]string{"Authorization": "Bearer " + c.MCPToken},
		}}
		agent.McpConfig = &servers
	}
	req := StartConversationRequest{
		Agent:          &agent,
		Workspace:      LocalWorkspace{WorkingDir: c.WorkingDir},
		Autotitle:      ptr(false), // no extra title-generation model call
		StuckDetection: ptr(true),
	}
	if c.MaxIterations > 0 {
		req.MaxIterations = ptr(c.MaxIterations)
	}
	if c.ConversationID != "" {
		req.ConversationId = ptr(c.ConversationID)
	}
	return req
}

// TextMessage builds a user message request. run=true makes the server start the agent
// loop if it is not already running.
func TextMessage(text string, run bool) SendMessageRequest {
	role := SendMessageRequestRoleUser
	typ := TextContentType("text")
	return SendMessageRequest{
		Role:    &role,
		Run:     &run,
		Content: &[]TextContent{{Text: text, Type: &typ}},
	}
}

// Settled reports whether a conversation execution status means the agent loop is no
// longer running and will not continue without a new message or a run call.
func (s ConversationExecutionStatus) Settled() bool {
	switch s {
	case "finished", "error", "stuck", "idle", "paused":
		return true
	}
	return false
}

// CheckStatus returns an error unless the HTTP status is one of the accepted codes.
func CheckStatus(op string, resp *http.Response, accepted ...int) error {
	for _, code := range accepted {
		if resp.StatusCode == code {
			return nil
		}
	}
	return fmt.Errorf("openhands %s: unexpected HTTP status %d", op, resp.StatusCode)
}

func ptr[T any](v T) *T { return &v }
