// Package api implements the yari.run Connect API. It is the only door to the system: the web,
// desktop and mobile clients, the CLI, the MCP server and the Agent all call it, and every call
// is authenticated, authorized with internal/authz, run in a tenant transaction and audited.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/rs/cors"

	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
	"github.com/synapsbranch-ux/yari.run/internal/audit"
	"github.com/synapsbranch-ux/yari.run/internal/authn"
	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/config"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
	"github.com/synapsbranch-ux/yari.run/internal/gateway"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

// ModelLister lists the models allowed by the model gateway.
type ModelLister interface {
	Models(ctx context.Context) ([]gateway.Model, error)
}

// Deps are the collaborators of the API server.
type Deps struct {
	Pool    *pgxpool.Pool
	Config  config.Config
	Authn   *authn.Authenticator
	River   *river.Client[pgx.Tx] // insert-only is enough
	Catalog *catalog.Catalog
	Models  ModelLister
	// WatchPoll is how often WatchSession looks for new steps. Defaults to 500ms.
	WatchPoll time.Duration
}

// Server implements every Connect service of the API.
type Server struct {
	Deps
}

// New returns the API server.
func New(d Deps) *Server {
	if d.WatchPoll == 0 {
		d.WatchPoll = 500 * time.Millisecond
	}
	return &Server{Deps: d}
}

// Handler mounts all services, health and CORS.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	opts := []connect.HandlerOption{connect.WithInterceptors(s.Authn.Interceptor())}
	for _, mount := range []func() (string, http.Handler){
		func() (string, http.Handler) { return yariv1connect.NewIdentityServiceHandler(s, opts...) },
		func() (string, http.Handler) { return yariv1connect.NewHierarchyServiceHandler(s, opts...) },
		func() (string, http.Handler) { return yariv1connect.NewAccessServiceHandler(s, opts...) },
		func() (string, http.Handler) { return yariv1connect.NewPlanServiceHandler(s, opts...) },
		func() (string, http.Handler) { return yariv1connect.NewAuditServiceHandler(s, opts...) },
		func() (string, http.Handler) { return yariv1connect.NewCatalogServiceHandler(s, opts...) },
		func() (string, http.Handler) { return yariv1connect.NewAgentServiceHandler(s, opts...) },
	} {
		path, h := mount()
		mux.Handle(path, h)
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Pool.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	if len(s.Config.CORSOrigins) == 0 {
		return next
	}
	return cors.New(cors.Options{
		AllowedOrigins: s.Config.CORSOrigins,
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: append(connectcors.AllowedHeaders(), "Authorization", authn.DevSubjectHeader),
		ExposedHeaders: connectcors.ExposedHeaders(),
		MaxAge:         7200,
	}).Handler(next)
}

// tx is what a handler gets inside a tenant transaction.
type tx struct {
	Tx  pgx.Tx
	Q   *dbsqlc.Queries
	Org uuid.UUID
	P   authz.Principal
	Az  *authz.Authorizer
}

func newTx(t pgx.Tx, org uuid.UUID, p authz.Principal) *tx {
	q := dbsqlc.New(t)
	return &tx{Tx: t, Q: q, Org: org, P: p, Az: authz.New(authz.DBLoader{Q: q})}
}

// check runs an authorization check and turns a denial into a PermissionDenied error.
func (t *tx) check(ctx context.Context, perm authz.Permission, scope authz.Scope) (authz.Decision, error) {
	d, err := t.Az.Can(ctx, t.P, perm, scope)
	if err != nil {
		return d, err
	}
	if !d.Allowed {
		return d, denied(perm, d)
	}
	return d, nil
}

func denied(perm authz.Permission, d authz.Decision) error {
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s denied: %s", perm, d.Reason))
}

// audit writes an audit event for the caller in this transaction.
func (t *tx) audit(ctx context.Context, action string, scope authz.Scope, target string, meta map[string]any, plan, session uuid.UUID) error {
	return audit.Record(ctx, t.Q, audit.Event{
		OrganizationID: t.Org, PrincipalID: t.P.ID, Action: action,
		ScopeType: string(scope.Type), ScopeID: scope.ID, Target: target,
		PlanID: plan, AgentSessionID: session, Metadata: meta,
	})
}

