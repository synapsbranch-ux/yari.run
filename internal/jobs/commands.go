package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CommandChannel is the PostgreSQL NOTIFY channel the API uses to reach a running Agent Session.
// NOTIFY is transactional: the command is delivered when the API transaction commits.
const CommandChannel = "yari_agent_commands"

// Command types.
const (
	CommandSend = "send"
	CommandStop = "stop"
)

// Command is a control message for a running Agent Session.
type Command struct {
	SessionID uuid.UUID `json:"session_id"`
	Type      string    `json:"type"`
	Message   string    `json:"message,omitempty"`
}

// MaxMessageBytes keeps a command under the NOTIFY payload limit.
const MaxMessageBytes = 4000

// Notify sends a command inside the caller's transaction.
func Notify(ctx context.Context, tx pgx.Tx, c Command) error {
	if len(c.Message) > MaxMessageBytes {
		return fmt.Errorf("jobs: message exceeds %d bytes", MaxMessageBytes)
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pg_notify($1, $2)`, CommandChannel, string(payload))
	return err
}
