// Package mcpserver exposes yari.run tools over MCP.
//
// It serves two kinds of clients with the same tools: the OpenHands agent of an Agent Session
// (with a session-scoped token) and a person's own MCP client (with a personal API token). Every
// tool is a call to the Connect API made with the caller's own token, so authorization,
// protected environments and the audit log apply exactly as they do for a human.
//
// This package must never import internal/db or the provider packages. It has no database and no
// provider access: the Connect API is its only door.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
)

// SessionHeader carries the Agent Session id from the agent's MCP configuration.
const SessionHeader = "X-Yari-Session"

// GuardrailPrefix starts the text of every refusal that comes from a guardrail.
const GuardrailPrefix = "guardrail: "

// dataStoreKinds may never be deleted by an agent. The API enforces the same list.
var dataStoreKinds = map[string]bool{"postgres": true, "mysql": true, "mongodb": true, "sqlite": true, "redis": true, "bucket": true}

// Server serves the MCP endpoint.
type Server struct {
	// APIURL is the base URL of the Connect API.
	APIURL string
	// HTTP is the client used to reach the API. Defaults to a client with a timeout.
	HTTP *http.Client
}

// Handler serves /mcp (streamable HTTP, stateless) and /healthz.
func (s *Server) Handler() http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		return s.newMCPServer(&session{api: s.newAPI(token), sessionID: r.Header.Get(SessionHeader)})
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	}))
	return mux
}

type api struct {
	hierarchy yariv1connect.HierarchyServiceClient
	access    yariv1connect.AccessServiceClient
	plan      yariv1connect.PlanServiceClient
	identity  yariv1connect.IdentityServiceClient
}

func (s *Server) newAPI(token string) *api {
	h := s.HTTP
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	opt := connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}))
	return &api{
		hierarchy: yariv1connect.NewHierarchyServiceClient(h, s.APIURL, opt),
		access:    yariv1connect.NewAccessServiceClient(h, s.APIURL, opt),
		plan:      yariv1connect.NewPlanServiceClient(h, s.APIURL, opt),
		identity:  yariv1connect.NewIdentityServiceClient(h, s.APIURL, opt),
	}
}

type session struct {
	api       *api
	sessionID string

	once    sync.Once
	isAgent bool
}

// callerIsAgent asks the API who the token belongs to.
func (s *session) callerIsAgent(ctx context.Context) bool {
	s.once.Do(func() {
		resp, err := s.api.identity.WhoAmI(ctx, connect.NewRequest(&yariv1.WhoAmIRequest{}))
		if err != nil {
			s.isAgent = true // when in doubt, apply the stricter rules
			return
		}
		for _, m := range resp.Msg.Memberships {
			if m.PrincipalKind == yariv1.PrincipalKind_PRINCIPAL_KIND_AGENT {
				s.isAgent = true
			}
		}
	})
	return s.isAgent
}

func text(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError(err)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
}

// toolError reports a failure to the model as tool output, so it can read the reason and adapt.
func toolError(err error) (*mcp.CallToolResult, any, error) {
	msg := err.Error()
	if ce := new(connect.Error); asConnect(err, ce) {
		msg = ce.Message()
	}
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: msg}}}, nil, nil
}

func asConnect(err error, target *connect.Error) bool {
	if ce, ok := err.(*connect.Error); ok {
		*target = *ce
		return true
	}
	return false
}

// EnvArgs identify an environment.
type EnvArgs struct {
	EnvironmentID string `json:"environment_id" jsonschema:"id of the environment to look at"`
}

// LogsArgs select a service's logs.
type LogsArgs struct {
	EnvironmentID string `json:"environment_id" jsonschema:"id of the environment"`
	Service       string `json:"service" jsonschema:"service name"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of lines, default 20"`
}

// MetricsArgs select a metric of a service.
type MetricsArgs struct {
	EnvironmentID string `json:"environment_id" jsonschema:"id of the environment"`
	Service       string `json:"service" jsonschema:"service name"`
	Metric        string `json:"metric,omitempty" jsonschema:"cpu, memory or requests, default cpu"`
}

