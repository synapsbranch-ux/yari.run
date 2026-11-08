package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
	"github.com/synapsbranch-ux/yari.run/internal/orchestrator"
	"github.com/synapsbranch-ux/yari.run/internal/plan"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

const maxChangesPerPlan = 200

// GuardrailPrefix starts the message of every refusal that comes from a guardrail, so the
// agent session can record it as a guardrail step instead of a plain error.
const GuardrailPrefix = "guardrail: "

// CreatePlan records a reviewable Plan in draft. Risk and cost are computed here, never trusted
// from the caller.
func (s *Server) CreatePlan(ctx context.Context, req *connect.Request[yariv1.CreatePlanRequest]) (*connect.Response[yariv1.CreatePlanResponse], error) {
	envID, err := parseID("environment_id", req.Msg.EnvironmentId)
	if err != nil {
		return nil, err
	}
	if n := len(req.Msg.Changes); n == 0 || n > maxChangesPerPlan {
		return nil, invalid("a plan needs between 1 and %d changes", maxChangesPerPlan)
	}
	var sessionID uuid.UUID
	if req.Msg.AgentSessionId != "" {
		if sessionID, err = parseID("agent_session_id", req.Msg.AgentSessionId); err != nil {
			return nil, err
		}
	}

	var out dbsqlc.Plan
	err = s.findOrg(ctx, "environment", func(ctx context.Context, t *tx) error {
		env, err := t.Q.GetEnvironment(ctx, envID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		}
		if err != nil {
			return err
		}
		scope := authz.Scope{Type: authz.ScopeEnvironment, ID: envID}
		if _, err := t.check(ctx, authz.PlanCreate, scope); err != nil {
			return err
		}

		changes, err := s.validateChanges(ctx, t, env, req.Msg.Changes)
		if err != nil {
			return err
		}
		isAgent := t.P.Kind == authz.KindAgent
		if isAgent {
			// Before provider checks: a refused delete must be refused whatever else is wrong.
			if err := s.enforceAgentDenyList(ctx, t, env, changes, sessionID); err != nil {
				return err
			}
		}
		if err := s.validateProviders(changes); err != nil {
			return err
		}

		var sessionRef uuid.NullUUID
		if isAgent && sessionID != uuid.Nil {
			sess, err := t.Q.GetAgentSession(ctx, sessionID)
			if err != nil || sess.AgentPrincipalID != t.P.ID || sess.EnvironmentID != envID {
				return invalid("agent_session_id does not belong to this agent and environment")
			}
			if sess.Status != "running" && sess.Status != "queued" {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New("the agent session is not active"))
			}
			sessionRef = uuid.NullUUID{UUID: sessionID, Valid: true}
		}

		pcs := make([]*yariv1.PlanChange, 0, len(changes))
		for _, c := range changes {
			pcs = append(pcs, plan.ToProto(c))
		}
		raw, err := plan.Marshal(pcs)
		if err != nil {
			return err
		}
		cost, risk := plan.Summarize(changes)
		if out, err = t.Q.CreatePlan(ctx, dbsqlc.CreatePlanParams{
			ID: db.NewID(), OrganizationID: t.Org, EnvironmentID: envID, CreatedBy: t.P.ID, AgentSessionID: sessionRef,
			Status: "draft", Changes: raw, CostDeltaCents: cost, Risk: string(risk),
		}); err != nil {
			return err
		}
		return t.audit(ctx, "plan.create", scope, out.ID.String(), map[string]any{"changes": len(changes), "risk": string(risk), "cost_delta_cents": cost}, out.ID, sessionRef.UUID)
	})
	if err != nil {
		return nil, err
	}
	p, err := planToProto(out)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&yariv1.CreatePlanResponse{Plan: p}), nil
}

