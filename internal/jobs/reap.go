package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/jackc/pgx/v5"
	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/sandbox"
)

// ReapArgs is the scheduled cleanup of sandboxes and keys.
type ReapArgs struct{}

// Kind implements river.JobArgs.
func (ReapArgs) Kind() string { return "reap_sandboxes" }

// ReapWorker destroys sandboxes past their TTL and cleans up sessions that ended uncleanly
// (for example when a worker died): their model key and session token are revoked, their sandbox
// destroyed and the session marked failed.
type ReapWorker struct {
	river.WorkerDefaults[ReapArgs]
	Pool      *pgxpool.Pool
	Sandboxes sandbox.Manager
	Gateway   ModelGateway
	// MaxAge is how old a queued or running session may be before it counts as stuck.
	MaxAge time.Duration
}

// Work implements river.Worker.
func (w *ReapWorker) Work(ctx context.Context, _ *river.Job[ReapArgs]) error {
	if n, err := w.Sandboxes.Reap(ctx); err != nil {
		slog.Error("reap sandboxes", "error", err)
	} else if n > 0 {
		slog.Info("reaped sandboxes past their TTL", "count", n)
	}

	maxAge := w.MaxAge
	if maxAge <= 0 {
		maxAge = time.Hour
	}
	stuck, err := db.StuckAgentSessions(ctx, w.Pool, time.Now().Add(-maxAge))
	if err != nil {
		return err
	}
	for _, s := range stuck {
		slog.Warn("cleaning up a session that ended uncleanly", "session", s.SessionID)
		if s.GatewayAlias != "" {
			if err := w.Gateway.Revoke(ctx, s.GatewayAlias); err != nil {
				slog.Error("revoke model key", "session", s.SessionID, "error", err)
			}
		}
		if s.SandboxID != "" {
			if sb, err := w.Sandboxes.Get(ctx, s.SandboxID); err == nil {
				_ = sb.Destroy(ctx)
			}
		}
		err := db.WithTenant(ctx, w.Pool, s.OrganizationID, func(ctx context.Context, tx pgx.Tx) error {
			q := dbsqlc.New(tx)
			if _, err := q.RevokeAPITokensByLabel(ctx, sessionTokenLabel(s.SessionID)); err != nil {
				return err
			}
			sess, err := q.FinishAgentSession(ctx, dbsqlc.FinishAgentSessionParams{ID: s.SessionID, Status: "failed"})
			if err != nil {
				return err
			}
			return audit.Record(ctx, q, audit.Event{OrganizationID: s.OrganizationID, PrincipalID: sess.AgentPrincipalID, Action: "agent.session.ended",
				ScopeType: "environment", ScopeID: sess.EnvironmentID, Target: s.SessionID.String(), AgentSessionID: s.SessionID,
				Metadata: map[string]any{"status": "failed", "reason": "ended uncleanly, reaped"}})
		})
		if err != nil {
			slog.Error("finish stuck session", "session", s.SessionID, "error", err)
		}
	}
	return nil
}