// candidateOrgs lists the Organizations the caller may act in.
func (s *Server) candidateOrgs(ctx context.Context, c *authn.Caller) ([]uuid.UUID, error) {
	if c.IsToken() {
		return []uuid.UUID{c.Principal.OrganizationID}, nil
	}
	return db.OrganizationsForUser(ctx, s.Pool, c.UserID)
}

// principalIn resolves the caller's principal inside an Organization.
func principalIn(ctx context.Context, q *dbsqlc.Queries, c *authn.Caller, org uuid.UUID) (authz.Principal, error) {
	if c.IsToken() {
		if c.Principal.OrganizationID != org {
			return authz.Principal{}, connect.NewError(connect.CodePermissionDenied, errors.New("this token belongs to another organization"))
		}
		return *c.Principal, nil
	}
	p, err := q.GetHumanPrincipal(ctx, uuid.NullUUID{UUID: c.UserID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return authz.Principal{}, connect.NewError(connect.CodePermissionDenied, errors.New("you are not a member of this organization"))
	}
	if err != nil {
		return authz.Principal{}, err
	}
	return authz.Principal{ID: p.ID, OrganizationID: org, Kind: authz.PrincipalKind(p.Kind)}, nil
}

func caller(ctx context.Context) (*authn.Caller, error) {
	c := authn.FromContext(ctx)
	if c == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	return c, nil
}

// inOrg runs fn in a tenant transaction of the given Organization as the caller.
func (s *Server) inOrg(ctx context.Context, org uuid.UUID, fn func(ctx context.Context, t *tx) error) error {
	c, err := caller(ctx)
	if err != nil {
		return err
	}
	return mapErr(db.WithTenant(ctx, s.Pool, org, func(ctx context.Context, pt pgx.Tx) error {
		p, err := principalIn(ctx, dbsqlc.New(pt), c, org)
		if err != nil {
			return err
		}
		return fn(ctx, newTx(pt, org, p))
	}))
}

// errNotHere means a lookup did not find its target in this Organization. findOrg keeps looking.
var errNotHere = errors.New("not in this organization")

// findOrg runs fn in each candidate Organization until it stops returning errNotHere. Row-level
// security makes a resource invisible outside its Organization, so this finds the owner of an
// id without a global index. Returns NotFound when no Organization has it.
func (s *Server) findOrg(ctx context.Context, what string, fn func(ctx context.Context, t *tx) error) error {
	c, err := caller(ctx)
	if err != nil {
		return err
	}
	orgs, err := s.candidateOrgs(ctx, c)
	if err != nil {
		return mapErr(err)
	}
	for _, org := range orgs {
		err := s.inOrg(ctx, org, func(ctx context.Context, t *tx) error { return fn(ctx, t) })
		if errors.Is(err, errNotHere) {
			continue
		}
		return err
	}
	return connect.NewError(connect.CodeNotFound, fmt.Errorf("%s not found", what))
}

var uniqueViolation = "23505"

// mapErr converts database and domain errors to Connect errors. Connect errors pass through.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return connect.NewError(connect.CodeNotFound, errors.New("not found"))
	case errors.As(err, &pg):
		switch pg.Code {
		case uniqueViolation:
			return connect.NewError(connect.CodeAlreadyExists, errors.New("already exists"))
		case "23503", "23514", "22P02": // foreign key, check, invalid text representation
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid request: %s", pg.ConstraintName))
		case "42501": // row-level security or privilege
			return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
		}
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	}
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func invalid(format string, args ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, args...))
}

func parseID(field, v string) (uuid.UUID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, invalid("%s must be a UUID", field)
	}
	return id, nil
}

var slugRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

func validSlug(field, v string) error {
	if !slugRE.MatchString(v) {
		return invalid("%s must be lowercase letters, digits and dashes (1 to 64 characters)", field)
	}
	return nil
}

func validName(field, v string) error {
	if len(v) == 0 || len(v) > 120 {
		return invalid("%s must be 1 to 120 characters", field)
	}
	return nil
}