// validateChanges checks the changes and fills in server-computed risk and cost.
func (s *Server) validateChanges(ctx context.Context, t *tx, env dbsqlc.Environment, in []*yariv1.PlanChange) ([]provider.Change, error) {
	out := make([]provider.Change, 0, len(in))
	seen := map[string]bool{}
	var specs []provider.ServiceSpec
	for i, c := range in {
		pc, err := plan.FromProto(c)
		if err != nil {
			return nil, invalid("change %d: %v", i, err)
		}
		if err := validName("service", pc.Service); err != nil {
			return nil, invalid("change %d: %v", i, err)
		}
		if seen[pc.Service] {
			return nil, invalid("change %d: service %q appears twice", i, pc.Service)
		}
		seen[pc.Service] = true

		kind := provider.Kind("")
		if pc.After != nil {
			if pc.After.Name == "" {
				pc.After.Name = pc.Service
			}
			if pc.After.Name != pc.Service {
				return nil, invalid("change %d: after.name must equal service", i)
			}
			if !pc.After.Kind.Valid() {
				return nil, invalid("change %d: unknown kind %q", i, pc.After.Kind)
			}
			kind = pc.After.Kind
			for name, v := range pc.After.Env {
				if v.Secret && v.FromOutput == nil && v.Value != "" {
					return nil, invalid("change %d: env %q is secret: pass it with from_output or a Connection, never as a literal", i, name)
				}
			}
		}
		switch pc.Action {
		case provider.ActionCreate, provider.ActionUpdate:
			if pc.After == nil {
				return nil, invalid("change %d: %s needs an after spec", i, pc.Action)
			}
			specs = append(specs, *pc.After)
		case provider.ActionDelete, provider.ActionNoop:
			if kind == "" {
				// Without a spec, take the kind from the stored service.
				if svc, err := t.Q.GetServiceByName(ctx, dbsqlc.GetServiceByNameParams{ProjectID: env.ProjectID, Name: pc.Service}); err == nil {
					kind = provider.Kind(svc.Kind)
				}
			}
		}
		pc.Risk, pc.CostDeltaCents = plan.Assess(pc.Action, kind)
		out = append(out, pc)
	}
	if _, err := orchestrator.Order(specs); err != nil {
		return nil, invalid("%v", err)
	}
	return out, nil
}

// validateProviders checks every spec against the Provider Catalog.
func (s *Server) validateProviders(changes []provider.Change) error {
	for i, c := range changes {
		if c.After == nil {
			continue
		}
		entry, ok := s.Catalog.Get(c.After.Provider)
		if !ok {
			return invalid("change %d: unknown provider %q (see ListProviders)", i, c.After.Provider)
		}
		if !entry.Supports(c.After.Kind) {
			return invalid("change %d: provider %q cannot run kind %q", i, entry.ID, c.After.Kind)
		}
	}
	return nil
}

// dataStoreDeleteRefusal explains why an agent may not delete this change's service, or "".
func (s *Server) dataStoreDeleteRefusal(ctx context.Context, t *tx, env dbsqlc.Environment, c provider.Change) string {
	if c.Action != provider.ActionDelete {
		return ""
	}
	kind := provider.Kind("")
	if c.After != nil {
		kind = c.After.Kind
	}
	if svc, err := t.Q.GetServiceByName(ctx, dbsqlc.GetServiceByNameParams{ProjectID: env.ProjectID, Name: c.Service}); err == nil {
		if provider.Kind(svc.Kind).IsDataStore() {
			kind = provider.Kind(svc.Kind) // the stored kind wins over what the caller claims
		} else if kind == "" {
			kind = provider.Kind(svc.Kind)
		}
	}
	switch {
	case kind == "":
		return fmt.Sprintf("cannot tell what kind of service %q is, so deleting it is refused for agents", c.Service)
	case kind.IsDataStore():
		return fmt.Sprintf("agents may not delete %s services (%q holds data)", kind, c.Service)
	}
	return ""
}

