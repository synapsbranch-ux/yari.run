package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/orchestrator"
	"github.com/synapsbranch-ux/yari.run/internal/plan"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

// ApplyPlanWorker applies a plan through the orchestrator and records the outcome.
type ApplyPlanWorker struct {
	river.WorkerDefaults[ApplyPlanArgs]
	Pool     *pgxpool.Pool
	Registry *provider.Registry
	Leaser   orchestrator.Leaser
}

// Work implements river.Worker. The plan row is the outcome: a failed apply sets the plan to
// failed and ends the job without a retry. Only infrastructure errors return an error.
func (w *ApplyPlanWorker) Work(ctx context.Context, job *river.Job[ApplyPlanArgs]) error {
	org, planID := job.Args.OrganizationID, job.Args.PlanID

	var p dbsqlc.Plan
	var env dbsqlc.Environment
	err := db.WithTenant(ctx, w.Pool, org, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		var err error
		if p, err = q.GetPlan(ctx, planID); err != nil {
			return err
		}
		env, err = q.GetEnvironment(ctx, p.EnvironmentID)
		return err
	})
	if err != nil {
		return fmt.Errorf("load plan: %w", err)
	}
	if p.Status != "applying" {
		slog.Warn("apply job found a plan that is not applying, skipping", "plan", planID, "status", p.Status)
		return nil
	}

	stored, err := plan.Unmarshal(p.Changes)
	var changes []provider.Change
	if err == nil {
		changes, err = plan.Providers(stored)
	}
	var res *orchestrator.Result
	if err == nil {
		changes = w.fillDeleteProviders(ctx, org, env, changes)
		res, err = orchestrator.Run(ctx, w.Registry, w.Leaser, changes)
	}

	status, msg := "applied", ""
	if err != nil {
		status, msg = "failed", err.Error() // ApplyError messages are already free of secrets
		slog.Error("plan apply failed", "plan", planID, "error", msg)
	}
	return db.WithTenant(ctx, w.Pool, org, func(ctx context.Context, tx pgx.Tx) error {
		q := dbsqlc.New(tx)
		if _, err := q.SetPlanStatus(ctx, dbsqlc.SetPlanStatusParams{ID: planID, Status: status}); err != nil {
			return err
		}
		if res != nil {
			if err := recordServices(ctx, q, org, env, changes, res); err != nil {
				return err
			}
		}
		meta := map[string]any{"status": status, "system": true}
		if msg != "" {
			meta["error"] = msg
		}
		return audit.Record(ctx, q, audit.Event{
			OrganizationID: org, PrincipalID: p.CreatedBy, Action: "plan." + status,
			ScopeType: "environment", ScopeID: p.EnvironmentID, Target: planID.String(),
			PlanID: planID, AgentSessionID: p.AgentSessionID.UUID, Metadata: meta,
		})
	})
}

// fillDeleteProviders gives delete changes without a spec the provider recorded for the service.
func (w *ApplyPlanWorker) fillDeleteProviders(ctx context.Context, org uuid.UUID, env dbsqlc.Environment, changes []provider.Change) []provider.Change {
	for i, c := range changes {
		if c.Action != provider.ActionDelete || (c.After != nil && c.After.Provider != "") {
			continue
		}
		_ = db.WithTenant(ctx, w.Pool, org, func(ctx context.Context, tx pgx.Tx) error {
			var prov string
			err := tx.QueryRow(ctx, `SELECT si.provider FROM service_instances si JOIN services s ON s.id = si.service_id
				WHERE s.project_id = $1 AND s.name = $2 AND si.environment_id = $3`, env.ProjectID, c.Service, env.ID).Scan(&prov)
			if err == nil {
				changes[i].After = &provider.ServiceSpec{Name: c.Service, Provider: prov}
			}
			return nil
		})
	}
	return changes
}

func recordServices(ctx context.Context, q *dbsqlc.Queries, org uuid.UUID, env dbsqlc.Environment, changes []provider.Change, res *orchestrator.Result) error {
	for _, c := range changes {
		switch c.Action {
		case provider.ActionCreate, provider.ActionUpdate:
			ref, ok := res.Refs[c.Service]
			if !ok || c.After == nil {
				continue
			}
			svc, err := q.UpsertService(ctx, dbsqlc.UpsertServiceParams{ID: db.NewID(), OrganizationID: org, ProjectID: env.ProjectID, Name: c.Service, Kind: string(c.After.Kind)})
			if err != nil {
				return err
			}
			if err := q.UpsertServiceInstance(ctx, dbsqlc.UpsertServiceInstanceParams{
				ID: db.NewID(), OrganizationID: org, ServiceID: svc.ID, EnvironmentID: env.ID, Provider: c.After.Provider,
				ExternalRef: pgtype.Text{String: ref.ExternalID, Valid: ref.ExternalID != ""}, Status: "online",
			}); err != nil {
				return err
			}
		case provider.ActionDelete:
			for _, d := range res.Deleted {
				if d == c.Service {
					if err := q.DeleteServiceInstanceByName(ctx, dbsqlc.DeleteServiceInstanceByNameParams{ProjectID: env.ProjectID, Name: c.Service, EnvironmentID: env.ID}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
