package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

// StepInput is one entry to append to an Agent Session's step log.
type StepInput struct {
	Kind            string // message, action, observation, error, guardrail, status
	Tool            string
	Location        string // sandbox, yari, none
	Summary         string
	RawEvent        json.RawMessage // harness event, already redacted
	ExternalEventID string
	PlanID          uuid.UUID // zero if none
}

// Canonical re-encodes JSON with sorted keys and no insignificant whitespace. PostgreSQL jsonb does
// not preserve key order or spacing, so the hash is computed over this canonical form: reading the
// row back and canonicalizing it again yields the same bytes.
func Canonical(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte("{}"), nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("audit: raw event is not JSON: %w", err)
	}
	return json.Marshal(v)
}

// ComputeHash returns sha256(prev_hash || seq || kind || tool || summary || raw_event).
//
// Each variable-length field is prefixed with its 4-byte big-endian length and seq is encoded as
// 4 bytes big-endian, so two different entries can never produce the same byte stream.
// raw must already be canonical (see Canonical).
func ComputeHash(prev []byte, seq int32, kind, tool, summary string, raw []byte) []byte {
	h := sha256.New()
	writeField(h, prev)
	var s [4]byte
	binary.BigEndian.PutUint32(s[:], uint32(seq))
	h.Write(s[:])
	writeField(h, []byte(kind))
	writeField(h, []byte(tool))
	writeField(h, []byte(summary))
	writeField(h, raw)
	return h.Sum(nil)
}

type hashWriter interface{ Write([]byte) (int, error) }

func writeField(w hashWriter, b []byte) {
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(b)))
	_, _ = w.Write(l[:])
	_, _ = w.Write(b)
}

// AppendStep appends a step to the session log, chaining it to the previous step. It must run
// inside the session's tenant transaction. The caller is the only writer of a session's log.
func AppendStep(ctx context.Context, q *dbsqlc.Queries, orgID, sessionID uuid.UUID, in StepInput) (dbsqlc.AgentStep, error) {
	var prevHash []byte
	var seq int32 = 1
	last, err := q.LastAgentStep(ctx, sessionID)
	switch {
	case err == nil:
		prevHash, seq = last.Hash, last.Seq+1
	case errors.Is(err, pgx.ErrNoRows):
		prevHash = []byte{}
	default:
		return dbsqlc.AgentStep{}, fmt.Errorf("audit: last step: %w", err)
	}

	raw, err := Canonical(in.RawEvent)
	if err != nil {
		return dbsqlc.AgentStep{}, err
	}
	hash := ComputeHash(prevHash, seq, in.Kind, in.Tool, in.Summary, raw)
	step, err := q.AppendAgentStep(ctx, dbsqlc.AppendAgentStepParams{
		OrganizationID:  orgID,
		SessionID:       sessionID,
		Seq:             seq,
		Kind:            in.Kind,
		Tool:            pgtype.Text{String: in.Tool, Valid: in.Tool != ""},
		Location:        in.Location,
		Summary:         in.Summary,
		RawEvent:        raw,
		ExternalEventID: pgtype.Text{String: in.ExternalEventID, Valid: in.ExternalEventID != ""},
		PlanID:          nullUUID(in.PlanID),
		PrevHash:        prevHash,
		Hash:            hash,
	})
	if err != nil {
		return dbsqlc.AgentStep{}, fmt.Errorf("audit: append step: %w", err)
	}
	return step, nil
}

// ErrTampered is returned by VerifyChain when the log does not match its hashes.
var ErrTampered = errors.New("audit: step log is tampered or incomplete")

// VerifyChain checks a session's steps, ordered by seq starting at 1. It detects modified fields,
// removed steps, reordered steps and a replaced prefix.
func VerifyChain(steps []dbsqlc.AgentStep) error {
	prev := []byte{}
	for i, s := range steps {
		if s.Seq != int32(i+1) {
			return fmt.Errorf("%w: expected seq %d, found %d", ErrTampered, i+1, s.Seq)
		}
		if !bytes.Equal(s.PrevHash, prev) {
			return fmt.Errorf("%w: step %d does not chain to step %d", ErrTampered, s.Seq, s.Seq-1)
		}
		raw, err := Canonical(s.RawEvent)
		if err != nil {
			return fmt.Errorf("%w: step %d: %v", ErrTampered, s.Seq, err)
		}
		want := ComputeHash(prev, s.Seq, s.Kind, s.Tool.String, s.Summary, raw)
		if !bytes.Equal(want, s.Hash) {
			return fmt.Errorf("%w: step %d hash mismatch", ErrTampered, s.Seq)
		}
		prev = s.Hash
	}
	return nil
}