// enforceAgentDenyList rejects deletes of data stores proposed by an agent and records the refusal.
func (s *Server) enforceAgentDenyList(ctx context.Context, t *tx, env dbsqlc.Environment, changes []provider.Change, session uuid.UUID) error {
	for _, c := range changes {
		reason := s.dataStoreDeleteRefusal(ctx, t, env, c)
		if reason == "" {
			continue
		}
		// The refusal is recorded in its own transaction: the request transaction rolls back.
		scope := authz.Scope{Type: authz.ScopeEnvironment, ID: env.ID}
		_ = db.WithTenant(ctx, s.Pool, t.Org, func(ctx context.Context, pt pgx.Tx) error {
			return newTx(pt, t.Org, t.P).audit(ctx, "plan.refused", scope, c.Service,
				map[string]any{"reason": reason, "guardrail": "agent-delete-data-store"}, uuid.Nil, session)
		})
		return connect.NewError(connect.CodePermissionDenied, errors.New(GuardrailPrefix+reason))
	}
	return nil
}

func (s *Server) loadPlan(ctx context.Context, t *tx, id uuid.UUID, lock bool) (dbsqlc.Plan, authz.Scope, error) {
	var p dbsqlc.Plan
	var err error
	if lock {
		p, err = t.Q.GetPlanForUpdate(ctx, id)
	} else {
		p, err = t.Q.GetPlan(ctx, id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return p, authz.Scope{}, errNotHere
	}
	return p, authz.Scope{Type: authz.ScopeEnvironment, ID: p.EnvironmentID}, err
}

// GetPlan returns a Plan. Needs plan.read.
func (s *Server) GetPlan(ctx context.Context, req *connect.Request[yariv1.GetPlanRequest]) (*connect.Response[yariv1.GetPlanResponse], error) {
	id, err := parseID("plan_id", req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	var p dbsqlc.Plan
	err = s.findOrg(ctx, "plan", func(ctx context.Context, t *tx) error {
		var scope authz.Scope
		var err error
		if p, scope, err = s.loadPlan(ctx, t, id, false); err != nil {
			return err
		}
		_, err = t.check(ctx, authz.PlanRead, scope)
		return err
	})
	if err != nil {
		return nil, err
	}
	pp, err := planToProto(p)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&yariv1.GetPlanResponse{Plan: pp}), nil
}

// ListPendingApprovals lists plans waiting for the caller's approval across the caller's Organizations.
func (s *Server) ListPendingApprovals(ctx context.Context, _ *connect.Request[yariv1.ListPendingApprovalsRequest]) (*connect.Response[yariv1.ListPendingApprovalsResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.candidateOrgs(ctx, c)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &yariv1.ListPendingApprovalsResponse{}
	for _, org := range orgs {
		err := s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
			pending, err := t.Q.ListPendingPlans(ctx)
			if err != nil {
				return err
			}
			for _, p := range pending {
				d, err := t.Az.CanApprove(ctx, t.P, authz.Scope{Type: authz.ScopeEnvironment, ID: p.EnvironmentID}, p.CreatedBy)
				if err != nil {
					return err
				}
				if !d.Allowed {
					continue
				}
				pp, err := planToProto(p)
				if err != nil {
					return err
				}
				resp.Plans = append(resp.Plans, pp)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(resp), nil
}

// ApprovePlan approves a plan waiting for approval. The creator can never approve its own plan.
func (s *Server) ApprovePlan(ctx context.Context, req *connect.Request[yariv1.ApprovePlanRequest]) (*connect.Response[yariv1.ApprovePlanResponse], error) {
	id, err := parseID("plan_id", req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	var out dbsqlc.Plan
	err = s.findOrg(ctx, "plan", func(ctx context.Context, t *tx) error {
		p, scope, err := s.loadPlan(ctx, t, id, true)
		if err != nil {
			return err
		}
		d, err := t.Az.CanApprove(ctx, t.P, scope, p.CreatedBy)
		if err != nil {
			return err
		}
		if !d.Allowed {
			return denied(authz.PlanApprove, d)
		}
		if p.Status != "pending_approval" {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plan is %s, only pending_approval plans can be approved", p.Status))
		}
		if out, err = t.Q.SetPlanStatus(ctx, dbsqlc.SetPlanStatusParams{ID: id, Status: "approved", ApprovedBy: uuid.NullUUID{UUID: t.P.ID, Valid: true}}); err != nil {
			return err
		}
		return t.audit(ctx, "plan.approve", scope, id.String(), nil, id, p.AgentSessionID.UUID)
	})
	if err != nil {
		return nil, err
	}
	pp, err := planToProto(out)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&yariv1.ApprovePlanResponse{Plan: pp}), nil
}

// CancelPlan cancels a plan that has not started applying. Needs plan.create.
func (s *Server) CancelPlan(ctx context.Context, req *connect.Request[yariv1.CancelPlanRequest]) (*connect.Response[yariv1.CancelPlanResponse], error) {
	id, err := parseID("plan_id", req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	var out dbsqlc.Plan
	err = s.findOrg(ctx, "plan", func(ctx context.Context, t *tx) error {
		p, scope, err := s.loadPlan(ctx, t, id, true)
		if err != nil {
			return err
		}
		if _, err := t.check(ctx, authz.PlanCreate, scope); err != nil {
			return err
		}
		switch p.Status {
		case "draft", "pending_approval", "approved":
		default:
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plan is %s and can no longer be cancelled", p.Status))
		}
		if out, err = t.Q.SetPlanStatus(ctx, dbsqlc.SetPlanStatusParams{ID: id, Status: "cancelled"}); err != nil {
			return err
		}
		return t.audit(ctx, "plan.cancel", scope, id.String(), nil, id, p.AgentSessionID.UUID)
	})
	if err != nil {
		return nil, err
	}
	pp, err := planToProto(out)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&yariv1.CancelPlanResponse{Plan: pp}), nil
}

// ApplyPlan applies a plan, or parks it in pending_approval when the decision needs approval
// that has not been given. The job is inserted in the same transaction that marks it applying.
func (s *Server) ApplyPlan(ctx context.Context, req *connect.Request[yariv1.ApplyPlanRequest]) (*connect.Response[yariv1.ApplyPlanResponse], error) {
	id, err := parseID("plan_id", req.Msg.PlanId)
	if err != nil {
		return nil, err
	}
	var out dbsqlc.Plan
	var needsApproval bool
	err = s.findOrg(ctx, "plan", func(ctx context.Context, t *tx) error {
		p, scope, err := s.loadPlan(ctx, t, id, true)
		if err != nil {
			return err
		}
		d, err := t.check(ctx, authz.PlanApply, scope)
		if err != nil {
			return err
		}
		switch p.Status {
		case "draft", "pending_approval", "approved":
		default:
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plan is %s and cannot be applied", p.Status))
		}

		if d.RequiresApproval && p.Status != "approved" {
			needsApproval = true
			out = p
			if p.Status != "pending_approval" {
				if out, err = t.Q.SetPlanStatus(ctx, dbsqlc.SetPlanStatusParams{ID: id, Status: "pending_approval"}); err != nil {
					return err
				}
			}
			return t.audit(ctx, "plan.apply.requested", scope, id.String(), map[string]any{"reason": d.Reason}, id, p.AgentSessionID.UUID)
		}

		if out, err = t.Q.SetPlanStatus(ctx, dbsqlc.SetPlanStatusParams{ID: id, Status: "applying"}); err != nil {
			return err
		}
		if _, err := s.River.InsertTx(ctx, t.Tx, jobs.ApplyPlanArgs{OrganizationID: t.Org, PlanID: id}, nil); err != nil {
			return fmt.Errorf("enqueue apply: %w", err)
		}
		return t.audit(ctx, "plan.apply", scope, id.String(), nil, id, p.AgentSessionID.UUID)
	})
	if err != nil {
		return nil, err
	}
	pp, err := planToProto(out)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&yariv1.ApplyPlanResponse{Plan: pp, RequiresApproval: needsApproval}), nil
}
