package api

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/plan"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

func ts(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func idStr(id uuid.NullUUID) string {
	if !id.Valid {
		return ""
	}
	return id.UUID.String()
}

var scopeTypes = map[yariv1.ScopeType]authz.ScopeType{
	yariv1.ScopeType_SCOPE_TYPE_ORGANIZATION: authz.ScopeOrganization,
	yariv1.ScopeType_SCOPE_TYPE_WORKSPACE:    authz.ScopeWorkspace,
	yariv1.ScopeType_SCOPE_TYPE_PROJECT:      authz.ScopeProject,
	yariv1.ScopeType_SCOPE_TYPE_ENVIRONMENT:  authz.ScopeEnvironment,
}

func scopeFromProto(s *yariv1.Scope) (authz.Scope, error) {
	if s == nil {
		return authz.Scope{}, invalid("scope is required")
	}
	t, ok := scopeTypes[s.Type]
	if !ok {
		return authz.Scope{}, invalid("scope.type is required")
	}
	id, err := parseID("scope.id", s.Id)
	if err != nil {
		return authz.Scope{}, err
	}
	return authz.Scope{Type: t, ID: id}, nil
}

func scopeToProto(t string, id uuid.UUID) *yariv1.Scope {
	for k, v := range scopeTypes {
		if string(v) == t {
			return &yariv1.Scope{Type: k, Id: id.String()}
		}
	}
	return &yariv1.Scope{Id: id.String()}
}

var roles = map[yariv1.Role]authz.Role{
	yariv1.Role_ROLE_VIEWER:    authz.RoleViewer,
	yariv1.Role_ROLE_DEVELOPER: authz.RoleDeveloper,
	yariv1.Role_ROLE_ADMIN:     authz.RoleAdmin,
	yariv1.Role_ROLE_OWNER:     authz.RoleOwner,
}

func roleToProto(r string) yariv1.Role {
	for k, v := range roles {
		if string(v) == r {
			return k
		}
	}
	return yariv1.Role_ROLE_UNSPECIFIED
}

func decisionToProto(d authz.Decision) *yariv1.Decision {
	return &yariv1.Decision{Allowed: d.Allowed, RequiresApproval: d.RequiresApproval, Reason: d.Reason}
}

var principalKinds = map[string]yariv1.PrincipalKind{
	"human": yariv1.PrincipalKind_PRINCIPAL_KIND_HUMAN, "api_key": yariv1.PrincipalKind_PRINCIPAL_KIND_API_KEY,
	"ci": yariv1.PrincipalKind_PRINCIPAL_KIND_CI, "agent": yariv1.PrincipalKind_PRINCIPAL_KIND_AGENT,
}

func orgToProto(o dbsqlc.Organization) *yariv1.Organization {
	return &yariv1.Organization{Id: o.ID.String(), Slug: o.Slug, Name: o.Name, CreatedAt: ts(o.CreatedAt)}
}

func workspaceToProto(w dbsqlc.Workspace) *yariv1.Workspace {
	return &yariv1.Workspace{Id: w.ID.String(), OrganizationId: w.OrganizationID.String(), Slug: w.Slug, Name: w.Name, CreatedAt: ts(w.CreatedAt)}
}

func projectToProto(p dbsqlc.Project) *yariv1.Project {
	return &yariv1.Project{Id: p.ID.String(), OrganizationId: p.OrganizationID.String(), WorkspaceId: p.WorkspaceID.String(), Slug: p.Slug, Name: p.Name, CreatedAt: ts(p.CreatedAt)}
}

func envToProto(e dbsqlc.Environment) *yariv1.Environment {
	return &yariv1.Environment{
		Id: e.ID.String(), OrganizationId: e.OrganizationID.String(), ProjectId: e.ProjectID.String(), Slug: e.Slug, Name: e.Name,
		Protected: e.Protected, RequireSecondApprover: e.RequireSecondApprover, CreatedAt: ts(e.CreatedAt),
	}
}

func grantToProto(g dbsqlc.Grant) *yariv1.Grant {
	return &yariv1.Grant{
		Id: g.ID.String(), OrganizationId: g.OrganizationID.String(), PrincipalId: g.PrincipalID.String(),
		Role: roleToProto(g.Role), Scope: scopeToProto(g.ScopeType, g.ScopeID), CreatedBy: g.CreatedBy.String(), CreatedAt: ts(g.CreatedAt),
	}
}

var planStatuses = map[string]yariv1.PlanStatus{
	"draft": yariv1.PlanStatus_PLAN_STATUS_DRAFT, "pending_approval": yariv1.PlanStatus_PLAN_STATUS_PENDING_APPROVAL,
	"approved": yariv1.PlanStatus_PLAN_STATUS_APPROVED, "applying": yariv1.PlanStatus_PLAN_STATUS_APPLYING,
	"applied": yariv1.PlanStatus_PLAN_STATUS_APPLIED, "failed": yariv1.PlanStatus_PLAN_STATUS_FAILED,
	"cancelled": yariv1.PlanStatus_PLAN_STATUS_CANCELLED,
}

func planToProto(p dbsqlc.Plan) (*yariv1.Plan, error) {
	changes, err := plan.Unmarshal(p.Changes)
	if err != nil {
		return nil, err
	}
	plan.RedactForResponse(changes)
	return &yariv1.Plan{
		Id: p.ID.String(), OrganizationId: p.OrganizationID.String(), EnvironmentId: p.EnvironmentID.String(),
		CreatedBy: p.CreatedBy.String(), AgentSessionId: idStr(p.AgentSessionID), Status: planStatuses[p.Status],
		Changes: changes, CostDeltaCents: p.CostDeltaCents, Risk: plan.RiskToProto(riskOf(p.Risk)),
		ApprovedBy: idStr(p.ApprovedBy), CreatedAt: ts(p.CreatedAt), UpdatedAt: ts(p.UpdatedAt),
	}, nil
}

var sessionStatuses = map[string]yariv1.SessionStatus{
	"queued": yariv1.SessionStatus_SESSION_STATUS_QUEUED, "running": yariv1.SessionStatus_SESSION_STATUS_RUNNING,
	"waiting_approval": yariv1.SessionStatus_SESSION_STATUS_WAITING_APPROVAL, "succeeded": yariv1.SessionStatus_SESSION_STATUS_SUCCEEDED,
	"failed": yariv1.SessionStatus_SESSION_STATUS_FAILED, "stopped": yariv1.SessionStatus_SESSION_STATUS_STOPPED,
}

func sessionToProto(s dbsqlc.AgentSession) *yariv1.AgentSession {
	out := &yariv1.AgentSession{
		Id: s.ID.String(), OrganizationId: s.OrganizationID.String(), EnvironmentId: s.EnvironmentID.String(),
		AgentPrincipalId: s.AgentPrincipalID.String(), StartedBy: s.StartedBy.String(), Goal: s.Goal, Runtime: s.Runtime, Model: s.Model,
		Status: sessionStatuses[s.Status], BudgetUsdMicros: s.BudgetUsdMicros, SpendUsdMicros: s.SpendUsdMicros,
		MaxIterations: s.MaxIterations, StartedAt: tsPtr(s.StartedAt), EndedAt: tsPtr(s.EndedAt), CreatedAt: ts(s.CreatedAt),
	}
	if s.SandboxID.Valid {
		out.SandboxId = s.SandboxID.String
	}
	if s.ExternalConversationID.Valid {
		out.ExternalConversationId = s.ExternalConversationID.String
	}
	return out
}

var stepKinds = map[string]yariv1.StepKind{
	"message": yariv1.StepKind_STEP_KIND_MESSAGE, "action": yariv1.StepKind_STEP_KIND_ACTION,
	"observation": yariv1.StepKind_STEP_KIND_OBSERVATION, "error": yariv1.StepKind_STEP_KIND_ERROR,
	"guardrail": yariv1.StepKind_STEP_KIND_GUARDRAIL, "status": yariv1.StepKind_STEP_KIND_STATUS,
}

var stepLocations = map[string]yariv1.StepLocation{
	"sandbox": yariv1.StepLocation_STEP_LOCATION_SANDBOX, "yari": yariv1.StepLocation_STEP_LOCATION_YARI,
	"none": yariv1.StepLocation_STEP_LOCATION_NONE,
}

func stepToProto(s dbsqlc.AgentStep) *yariv1.AgentStep {
	return &yariv1.AgentStep{
		Id: s.ID, SessionId: s.SessionID.String(), Seq: s.Seq, Kind: stepKinds[s.Kind], Tool: s.Tool.String,
		Location: stepLocations[s.Location], Summary: s.Summary, RawEventJson: string(s.RawEvent),
		PlanId: idStr(s.PlanID), CreatedAt: ts(s.CreatedAt),
	}
}

func auditToProto(e dbsqlc.AuditEvent) (*yariv1.AuditEvent, error) {
	var m map[string]any
	if len(e.Metadata) > 0 {
		if err := json.Unmarshal(e.Metadata, &m); err != nil {
			return nil, err
		}
	}
	meta, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return &yariv1.AuditEvent{
		Id: e.ID, OrganizationId: e.OrganizationID.String(), OccurredAt: ts(e.OccurredAt), PrincipalId: e.PrincipalID.String(),
		Action: e.Action, Scope: scopeToProto(e.ScopeType, e.ScopeID), Target: e.Target, PlanId: idStr(e.PlanID),
		AgentSessionId: idStr(e.AgentSessionID), Metadata: meta,
	}, nil
}

func riskOf(s string) provider.Risk { return provider.Risk(s) }
