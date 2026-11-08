package api

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

// CreateOrganization creates an Organization. The human caller becomes its owner.
func (s *Server) CreateOrganization(ctx context.Context, req *connect.Request[yariv1.CreateOrganizationRequest]) (*connect.Response[yariv1.CreateOrganizationResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	if c.IsToken() {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only a signed-in user can create an organization"))
	}
	if err := validSlug("slug", req.Msg.Slug); err != nil {
		return nil, err
	}
	if err := validName("name", req.Msg.Name); err != nil {
		return nil, err
	}
	user, err := dbsqlc.New(s.Pool).GetUser(ctx, c.UserID)
	if err != nil {
		return nil, mapErr(err)
	}
	orgID := db.NewID()
	var org dbsqlc.Organization
	err = mapErr(db.WithTenant(ctx, s.Pool, orgID, func(ctx context.Context, pt pgx.Tx) error {
		q := dbsqlc.New(pt)
		var err error
		if org, err = q.CreateOrganization(ctx, dbsqlc.CreateOrganizationParams{ID: orgID, Slug: req.Msg.Slug, Name: req.Msg.Name}); err != nil {
			return err
		}
		name := user.DisplayName
		if name == "" {
			name = user.Email
		}
		p, err := q.CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{
			ID: db.NewID(), OrganizationID: orgID, Kind: "human", UserID: uuid.NullUUID{UUID: user.ID, Valid: true}, DisplayName: name,
		})
		if err != nil {
			return err
		}
		if _, err := q.CreateGrant(ctx, dbsqlc.CreateGrantParams{
			ID: db.NewID(), OrganizationID: orgID, PrincipalID: p.ID, Role: string(authz.RoleOwner),
			ScopeType: string(authz.ScopeOrganization), ScopeID: orgID, CreatedBy: p.ID,
		}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Event{
			OrganizationID: orgID, PrincipalID: p.ID, Action: "organization.create",
			ScopeType: string(authz.ScopeOrganization), ScopeID: orgID, Target: org.Slug,
			Metadata: map[string]any{"slug": org.Slug},
		})
	}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.CreateOrganizationResponse{Organization: orgToProto(org)}), nil
}

// ListOrganizations lists the Organizations the caller belongs to.
func (s *Server) ListOrganizations(ctx context.Context, _ *connect.Request[yariv1.ListOrganizationsRequest]) (*connect.Response[yariv1.ListOrganizationsResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.candidateOrgs(ctx, c)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &yariv1.ListOrganizationsResponse{}
	for _, id := range orgs {
		err := s.inOrg(ctx, id, func(ctx context.Context, t *tx) error {
			o, err := t.Q.GetOrganization(ctx, id)
			if err != nil {
				return err
			}
			resp.Organizations = append(resp.Organizations, orgToProto(o))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(resp), nil
}

// CreateWorkspace creates a Workspace. Needs organization.manage.
func (s *Server) CreateWorkspace(ctx context.Context, req *connect.Request[yariv1.CreateWorkspaceRequest]) (*connect.Response[yariv1.CreateWorkspaceResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	if err := validSlug("slug", req.Msg.Slug); err != nil {
		return nil, err
	}
	if err := validName("name", req.Msg.Name); err != nil {
		return nil, err
	}
	var ws dbsqlc.Workspace
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		scope := authz.Scope{Type: authz.ScopeOrganization, ID: org}
		if _, err := t.check(ctx, authz.OrganizationManage, scope); err != nil {
			return err
		}
		var err error
		if ws, err = t.Q.CreateWorkspace(ctx, dbsqlc.CreateWorkspaceParams{ID: db.NewID(), OrganizationID: org, Slug: req.Msg.Slug, Name: req.Msg.Name}); err != nil {
			return err
		}
		return t.audit(ctx, "workspace.create", scope, ws.ID.String(), map[string]any{"slug": ws.Slug}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.CreateWorkspaceResponse{Workspace: workspaceToProto(ws)}), nil
}

// ListWorkspaces lists the Workspaces the caller can read.
func (s *Server) ListWorkspaces(ctx context.Context, req *connect.Request[yariv1.ListWorkspacesRequest]) (*connect.Response[yariv1.ListWorkspacesResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	resp := &yariv1.ListWorkspacesResponse{}
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		all, err := t.Q.ListWorkspaces(ctx)
		if err != nil {
			return err
		}
		for _, w := range all {
			d, err := t.Az.Can(ctx, t.P, authz.WorkspaceRead, authz.Scope{Type: authz.ScopeWorkspace, ID: w.ID})
			if err != nil {
				return err
			}
			if d.Allowed {
				resp.Workspaces = append(resp.Workspaces, workspaceToProto(w))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// CreateProject creates a Project. Needs environment.manage on the Workspace.
func (s *Server) CreateProject(ctx context.Context, req *connect.Request[yariv1.CreateProjectRequest]) (*connect.Response[yariv1.CreateProjectResponse], error) {
	wsID, err := parseID("workspace_id", req.Msg.WorkspaceId)
	if err != nil {
		return nil, err
	}
	if err := validSlug("slug", req.Msg.Slug); err != nil {
		return nil, err
	}
	if err := validName("name", req.Msg.Name); err != nil {
		return nil, err
	}
	var p dbsqlc.Project
	err = s.findOrg(ctx, "workspace", func(ctx context.Context, t *tx) error {
		ws, err := t.Q.GetWorkspace(ctx, wsID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		}
		if err != nil {
			return err
		}
		scope := authz.Scope{Type: authz.ScopeWorkspace, ID: ws.ID}
		if _, err := t.check(ctx, authz.EnvironmentManage, scope); err != nil {
			return err
		}
		if p, err = t.Q.CreateProject(ctx, dbsqlc.CreateProjectParams{ID: db.NewID(), OrganizationID: t.Org, WorkspaceID: ws.ID, Slug: req.Msg.Slug, Name: req.Msg.Name}); err != nil {
			return err
		}
		return t.audit(ctx, "project.create", scope, p.ID.String(), map[string]any{"slug": p.Slug}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.CreateProjectResponse{Project: projectToProto(p)}), nil
}

// ListProjects lists the Projects of a Workspace that the caller can read.
func (s *Server) ListProjects(ctx context.Context, req *connect.Request[yariv1.ListProjectsRequest]) (*connect.Response[yariv1.ListProjectsResponse], error) {
	wsID, err := parseID("workspace_id", req.Msg.WorkspaceId)
	if err != nil {
		return nil, err
	}
	resp := &yariv1.ListProjectsResponse{}
	err = s.findOrg(ctx, "workspace", func(ctx context.Context, t *tx) error {
		if _, err := t.Q.GetWorkspace(ctx, wsID); errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		} else if err != nil {
			return err
		}
		all, err := t.Q.ListProjects(ctx, wsID)
		if err != nil {
			return err
		}
		for _, p := range all {
			d, err := t.Az.Can(ctx, t.P, authz.ProjectRead, authz.Scope{Type: authz.ScopeProject, ID: p.ID})
			if err != nil {
				return err
			}
			if d.Allowed {
				resp.Projects = append(resp.Projects, projectToProto(p))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// CreateEnvironment creates an Environment. The slug "production" is always protected.
func (s *Server) CreateEnvironment(ctx context.Context, req *connect.Request[yariv1.CreateEnvironmentRequest]) (*connect.Response[yariv1.CreateEnvironmentResponse], error) {
	projectID, err := parseID("project_id", req.Msg.ProjectId)
	if err != nil {
		return nil, err
	}
	if err := validSlug("slug", req.Msg.Slug); err != nil {
		return nil, err
	}
	if err := validName("name", req.Msg.Name); err != nil {
		return nil, err
	}
	var e dbsqlc.Environment
	err = s.findOrg(ctx, "project", func(ctx context.Context, t *tx) error {
		pr, err := t.Q.GetProject(ctx, projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		}
		if err != nil {
			return err
		}
		scope := authz.Scope{Type: authz.ScopeProject, ID: pr.ID}
		if _, err := t.check(ctx, authz.EnvironmentManage, scope); err != nil {
			return err
		}
		protected := req.Msg.Protected || strings.EqualFold(req.Msg.Slug, "production")
		if e, err = t.Q.CreateEnvironment(ctx, dbsqlc.CreateEnvironmentParams{
			ID: db.NewID(), OrganizationID: t.Org, ProjectID: pr.ID, Slug: req.Msg.Slug, Name: req.Msg.Name,
			Protected: protected, RequireSecondApprover: req.Msg.RequireSecondApprover,
		}); err != nil {
			return err
		}
		return t.audit(ctx, "environment.create", scope, e.ID.String(), map[string]any{"slug": e.Slug, "protected": e.Protected, "require_second_approver": e.RequireSecondApprover}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.CreateEnvironmentResponse{Environment: envToProto(e)}), nil
}

// ListEnvironments lists the Environments of a Project that the caller can read.
func (s *Server) ListEnvironments(ctx context.Context, req *connect.Request[yariv1.ListEnvironmentsRequest]) (*connect.Response[yariv1.ListEnvironmentsResponse], error) {
	projectID, err := parseID("project_id", req.Msg.ProjectId)
	if err != nil {
		return nil, err
	}
	resp := &yariv1.ListEnvironmentsResponse{}
	err = s.findOrg(ctx, "project", func(ctx context.Context, t *tx) error {
		if _, err := t.Q.GetProject(ctx, projectID); errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		} else if err != nil {
			return err
		}
		all, err := t.Q.ListEnvironments(ctx, projectID)
		if err != nil {
			return err
		}
		for _, e := range all {
			d, err := t.Az.Can(ctx, t.P, authz.EnvironmentRead, authz.Scope{Type: authz.ScopeEnvironment, ID: e.ID})
			if err != nil {
				return err
			}
			if d.Allowed {
				resp.Environments = append(resp.Environments, envToProto(e))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// GetEnvironment returns one Environment.
func (s *Server) GetEnvironment(ctx context.Context, req *connect.Request[yariv1.GetEnvironmentRequest]) (*connect.Response[yariv1.GetEnvironmentResponse], error) {
	envID, err := parseID("environment_id", req.Msg.EnvironmentId)
	if err != nil {
		return nil, err
	}
	var e dbsqlc.Environment
	err = s.findOrg(ctx, "environment", func(ctx context.Context, t *tx) error {
		var err error
		if e, err = t.Q.GetEnvironment(ctx, envID); errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		} else if err != nil {
			return err
		}
		_, err = t.check(ctx, authz.EnvironmentRead, authz.Scope{Type: authz.ScopeEnvironment, ID: envID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.GetEnvironmentResponse{Environment: envToProto(e)}), nil
}

// ListServices lists the Services of an Environment's Project with their state there.
func (s *Server) ListServices(ctx context.Context, req *connect.Request[yariv1.ListServicesRequest]) (*connect.Response[yariv1.ListServicesResponse], error) {
	envID, err := parseID("environment_id", req.Msg.EnvironmentId)
	if err != nil {
		return nil, err
	}
	resp := &yariv1.ListServicesResponse{}
	err = s.findOrg(ctx, "environment", func(ctx context.Context, t *tx) error {
		if _, err := t.Q.GetEnvironment(ctx, envID); errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		} else if err != nil {
			return err
		}
		if _, err := t.check(ctx, authz.ServiceRead, authz.Scope{Type: authz.ScopeEnvironment, ID: envID}); err != nil {
			return err
		}
		rows, err := t.Q.ListServicesForEnvironment(ctx, envID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			resp.Services = append(resp.Services, &yariv1.Service{
				Id: r.ID.String(), OrganizationId: r.OrganizationID.String(), ProjectId: r.ProjectID.String(),
				Name: r.Name, Kind: r.Kind, Provider: r.Provider, Status: r.Status,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
