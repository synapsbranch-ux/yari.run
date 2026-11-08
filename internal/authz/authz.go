// Package authz decides whether a Principal may perform a Permission on a Scope.
//
// Built-in Roles live here, in Go. Grants carry down the hierarchy
// (Organization > Workspace > Project > Environment) and the highest Role on the chain wins.
// Environment rules (protected, require_second_approver) and Agent rules are applied on top.
package authz

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Permission is a string constant naming one action.
type Permission string

const (
	OrganizationRead Permission = "organization.read"
	WorkspaceRead    Permission = "workspace.read"
	ProjectRead      Permission = "project.read"
	EnvironmentRead  Permission = "environment.read"
	ServiceRead      Permission = "service.read"
	PlanRead         Permission = "plan.read"
	LogsRead         Permission = "logs.read"
	AgentRead        Permission = "agent.read"

	ServiceWrite  Permission = "service.write"
	PlanCreate    Permission = "plan.create"
	ConnectionUse Permission = "connection.use"
	AgentRun      Permission = "agent.run"

	PlanApply  Permission = "plan.apply"
	SecretRead Permission = "secret.read"

	PlanApprove       Permission = "plan.approve"
	ConnectionManage  Permission = "connection.manage"
	MemberManage      Permission = "member.manage"
	EnvironmentManage Permission = "environment.manage"
	AuditRead         Permission = "audit.read"
	AgentManage       Permission = "agent.manage"

	OrganizationManage Permission = "organization.manage"
)

// Role is a built-in role.
type Role string

const (
	RoleViewer    Role = "viewer"
	RoleDeveloper Role = "developer"
	RoleAdmin     Role = "admin"
	RoleOwner     Role = "owner"
)

// rank orders roles. A higher rank includes everything a lower rank can do.
func (r Role) rank() int {
	switch r {
	case RoleViewer:
		return 1
	case RoleDeveloper:
		return 2
	case RoleAdmin:
		return 3
	case RoleOwner:
		return 4
	}
	return 0
}

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r.rank() > 0 }

var viewerPerms = []Permission{
	OrganizationRead, WorkspaceRead, ProjectRead, EnvironmentRead, ServiceRead, PlanRead, LogsRead, AgentRead,
}
var developerPerms = []Permission{
	ServiceWrite, PlanCreate, ConnectionUse, AgentRun,
	PlanApply,  // see the environment and agent rules in Evaluate
	SecretRead, // not on protected environments, see Evaluate
}
var adminPerms = []Permission{
	PlanApprove, ConnectionManage, MemberManage, EnvironmentManage, AuditRead, AgentManage,
}
var ownerPerms = []Permission{OrganizationManage}

// rolePerms lists the permissions each role holds, cumulative.
var rolePerms = func() map[Role]map[Permission]bool {
	out := map[Role]map[Permission]bool{}
	add := func(r Role, lists ...[]Permission) {
		m := map[Permission]bool{}
		for _, l := range lists {
			for _, p := range l {
				m[p] = true
			}
		}
		out[r] = m
	}
	add(RoleViewer, viewerPerms)
	add(RoleDeveloper, viewerPerms, developerPerms)
	add(RoleAdmin, viewerPerms, developerPerms, adminPerms)
	add(RoleOwner, viewerPerms, developerPerms, adminPerms, ownerPerms)
	return out
}()

// RoleHas reports whether the role includes the permission, before environment and agent rules.
func RoleHas(r Role, p Permission) bool { return rolePerms[r][p] }

// PrincipalKind mirrors principals.kind.
type PrincipalKind string

const (
	KindHuman  PrincipalKind = "human"
	KindAPIKey PrincipalKind = "api_key"
	KindCI     PrincipalKind = "ci"
	KindAgent  PrincipalKind = "agent"
)

// Principal is the authenticated actor of a request.
type Principal struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Kind           PrincipalKind
}

// ScopeType is a level of the hierarchy.
type ScopeType string

const (
	ScopeOrganization ScopeType = "organization"
	ScopeWorkspace    ScopeType = "workspace"
	ScopeProject      ScopeType = "project"
	ScopeEnvironment  ScopeType = "environment"
)

// Valid reports whether t is a known scope type.
func (t ScopeType) Valid() bool {
	switch t {
	case ScopeOrganization, ScopeWorkspace, ScopeProject, ScopeEnvironment:
		return true
	}
	return false
}

// Scope identifies one node of the hierarchy.
type Scope struct {
	Type ScopeType
	ID   uuid.UUID
}

// Grant gives a principal a role on a scope.
type Grant struct {
	Role  Role
	Scope Scope
}

// EnvInfo carries the environment flags that change decisions.
type EnvInfo struct {
	Protected             bool
	RequireSecondApprover bool
}

