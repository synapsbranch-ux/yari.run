package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/jobs"
)

const (
	maxGoalBytes     = 4000
	defaultSessionPg = 20
	maxSessionPg     = 100
)

// SessionTokenLabel is the label of the session-scoped API token of an Agent Session.
func SessionTokenLabel(session uuid.UUID) string { return "agent-session:" + session.String() }

func envScope(id uuid.UUID) authz.Scope { return authz.Scope{Type: authz.ScopeEnvironment, ID: id} }

// ListModels lists the models the gateway allows.
func (s *Server) ListModels(ctx context.Context, _ *connect.Request[yariv1.ListModelsRequest]) (*connect.Response[yariv1.ListModelsResponse], error) {
	models, err := s.Models.Models(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("model gateway: %w", err))
	}
	resp := &yariv1.ListModelsResponse{}
	for _, m := range models {
		resp.Models = append(resp.Models, &yariv1.ModelInfo{Alias: m.Alias, DisplayName: m.DisplayName, Provider: m.Provider})
	}
	return connect.NewResponse(resp), nil
}

// StartSession creates an Agent Session. In one transaction it creates (or reuses) the
// workspace's agent principal with a developer grant on the environment, the session, a
// session-scoped API token that expires with the session, and the River job that runs it.
// The token plaintext goes only into the job args.
func (s *Server) StartSession(ctx context.Context, req *connect.Request[yariv1.StartSessionRequest]) (*connect.Response[yariv1.StartSessionResponse], error) {
	envID, err := parseID("environment_id", req.Msg.EnvironmentId)
	if err != nil {
		return nil, err
	}
	goal := strings.TrimSpace(req.Msg.Goal)
	if goal == "" || len(goal) > maxGoalBytes {
		return nil, invalid("goal must be 1 to %d bytes", maxGoalBytes)
	}
	model := req.Msg.Model
	if model == "" {
		model = s.Config.AgentDefaultModel
	}
	models, err := s.Models.Models(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("model gateway: %w", err))
	}
	allowed := false
	for _, m := range models {
		if m.Alias == model {
			allowed = true
		}
	}
	if !allowed {
		return nil, invalid("model %q is not allowed by the gateway (see ListModels)", model)
	}

	var sess dbsqlc.AgentSession
	err = s.findOrg(ctx, "environment", func(ctx context.Context, t *tx) error {
		env, err := t.Q.GetEnvironment(ctx, envID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotHere
		}
		if err != nil {
			return err
		}
		scope := envScope(envID)
		if _, err := t.check(ctx, authz.AgentRun, scope); err != nil {
			return err
		}
		project, err := t.Q.GetProject(ctx, env.ProjectID)
		if err != nil {
			return err
		}
		ws, err := t.Q.GetWorkspace(ctx, project.WorkspaceID)
		if err != nil {
			return err
		}

		agentName := "yari-agent/" + ws.Slug
		agent, err := t.Q.GetAgentPrincipalByName(ctx, agentName)
		if errors.Is(err, pgx.ErrNoRows) {
			agent, err = t.Q.CreatePrincipal(ctx, dbsqlc.CreatePrincipalParams{ID: db.NewID(), OrganizationID: t.Org, Kind: "agent", DisplayName: agentName})
		}
		if err != nil {
			return err
		}
		if _, err := t.Q.CreateGrant(ctx, dbsqlc.CreateGrantParams{
			ID: db.NewID(), OrganizationID: t.Org, PrincipalID: agent.ID, Role: string(authz.RoleDeveloper),
			ScopeType: string(authz.ScopeEnvironment), ScopeID: envID, CreatedBy: t.P.ID,
		}); err != nil {
			return err
		}

		ttl := s.Config.AgentSessionTTL
		sessionID := db.NewID()
		if sess, err = t.Q.CreateAgentSession(ctx, dbsqlc.CreateAgentSessionParams{
			ID: sessionID, OrganizationID: t.Org, EnvironmentID: envID, AgentPrincipalID: agent.ID, StartedBy: t.P.ID,
			Goal: goal, Runtime: s.Config.AgentRuntime, Model: model,
			BudgetUsdMicros: int64(s.Config.AgentSessionBudget * 1_000_000), MaxIterations: int32(s.Config.AgentMaxIterations),
		}); err != nil {
			return err
		}

		plain, prefix, hash, err := authn.GenerateToken()
		if err != nil {
			return err
		}
		if _, err := t.Q.CreateAPIToken(ctx, dbsqlc.CreateAPITokenParams{
			ID: db.NewID(), OrganizationID: t.Org, PrincipalID: agent.ID, TokenHash: hash, Prefix: prefix,
			Label: SessionTokenLabel(sessionID), ExpiresAt: time.Now().Add(ttl),
		}); err != nil {
			return err
		}
		if _, err := s.River.InsertTx(ctx, t.Tx, jobs.RunAgentSessionArgs{OrganizationID: t.Org, SessionID: sessionID, MCPToken: plain}, nil); err != nil {
			return fmt.Errorf("enqueue session: %w", err)
		}
		return t.audit(ctx, "agent.session.create", scope, sessionID.String(),
			map[string]any{"model": model, "runtime": sess.Runtime, "budget_usd_micros": sess.BudgetUsdMicros}, uuid.Nil, sessionID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.StartSessionResponse{Session: sessionToProto(sess)}), nil
}

func (s *Server) loadSession(ctx context.Context, t *tx, id uuid.UUID) (dbsqlc.AgentSession, error) {
	sess, err := t.Q.GetAgentSession(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sess, errNotHere
	}
	return sess, err
}

// GetSession returns one Agent Session. Needs agent.read.
func (s *Server) GetSession(ctx context.Context, req *connect.Request[yariv1.GetSessionRequest]) (*connect.Response[yariv1.GetSessionResponse], error) {
	id, err := parseID("session_id", req.Msg.SessionId)
	if err != nil {
		return nil, err
	}
	var sess dbsqlc.AgentSession
	err = s.findOrg(ctx, "session", func(ctx context.Context, t *tx) error {
		var err error
		if sess, err = s.loadSession(ctx, t, id); err != nil {
			return err
		}
		_, err = t.check(ctx, authz.AgentRead, envScope(sess.EnvironmentID))
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.GetSessionResponse{Session: sessionToProto(sess)}), nil
}

// ListSessions lists Agent Sessions the caller can read, newest first.
func (s *Server) ListSessions(ctx context.Context, req *connect.Request[yariv1.ListSessionsRequest]) (*connect.Response[yariv1.ListSessionsResponse], error) {
	size := int(req.Msg.PageSize)
	if size <= 0 {
		size = defaultSessionPg
	}
	if size > maxSessionPg {
		size = maxSessionPg
	}
	filter := uuid.NullUUID{}
	if req.Msg.EnvironmentId != "" {
		id, err := parseID("environment_id", req.Msg.EnvironmentId)
		if err != nil {
			return nil, err
		}
		filter = uuid.NullUUID{UUID: id, Valid: true}
	}
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.candidateOrgs(ctx, c)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &yariv1.ListSessionsResponse{}
	for _, org := range orgs {
		err := s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
			rows, err := t.Q.ListAgentSessions(ctx, dbsqlc.ListAgentSessionsParams{EnvironmentID: filter, PageSize: int32(size)})
			if err != nil {
				return err
			}
			for _, r := range rows {
				d, err := t.Az.Can(ctx, t.P, authz.AgentRead, envScope(r.EnvironmentID))
				if err != nil {
					return err
				}
				if d.Allowed {
					resp.Sessions = append(resp.Sessions, sessionToProto(r))
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(resp), nil
}

// SendMessage sends a follow-up instruction to a running session. Needs agent.run.
func (s *Server) SendMessage(ctx context.Context, req *connect.Request[yariv1.SendMessageRequest]) (*connect.Response[yariv1.SendMessageResponse], error) {
	id, err := parseID("session_id", req.Msg.SessionId)
	if err != nil {
		return nil, err
	}
	msg := strings.TrimSpace(req.Msg.Message)
	if msg == "" || len(msg) > jobs.MaxMessageBytes {
		return nil, invalid("message must be 1 to %d bytes", jobs.MaxMessageBytes)
	}
	err = s.findOrg(ctx, "session", func(ctx context.Context, t *tx) error {
		sess, err := s.loadSession(ctx, t, id)
		if err != nil {
			return err
		}
		scope := envScope(sess.EnvironmentID)
		if _, err := t.check(ctx, authz.AgentRun, scope); err != nil {
			return err
		}
		if sess.Status != "running" {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("session is %s, messages can only be sent to a running session", sess.Status))
		}
		if err := jobs.Notify(ctx, t.Tx, jobs.Command{SessionID: id, Type: jobs.CommandSend, Message: msg}); err != nil {
			return err
		}
		return t.audit(ctx, "agent.session.message", scope, id.String(), map[string]any{"bytes": len(msg)}, uuid.Nil, id)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.SendMessageResponse{}), nil
}

// StopSession stops a session. A queued session stops at once; a running one is told to stop.
func (s *Server) StopSession(ctx context.Context, req *connect.Request[yariv1.StopSessionRequest]) (*connect.Response[yariv1.StopSessionResponse], error) {
	id, err := parseID("session_id", req.Msg.SessionId)
	if err != nil {
		return nil, err
	}
	var sess dbsqlc.AgentSession
	err = s.findOrg(ctx, "session", func(ctx context.Context, t *tx) error {
		var err error
		if sess, err = s.loadSession(ctx, t, id); err != nil {
			return err
		}
		scope := envScope(sess.EnvironmentID)
		if _, err := t.check(ctx, authz.AgentRun, scope); err != nil {
			return err
		}
		switch sess.Status {
		case "queued":
			if sess, err = t.Q.FinishAgentSession(ctx, dbsqlc.FinishAgentSessionParams{ID: id, Status: "stopped"}); err != nil {
				return err
			}
			if _, err := t.Q.RevokeAPITokensByLabel(ctx, SessionTokenLabel(id)); err != nil {
				return err
			}
		case "running":
			if err := jobs.Notify(ctx, t.Tx, jobs.Command{SessionID: id, Type: jobs.CommandStop}); err != nil {
				return err
			}
		default:
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("session already ended (%s)", sess.Status))
		}
		return t.audit(ctx, "agent.session.stop", scope, id.String(), nil, uuid.Nil, id)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&yariv1.StopSessionResponse{Session: sessionToProto(sess)}), nil
}

func sessionEnded(status string) bool {
	switch status {
	case "succeeded", "failed", "stopped", "waiting_approval":
		return true
	}
	return false
}

// WatchSession streams steps as the worker writes them. The worker and the API are separate
// processes that share only PostgreSQL, so this polls the step log.
func (s *Server) WatchSession(ctx context.Context, req *connect.Request[yariv1.WatchSessionRequest], stream *connect.ServerStream[yariv1.WatchSessionResponse]) error {
	id, err := parseID("session_id", req.Msg.SessionId)
	if err != nil {
		return err
	}
	var org uuid.UUID
	err = s.findOrg(ctx, "session", func(ctx context.Context, t *tx) error {
		sess, err := s.loadSession(ctx, t, id)
		if err != nil {
			return err
		}
		org = t.Org
		_, err = t.check(ctx, authz.AgentRead, envScope(sess.EnvironmentID))
		return err
	})
	if err != nil {
		return err
	}

	after := req.Msg.AfterSeq
	lastStatus := ""
	ticker := time.NewTicker(s.WatchPoll)
	defer ticker.Stop()
	for {
		var sess dbsqlc.AgentSession
		var steps []dbsqlc.AgentStep
		err := s.inOrg(ctx, org, func(ctx context.Context, t *tx) error {
			var err error
			// Status first, then steps: a terminal status means every step is already written.
			if sess, err = t.Q.GetAgentSession(ctx, id); err != nil {
				return err
			}
			steps, err = t.Q.ListAgentSteps(ctx, dbsqlc.ListAgentStepsParams{SessionID: id, AfterSeq: after})
			return err
		})
		if err != nil {
			return err
		}
		for _, st := range steps {
			if err := stream.Send(&yariv1.WatchSessionResponse{Step: stepToProto(st), Status: sessionStatuses[sess.Status]}); err != nil {
				return err
			}
			after = st.Seq
			lastStatus = sess.Status
		}
		if sess.Status != lastStatus {
			if err := stream.Send(&yariv1.WatchSessionResponse{Status: sessionStatuses[sess.Status]}); err != nil {
				return err
			}
			lastStatus = sess.Status
		}
		if sessionEnded(sess.Status) && len(steps) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
