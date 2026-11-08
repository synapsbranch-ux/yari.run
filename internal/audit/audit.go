// Package audit records audit events and the tamper-evident agent step log.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

// Event is one audit entry. Every mutation writes one, in the same transaction as the change.
type Event struct {
	OrganizationID uuid.UUID
	PrincipalID    uuid.UUID
	Action         string // e.g. "plan.apply", "organization.create", "agent.session.started"
	ScopeType      string
	ScopeID        uuid.UUID
	Target         string
	PlanID         uuid.UUID // zero if none
	AgentSessionID uuid.UUID // zero if none
	Metadata       map[string]any
}

// Record writes the event with the given queries, which must be bound to the caller's transaction.
func Record(ctx context.Context, q *dbsqlc.Queries, e Event) error {
	meta := e.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("audit: marshal metadata: %w", err)
	}
	_, err = q.InsertAuditEvent(ctx, dbsqlc.InsertAuditEventParams{
		OrganizationID: e.OrganizationID,
		PrincipalID:    e.PrincipalID,
		Action:         e.Action,
		ScopeType:      e.ScopeType,
		ScopeID:        e.ScopeID,
		Target:         e.Target,
		PlanID:         nullUUID(e.PlanID),
		AgentSessionID: nullUUID(e.AgentSessionID),
		Metadata:       raw,
	})
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

func nullUUID(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}