// EnvValueArg is an environment variable of a service.
type EnvValueArg struct {
	Value       string `json:"value,omitempty" jsonschema:"literal value. Never put secrets here"`
	FromService string `json:"from_service,omitempty" jsonschema:"inject an output of this service instead of a literal"`
	FromKey     string `json:"from_key,omitempty" jsonschema:"name of the output, for example DATABASE_URL"`
	Secret      bool   `json:"secret,omitempty" jsonschema:"true when the value is secret"`
}

// ChangeArg is one change of a plan.
type ChangeArg struct {
	Service  string                 `json:"service" jsonschema:"service name"`
	Action   string                 `json:"action" jsonschema:"create, update, delete or noop"`
	Kind     string                 `json:"kind,omitempty" jsonschema:"web, worker, cron, function, static, postgres, mysql, sqlite, mongodb, redis, kv, bucket, queue, search, email, dns, cdn, vm"`
	Provider string                 `json:"provider,omitempty" jsonschema:"Provider Catalog id, for example railway"`
	Config   map[string]string      `json:"config,omitempty" jsonschema:"provider specific settings"`
	Env      map[string]EnvValueArg `json:"env,omitempty" jsonschema:"environment variables"`
}

// ProposeArgs propose a plan.
type ProposeArgs struct {
	EnvironmentID string      `json:"environment_id" jsonschema:"id of the environment to change"`
	Changes       []ChangeArg `json:"changes" jsonschema:"the changes, one per service"`
}

// PlanArgs identify a plan.
type PlanArgs struct {
	PlanID string `json:"plan_id" jsonschema:"id of the plan"`
}

var actions = map[string]yariv1.ChangeAction{
	"create": yariv1.ChangeAction_CHANGE_ACTION_CREATE, "update": yariv1.ChangeAction_CHANGE_ACTION_UPDATE,
	"delete": yariv1.ChangeAction_CHANGE_ACTION_DELETE, "noop": yariv1.ChangeAction_CHANGE_ACTION_NOOP,
}

func (s *Server) newMCPServer(sess *session) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "yari", Version: "0.1.0"}, nil)

	mcp.AddTool(srv, &mcp.Tool{Name: "spec.read", Description: "Read the desired state of an environment: its flags and its services with provider and status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvArgs) (*mcp.CallToolResult, any, error) {
			env, err := sess.api.hierarchy.GetEnvironment(ctx, connect.NewRequest(&yariv1.GetEnvironmentRequest{EnvironmentId: in.EnvironmentID}))
			if err != nil {
				return toolError(err)
			}
			svc, err := sess.api.hierarchy.ListServices(ctx, connect.NewRequest(&yariv1.ListServicesRequest{EnvironmentId: in.EnvironmentID}))
			if err != nil {
				return toolError(err)
			}
			return text(map[string]any{"environment": env.Msg.Environment, "services": svc.Msg.Services})
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "deployments.status", Description: "Status of every service in an environment."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in EnvArgs) (*mcp.CallToolResult, any, error) {
			svc, err := sess.api.hierarchy.ListServices(ctx, connect.NewRequest(&yariv1.ListServicesRequest{EnvironmentId: in.EnvironmentID}))
			if err != nil {
				return toolError(err)
			}
			out := make([]map[string]string, 0, len(svc.Msg.Services))
			for _, x := range svc.Msg.Services {
				out = append(out, map[string]string{"service": x.Name, "provider": x.Provider, "status": x.Status})
			}
			return text(map[string]any{"deployments": out})
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "logs.query", Description: "Recent log lines of a service. Returns fake data in this scaffold."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in LogsArgs) (*mcp.CallToolResult, any, error) {
			if err := sess.require(ctx, "logs.read", in.EnvironmentID); err != nil {
				return toolError(err)
			}
			n := in.Limit
			if n <= 0 || n > 20 {
				n = 20
			}
			lines := []string{}
			for i := 0; i < n && i < 5; i++ {
				lines = append(lines, fmt.Sprintf("2026-01-01T00:00:%02dZ %s: fake log line %d", i, in.Service, i+1))
			}
			return text(map[string]any{"service": in.Service, "fake": true, "lines": lines})
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "metrics.query", Description: "A metric time series of a service. Returns fake data in this scaffold."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in MetricsArgs) (*mcp.CallToolResult, any, error) {
			if err := sess.require(ctx, "logs.read", in.EnvironmentID); err != nil {
				return toolError(err)
			}
			metric := in.Metric
			if metric == "" {
				metric = "cpu"
			}
			return text(map[string]any{"service": in.Service, "metric": metric, "fake": true, "points": []float64{0.1, 0.2, 0.15, 0.3}})
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "plan.propose", Description: "Propose a Plan of infrastructure changes for review. Nothing changes until the plan is applied and, on protected environments, approved by a person. Agents may not delete databases or buckets."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in ProposeArgs) (*mcp.CallToolResult, any, error) {
			req, err := sess.buildPlan(ctx, in)
			if err != nil {
				return toolError(err)
			}
			resp, err := sess.api.plan.CreatePlan(ctx, connect.NewRequest(req))
			if err != nil {
				return toolError(err)
			}
			return text(planSummary(resp.Msg.Plan))
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "plan.apply", Description: "Apply a proposed plan. On a protected environment this requests approval from a person and returns pending_approval."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in PlanArgs) (*mcp.CallToolResult, any, error) {
			resp, err := sess.api.plan.ApplyPlan(ctx, connect.NewRequest(&yariv1.ApplyPlanRequest{PlanId: in.PlanID}))
			if err != nil {
				return toolError(err)
			}
			out := planSummary(resp.Msg.Plan)
			out["requires_approval"] = resp.Msg.RequiresApproval
			return text(out)
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "plan.status", Description: "Status of a plan."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in PlanArgs) (*mcp.CallToolResult, any, error) {
			resp, err := sess.api.plan.GetPlan(ctx, connect.NewRequest(&yariv1.GetPlanRequest{PlanId: in.PlanID}))
			if err != nil {
				return toolError(err)
			}
			return text(planSummary(resp.Msg.Plan))
		})
	return srv
}

