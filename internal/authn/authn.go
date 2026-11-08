// Package authn resolves who is calling the API.
//
// Order: (1) a yari API token (prefix yari_), looked up by SHA-256 hash; (2) an OIDC bearer token
// verified against the Zitadel issuer; (3) only when YARI_ENV=dev, the X-Yari-Dev-Subject header.
package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/synapsbranch-ux/yari.run/internal/authz"
	"github.com/synapsbranch-ux/yari.run/internal/db"
	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

// TokenPrefix starts every yari API token.
const TokenPrefix = "yari_"

// DevSubjectHeader selects a user in the development environment only.
const DevSubjectHeader = "X-Yari-Dev-Subject"

// Caller is the authenticated identity of a request.
type Caller struct {
	// Token callers are bound to one Organization and one Principal.
	Principal *authz.Principal
	// User callers (OIDC, dev header) are a human who may belong to many Organizations.
	UserID  uuid.UUID
	Subject string
	Email   string
}

// IsToken reports whether the caller authenticated with an API token.
func (c *Caller) IsToken() bool { return c.Principal != nil }

type ctxKey struct{}

// WithCaller stores the caller on the context.
func WithCaller(ctx context.Context, c *Caller) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the caller, or nil when the request was not authenticated.
func FromContext(ctx context.Context) *Caller {
	c, _ := ctx.Value(ctxKey{}).(*Caller)
	return c
}

// Config configures an Authenticator.
type Config struct {
	Env      string // YARI_ENV
	Issuer   string // ZITADEL_ISSUER, empty disables OIDC
	ClientID string // ZITADEL_CLIENT_ID
}

// Authenticator resolves callers.
type Authenticator struct {
	pool *pgxpool.Pool
	cfg  Config

	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
}

// New returns an Authenticator. In the dev environment it logs a warning, because the dev header
// lets any client pick any identity.
func New(pool *pgxpool.Pool, cfg Config) *Authenticator {
	if cfg.Env == "dev" {
		slog.Warn("authn: dev identity header is enabled (YARI_ENV=dev). Never run this way in production.", "header", DevSubjectHeader)
	}
	return &Authenticator{pool: pool, cfg: cfg}
}

// GenerateToken returns a new random API token, its display prefix and the hash to store.
func GenerateToken() (plain, prefix string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", nil, fmt.Errorf("authn: random: %w", err)
	}
	plain = TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return plain, plain[:len(TokenPrefix)+6], HashToken(plain), nil
}

// HashToken returns the SHA-256 of a token. Only the hash is ever stored.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Authenticate resolves the caller from request headers.
func (a *Authenticator) Authenticate(ctx context.Context, h http.Header) (*Caller, error) {
	if bearer, ok := strings.CutPrefix(h.Get("Authorization"), "Bearer "); ok && bearer != "" {
		if strings.HasPrefix(bearer, TokenPrefix) {
			return a.byAPIToken(ctx, bearer)
		}
		return a.byOIDC(ctx, bearer)
	}
	if a.cfg.Env == "dev" {
		if subject := h.Get(DevSubjectHeader); subject != "" {
			return a.userCaller(ctx, "dev:"+subject, subject+"@dev.invalid", subject)
		}
	}
	return nil, errors.New("missing credentials")
}

func (a *Authenticator) byAPIToken(ctx context.Context, token string) (*Caller, error) {
	owner, err := db.PrincipalForToken(ctx, a.pool, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("invalid, expired or revoked token")
	}
	if err != nil {
		return nil, fmt.Errorf("token lookup: %w", err)
	}
	var p dbsqlc.Principal
	err = db.WithTenant(ctx, a.pool, owner.OrganizationID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		p, err = dbsqlc.New(tx).GetPrincipal(ctx, owner.PrincipalID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load principal: %w", err)
	}
	return &Caller{Principal: &authz.Principal{ID: p.ID, OrganizationID: p.OrganizationID, Kind: authz.PrincipalKind(p.Kind)}}, nil
}

func (a *Authenticator) byOIDC(ctx context.Context, raw string) (*Caller, error) {
	v, err := a.oidcVerifier(ctx)
	if err != nil {
		return nil, err
	}
	tok, err := v.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid bearer token: %w", err)
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	_ = tok.Claims(&claims) // optional claims
	return a.userCaller(ctx, tok.Subject, claims.Email, claims.Name)
}

func (a *Authenticator) oidcVerifier(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	if a.cfg.Issuer == "" {
		return nil, errors.New("OIDC is not configured (ZITADEL_ISSUER)")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.verifier != nil {
		return a.verifier, nil
	}
	provider, err := oidc.NewProvider(ctx, a.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	conf := &oidc.Config{ClientID: a.cfg.ClientID}
	if a.cfg.ClientID == "" || a.cfg.ClientID == "change-me" {
		conf.SkipClientIDCheck = true
	}
	a.verifier = provider.Verifier(conf)
	return a.verifier, nil
}

// userCaller finds or creates the users row for an external subject.
func (a *Authenticator) userCaller(ctx context.Context, subject, email, name string) (*Caller, error) {
	u, err := dbsqlc.New(a.pool).UpsertUser(ctx, dbsqlc.UpsertUserParams{ID: db.NewID(), ExternalSubject: subject, Email: email, DisplayName: name})
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &Caller{UserID: u.ID, Subject: subject, Email: u.Email}, nil
}

// Interceptor authenticates every Connect call, unary and streaming.
func (a *Authenticator) Interceptor() connect.Interceptor { return &interceptor{a: a} }

type interceptor struct{ a *Authenticator }

func (i *interceptor) auth(ctx context.Context, h http.Header) (context.Context, error) {
	c, err := i.a.Authenticate(ctx, h)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return WithCaller(ctx, c), nil
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.auth(ctx, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.auth(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
