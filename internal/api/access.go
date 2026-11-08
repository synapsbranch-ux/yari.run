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
)

// scopeExists runs the scope-chain query; an empty chain means the scope is not in this tenant.
func (t *tx) scopeExists(ctx context.Context, s authz.Scope) (bool, error) {
	chain, err := authz.DBLoader{Q: t.Q}.ScopeChain(ctx, s)
	return len(chain) > 0, err
}

// CheckPermission answers whether the caller holds a permission on a scope.
func (s *Server) CheckPermission(ctx context.Context, req *connect.Request[yariv1.CheckPermissionRequest]) (*connect.Response[yariv1.CheckPermissionResponse], error) {
	perm := authz.Permission(req.Msg.Permission)
	if !authz.ValidPermission(perm) {
		return nil, invalid("unknown permission %q", req.Msg.Permission)
	}
	scope, err := scopeFromProto(req.Msg.Scope)
	if err != nil {
		return nil, err
	}
	var d authz.Decision
	err = s.findOrg(ctx, "scope", func(ctx context.Context, t *tx) error {
		ok, err := t.scopeExists(ctx, scope)
		if err != nil {
			return err
		}
		if !ok {
			return errNotHere
		}
		d, err = t.Az.Can(ctx, t.P, perm, scope)
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.CheckPermissionResponse{Decision: decisionToProto(d)}), nil
}

// GrantRole gives a principal a role on a scope. Needs member.manage on that scope, and the
// caller cannot grant a role above its own.
func (s *Server) GrantRole(ctx context.Context, req *connect.Request[yariv1.GrantRoleRequest]) (*connect.Response[yariv1.GrantRoleResponse], error) {
	scope, err := scopeFromProto(req.Msg.Scope)
	if err != nil {
		return nil, err
	}
	role, ok := roles[req.Msg.Role]
	if !ok {
		return nil, invalid("role is required")
	}
	target, err := parseID("principal_id", req.Msg.PrincipalId)
	if err != nil {
		return nil, err
	}
	var g dbsqlc.Grant
	err = s.findOrg(ctx, "scope", func(ctx context.Context, t *tx) error {
		ok, err := t.scopeExists(ctx, scope)
		if err != nil {
			return err
		}
		if !ok {
			return errNotHere
		}
		if _, err := t.check(ctx, authz.MemberManage, scope); err != nil {
			return err
		}
		mine, err := t.Az.RoleOn(ctx, t.P, scope)
		if err != nil {
			return err
		}
		if !mine.AtLeast(role) {
			return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you cannot grant %s: your highest role here is %s", role, mine))
		}
		if _, err := t.Q.GetPrincipal(ctx, target); errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("principal not found in this organization"))
		} else if err != nil {
			return err
		}
		if g, err = t.Q.CreateGrant(ctx, dbsqlc.CreateGrantParams{
			ID: db.NewID(), OrganizationID: t.Org, PrincipalID: target, Role: string(role),
			ScopeType: string(scope.Type), ScopeID: scope.ID, CreatedBy: t.P.ID,
		}); err != nil {
			return err
		}
		return t.audit(ctx, "grant.create", scope, g.ID.String(), map[string]any{"principal_id": target.String(), "role": string(role)}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.GrantRoleResponse{Grant: grantToProto(g)}), nil
}

// RevokeGrant removes a grant. Needs member.manage on the grant's scope.
func (s *Server) RevokeGrant(ctx context.Context, req *connect.Request[yariv1.RevokeGrantRequest]) (*connect.Response[yariv1.RevokeGrantResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	grantID, err := parseID("grant_id", req.Msg.GrantId)
	if err != nil {
		return nil, err
	}
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		g, err := t.Q.GetGrant(ctx, grantID)
		if err != nil {
			return err
		}
		scope := authz.Scope{Type: authz.ScopeType(g.ScopeType), ID: g.ScopeID}
		if _, err := t.check(ctx, authz.MemberManage, scope); err != nil {
			return err
		}
		if _, err := t.Q.DeleteGrant(ctx, grantID); err != nil {
			return err
		}
		return t.audit(ctx, "grant.revoke", scope, grantID.String(), map[string]any{"principal_id": g.PrincipalID.String(), "role": g.Role}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.RevokeGrantResponse{}), nil
}

// ListGrants lists the grants of an Organization. Needs member.manage.
func (s *Server) ListGrants(ctx context.Context, req *connect.Request[yariv1.ListGrantsRequest]) (*connect.Response[yariv1.ListGrantsResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	filter := uuid.NullUUID{}
	if req.Msg.PrincipalId != "" {
		id, err := parseID("principal_id", req.Msg.PrincipalId)
		if err != nil {
			return nil, err
		}
		filter = uuid.NullUUID{UUID: id, Valid: true}
	}
	resp := &yariv1.ListGrantsResponse{}
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		if _, err := t.check(ctx, authz.MemberManage, authz.Scope{Type: authz.ScopeOrganization, ID: org}); err != nil {
			return err
		}
		rows, err := t.Q.ListGrants(ctx, filter)
		if err != nil {
			return err
		}
		for _, g := range rows {
			resp.Grants = append(resp.Grants, grantToProto(g))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