// Decision is the outcome of an authorization check.
type Decision struct {
	Allowed bool
	// RequiresApproval means the action is allowed only after another principal approves it.
	RequiresApproval bool
	Reason           string
}

func deny(format string, args ...any) Decision {
	return Decision{Reason: fmt.Sprintf(format, args...)}
}

// Evaluate applies the role matrix, environment rules and agent rules. It is pure.
// env is nil when the scope chain does not include an environment.
func Evaluate(kind PrincipalKind, role Role, perm Permission, env *EnvInfo) Decision {
	if !RoleHas(role, perm) {
		return deny("role %s does not include %s", role, perm)
	}
	protected := env != nil && env.Protected

	switch perm {
	case SecretRead:
		// Rule 4: an agent never reads secrets, whatever its role.
		if kind == KindAgent {
			return deny("agents can never read secrets")
		}
		// Rule 1: on protected environments only admin and owner may read secrets.
		if protected && role.rank() < RoleAdmin.rank() {
			return deny("secret.read on a protected environment needs the admin or owner role")
		}
	case PlanApprove:
		// Decision beyond the specified rules: an agent proposes, humans approve.
		if kind == KindAgent {
			return deny("agents cannot approve plans")
		}
	case PlanApply:
		switch {
		case kind == KindAgent && protected:
			// Rule 4: an agent's apply on a protected environment always needs approval.
			return Decision{Allowed: true, RequiresApproval: true, Reason: "agent changes to a protected environment need approval"}
		case role == RoleDeveloper && protected:
			// Rule 2.
			return Decision{Allowed: true, RequiresApproval: true, Reason: "developers need approval to apply to a protected environment"}
		case env != nil && env.RequireSecondApprover:
			// Rule 3: a second principal with plan.approve must approve, even for admin and owner.
			return Decision{Allowed: true, RequiresApproval: true, Reason: "this environment requires a second approver"}
		}
	}
	return Decision{Allowed: true, Reason: fmt.Sprintf("allowed by role %s", role)}
}

// Loader reads what Can needs. The database implementation runs inside the request's tenant
// transaction, so row-level security applies to every lookup.
type Loader interface {
	// ScopeChain returns the scope and all its ancestors. Empty if the scope does not exist
	// in the current tenant.
	ScopeChain(ctx context.Context, s Scope) ([]Scope, error)
	// Grants returns the principal's grants on any scope of the chain.
	Grants(ctx context.Context, principalID uuid.UUID, chain []Scope) ([]Grant, error)
	// Environment returns the flags of an environment.
	Environment(ctx context.Context, id uuid.UUID) (EnvInfo, error)
}

// Authorizer answers permission checks.
type Authorizer struct{ loader Loader }

// New returns an Authorizer reading through loader.
func New(loader Loader) *Authorizer { return &Authorizer{loader: loader} }

// Can resolves the scope's ancestor chain, collects every grant the principal holds on any
// ancestor (grants carry down), and applies the environment and agent rules.
func (a *Authorizer) Can(ctx context.Context, p Principal, perm Permission, scope Scope) (Decision, error) {
	if !scope.Type.Valid() || scope.ID == uuid.Nil {
		return deny("invalid scope"), nil
	}
	chain, err := a.loader.ScopeChain(ctx, scope)
	if err != nil {
		return Decision{}, fmt.Errorf("authz: scope chain: %w", err)
	}
	if len(chain) == 0 {
		return deny("scope not found"), nil
	}
	grants, err := a.loader.Grants(ctx, p.ID, chain)
	if err != nil {
		return Decision{}, fmt.Errorf("authz: grants: %w", err)
	}
	var best Role
	for _, g := range grants {
		if g.Role.rank() > best.rank() {
			best = g.Role
		}
	}
	if best == "" {
		return deny("no grant for this principal on %s or its ancestors", scope.Type), nil
	}

	var env *EnvInfo
	for _, s := range chain {
		if s.Type == ScopeEnvironment {
			info, err := a.loader.Environment(ctx, s.ID)
			if err != nil {
				return Decision{}, fmt.Errorf("authz: environment: %w", err)
			}
			env = &info
			break
		}
	}
	return Evaluate(p.Kind, best, perm, env), nil
}

// CanApprove checks plan.approve on the scope and rule 5: a principal can never approve a plan
// it created.
func (a *Authorizer) CanApprove(ctx context.Context, p Principal, scope Scope, planCreator uuid.UUID) (Decision, error) {
	d, err := a.Can(ctx, p, PlanApprove, scope)
	if err != nil || !d.Allowed {
		return d, err
	}
	if p.ID == planCreator {
		return deny("a principal cannot approve a plan it created"), nil
	}
	return d, nil
}
