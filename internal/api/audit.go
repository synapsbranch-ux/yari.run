package api

import (
	"context"
	"encoding/base64"
	"strconv"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

const (
	defaultAuditPage = 50
	maxAuditPage     = 200
)

// ListAuditEvents lists audit events, newest first. Needs audit.read on the Organization.
func (s *Server) ListAuditEvents(ctx context.Context, req *connect.Request[yariv1.ListAuditEventsRequest]) (*connect.Response[yariv1.ListAuditEventsResponse], error) {
	org, err := parseID("organization_id", req.Msg.OrganizationId)
	if err != nil {
		return nil, err
	}
	size := int(req.Msg.PageSize)
	if size <= 0 {
		size = defaultAuditPage
	}
	if size > maxAuditPage {
		size = maxAuditPage
	}
	var before int64
	if req.Msg.PageToken != "" {
		raw, err := base64.RawURLEncoding.DecodeString(req.Msg.PageToken)
		if err == nil {
			before, err = strconv.ParseInt(string(raw), 10, 64)
		}
		if err != nil || before <= 0 {
			return nil, invalid("page_token is invalid")
		}
	}
	params := struct {
		scopeType pgtype.Text
		scopeID   uuid.NullUUID
	}{}
	if req.Msg.Scope != nil {
		sc, err := scopeFromProto(req.Msg.Scope)
		if err != nil {
			return nil, err
		}
		params.scopeType = pgtype.Text{String: string(sc.Type), Valid: true}
		params.scopeID = uuid.NullUUID{UUID: sc.ID, Valid: true}
	}

	resp := &yariv1.ListAuditEventsResponse{}
	err = s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
		if _, err := t.check(ctx, authz.AuditRead, authz.Scope{Type: authz.ScopeOrganization, ID: org}); err != nil {
			return err
		}
		rows, err := t.Q.ListAuditEvents(ctx, listAuditParams(before, params.scopeType, params.scopeID, int32(size+1)))
		if err != nil {
			return err
		}
		if len(rows) > size {
			rows = rows[:size]
			resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(rows[len(rows)-1].ID, 10)))
		}
		for _, r := range rows {
			e, err := auditToProto(r)
			if err != nil {
				return err
			}
			resp.Events = append(resp.Events, e)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func listAuditParams(before int64, scopeType pgtype.Text, scopeID uuid.NullUUID, limit int32) dbsqlc.ListAuditEventsParams {
	return dbsqlc.ListAuditEventsParams{BeforeID: before, ScopeType: scopeType, ScopeID: scopeID, PageSize: limit}
}
