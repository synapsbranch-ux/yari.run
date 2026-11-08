package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

const (
	minTokenTTL = time.Minute
	maxTokenTTL = 365 * 24 * time.Hour
)

// WhoAmI returns the caller and the Organizations it belongs to.
func (s *Server) WhoAmI(ctx context.Context, req *connect.Request[yariv1.WhoAmIRequest]) (*connect.Response[yariv1.WhoAmIResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	resp := &yariv1.WhoAmIResponse{}
	if !c.IsToken() {
		u, err := dbsqlc.New(s.Pool).GetUser(ctx, c.UserID)
		if err != nil {
			return nil, mapErr(err)
		}
		resp.UserId, resp.Email, resp.DisplayName = u.ID.String(), u.Email, u.DisplayName
	}
	orgs, err := s.candidateOrgs(ctx, c)
	if err != nil {
		return nil, mapErr(err)
	}
	for _, org := range orgs {
		err := s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
			o, err := t.Q.GetOrganization(ctx, org)
			if err != nil {
				return err
			}
			p, err := t.Q.GetPrincipal(ctx, t.P.ID)
			if err != nil {
				return err
			}
			if c.IsToken() {
				resp.DisplayName = p.DisplayName
			}
			resp.Memberships = append(resp.Memberships, &yariv1.Membership{
				OrganizationId: org.String(), OrganizationSlug: o.Slug, PrincipalId: p.ID.String(), PrincipalKind: principalKinds[p.Kind],
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(resp), nil
}

// CreateApiToken creates a token and returns its plaintext once.
func (s *Server) CreateApiToken(ctx context.Context, req *connect.Request[yariv1.CreateApiTokenRequest]) (*connect.Response[yariv1.CreateApiTokenResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(req.Msg.TtlSeconds) * time.Second
	if ttl < minTokenTTL || ttl > maxTokenTTL {
		return nil, invalid("ttl_seconds must be between %d and %d", int(minTokenTTL.Seconds()), int(maxTokenTTL.Seconds()))
	}
	if len(req.Msg.Label) > 120 {
		return nil, invalid("label is too long")
	}
	var resp *yariv1.CreateApiTokenResponse
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		// Agents never mint credentials.
		if t.P.Kind == authz.KindAgent {
			return connect.NewError(connect.CodePermissionDenied, errors.New("agents cannot create API tokens"))
		}
		target := t.P.ID
		if req.Msg.PrincipalId != "" {
			var err error
			if target, err = parseID("principal_id", req.Msg.PrincipalId); err != nil {
				return err
			}
		}
		orgScope := authz.Scope{Type: authz.ScopeOrganization, ID: org}
		if target != t.P.ID {
			if _, err := t.check(ctx, authz.MemberManage, orgScope); err != nil {
				return err
			}
		}
		if _, err := t.Q.GetPrincipal(ctx, target); err != nil {
			return err
		}
		plain, prefix, hash, err := authn.GenerateToken()
		if err != nil {
			return err
		}
		tok, err := t.Q.CreateAPIToken(ctx, dbsqlc.CreateAPITokenParams{
			ID: db.NewID(), OrganizationID: org, PrincipalID: target, TokenHash: hash, Prefix: prefix,
			Label: req.Msg.Label, ExpiresAt: time.Now().Add(ttl),
		})
		if err != nil {
			return err
		}
		resp = &yariv1.CreateApiTokenResponse{TokenId: tok.ID.String(), Token: plain, Prefix: prefix, ExpiresAt: ts(tok.ExpiresAt)}
		return t.audit(ctx, "api_token.create", orgScope, tok.ID.String(), map[string]any{"prefix": prefix, "principal_id": target.String(), "ttl_seconds": req.Msg.TtlSeconds}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// RevokeApiToken revokes a token. Principals may revoke their own tokens.
func (s *Server) RevokeApiToken(ctx context.Context, req *connect.Request[yariv1.RevokeApiTokenRequest]) (*connect.Response[yariv1.RevokeApiTokenResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	tokenID, err := parseID("token_id", req.Msg.TokenId)
	if err != nil {
		return nil, err
	}
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		tok, err := t.Q.GetAPIToken(ctx, tokenID)
		if err != nil {
			return err
		}
		orgScope := authz.Scope{Type: authz.ScopeOrganization, ID: org}
		if tok.PrincipalID != t.P.ID {
			if _, err := t.check(ctx, authz.MemberManage, orgScope); err != nil {
				return err
			}
		}
		if _, err := t.Q.RevokeAPIToken(ctx, tokenID); err != nil {
			return err
		}
		return t.audit(ctx, "api_token.revoke", orgScope, tokenID.String(), map[string]any{"prefix": tok.Prefix}, uuid.Nil, uuid.Nil)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.RevokeApiTokenResponse{}), nil
}
