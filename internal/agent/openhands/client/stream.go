package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// Event kinds seen on the stream (the "kind" field of every event). The list is not
// exhaustive; unknown kinds must be tolerated.
const (
	KindSystemPrompt = "SystemPromptEvent"
	KindMessage      = "MessageEvent"           // source "user" or "agent"
	KindAction       = "ActionEvent"            // agent decided to call a tool
	KindObservation  = "ObservationEvent"       // tool result
	KindAgentError   = "AgentErrorEvent"        // tool call failed / invalid
	KindConvError    = "ConversationErrorEvent" // run aborted (for example LLM unreachable)
	KindStateUpdate  = "ConversationStateUpdateEvent"
	KindPause        = "PauseEvent"
	KindInterrupt    = "InterruptEvent"
	KindServerError  = "ServerErrorEvent" // sent by the socket handler itself on a bad frame
)

// StreamEvent is one event received over the WebSocket. Raw holds the complete JSON.
type StreamEvent struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Source    string          `json:"source"`
	Timestamp string          `json:"timestamp"`
	Raw       json.RawMessage `json:"-"`
}

// StateUpdate returns (key, value, true) for a ConversationStateUpdateEvent. For
// key "execution_status" value is one of idle, running, paused, waiting_for_confirmation,
// finished, error, stuck, deleting.
func (e StreamEvent) StateUpdate() (key string, value json.RawMessage, ok bool) {
	if e.Kind != KindStateUpdate {
		return "", nil, false
	}
	var v struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(e.Raw, &v); err != nil {
		return "", nil, false
	}
	return v.Key, v.Value, true
}

// ExecutionStatus returns the new execution status when the event is an
// execution_status state update.
func (e StreamEvent) ExecutionStatus() (ConversationExecutionStatus, bool) {
	key, val, ok := e.StateUpdate()
	if !ok || key != "execution_status" {
		return "", false
	}
	var s string
	if json.Unmarshal(val, &s) != nil {
		return "", false
	}
	return ConversationExecutionStatus(s), true
}

// StreamOptions configures Dial.
type StreamOptions struct {
	// ResendAll replays every stored event right after connecting (resend_mode=all), so a
	// reconnecting consumer does not miss events. Use ResendSince for a cheaper catch-up.
	ResendAll bool
	// ResendSince is an ISO 8601 timestamp; events with timestamp >= it are replayed
	// (resend_mode=since). Ignored when ResendAll is set.
	ResendSince string
}

// EventStream is a live WebSocket subscription to one conversation.
type EventStream struct {
	conn *websocket.Conn
}

// Dial opens ws(s)://host/sockets/events/{conversationID} and authenticates with the
// recommended first-message scheme: the first frame is {"type":"auth","session_api_key":...},
// which keeps the key out of URLs and access logs. The server accepts the upgrade
// first and only then validates the key; a bad key makes the first Next call fail with
// close code 4001.
func Dial(ctx context.Context, baseURL, sessionAPIKey, conversationID string, opts StreamOptions) (*EventStream, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = "/sockets/events/" + url.PathEscape(conversationID)
	q := url.Values{}
	switch {
	case opts.ResendAll:
		q.Set("resend_mode", "all")
	case opts.ResendSince != "":
		q.Set("resend_mode", "since")
		q.Set("after_timestamp", opts.ResendSince)
	}
	u.RawQuery = q.Encode()

	conn, _, err := websocket.Dial(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial event socket: %w", err)
	}
	// Events can be large (the system prompt event carries tool schemas).
	conn.SetReadLimit(32 << 20)

	auth, _ := json.Marshal(map[string]string{"type": "auth", "session_api_key": sessionAPIKey})
	if err := conn.Write(ctx, websocket.MessageText, auth); err != nil {
		_ = conn.CloseNow()
		return nil, fmt.Errorf("send auth frame: %w", err)
	}
	return &EventStream{conn: conn}, nil
}

// Next blocks for the next event. It returns an error when the context ends or the
// server closes the socket (close code 4001 auth failed, 4004 unknown conversation,
// 1013 too many subscribers).
func (s *EventStream) Next(ctx context.Context) (StreamEvent, error) {
	_, data, err := s.conn.Read(ctx)
	if err != nil {
		return StreamEvent{}, err
	}
	var ev StreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return StreamEvent{}, fmt.Errorf("decode event: %w", err)
	}
	ev.Raw = json.RawMessage(data)
	return ev, nil
}

// SendMessage sends a user message over the socket. The server treats it like
// POST /events with run=true. Prefer the REST SendMessage when you need run=false.
func (s *EventStream) SendMessage(ctx context.Context, text string) error {
	msg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []map[string]string{{"type": "text", "text": text}},
	})
	return s.conn.Write(ctx, websocket.MessageText, msg)
}

// Close closes the socket politely.
func (s *EventStream) Close() error {
	return s.conn.Close(websocket.StatusNormalClosure, "")
}