func (s *session) require(ctx context.Context, perm, envID string) error {
	resp, err := s.api.access.CheckPermission(ctx, connect.NewRequest(&yariv1.CheckPermissionRequest{
		Permission: perm, Scope: &yariv1.Scope{Type: yariv1.ScopeType_SCOPE_TYPE_ENVIRONMENT, Id: envID},
	}))
	if err != nil {
		return err
	}
	if !resp.Msg.Decision.Allowed {
		return fmt.Errorf("%s denied: %s", perm, resp.Msg.Decision.Reason)
	}
	return nil
}

func planSummary(p *yariv1.Plan) map[string]any {
	return map[string]any{
		"plan_id": p.Id, "environment_id": p.EnvironmentId, "status": strings.ToLower(strings.TrimPrefix(p.Status.String(), "PLAN_STATUS_")),
		"risk": strings.ToLower(strings.TrimPrefix(p.Risk.String(), "RISK_")), "cost_delta_cents": p.CostDeltaCents, "changes": len(p.Changes),
	}
}

// buildPlan converts tool arguments to an API request and enforces the agent deny list.
func (s *session) buildPlan(ctx context.Context, in ProposeArgs) (*yariv1.CreatePlanRequest, error) {
	req := &yariv1.CreatePlanRequest{EnvironmentId: in.EnvironmentID, AgentSessionId: s.sessionID}
	for _, c := range in.Changes {
		action, ok := actions[strings.ToLower(c.Action)]
		if !ok {
			return nil, fmt.Errorf("change %q: action must be create, update, delete or noop", c.Service)
		}
		if action == yariv1.ChangeAction_CHANGE_ACTION_DELETE && dataStoreKinds[strings.ToLower(c.Kind)] && s.callerIsAgent(ctx) {
			return nil, fmt.Errorf("%sagents may not delete %s services (%q holds data). Ask a person to do it", GuardrailPrefix, c.Kind, c.Service)
		}
		spec := &yariv1.ServiceSpec{Name: c.Service, Kind: strings.ToLower(c.Kind), Provider: c.Provider, Config: c.Config, Env: map[string]*yariv1.EnvValue{}}
		for k, v := range c.Env {
			ev := &yariv1.EnvValue{Value: v.Value, Secret: v.Secret}
			if v.FromService != "" {
				ev.FromOutput = &yariv1.OutputRef{Service: v.FromService, Key: v.FromKey}
			}
			spec.Env[k] = ev
		}
		req.Changes = append(req.Changes, &yariv1.PlanChange{Service: c.Service, Action: action, After: spec})
	}
	return req, nil
}
