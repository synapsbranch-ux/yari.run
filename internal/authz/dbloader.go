package authz

import (
	"context"

	"github.com/google/uuid"

	"github.com/synapsbranch-ux/yari.run/internal/db/dbsqlc"
)

// DBLoader reads authorization data with sqlc queries bound to a tenant transaction.
type DBLoader struct{ Q *dbsqlc.Queries }

// ScopeChain implements Loader.
func (l DBLoader) ScopeChain(ctx context.Context, s Scope) ([]Scope, error) {
	rows, err := l.Q.ScopeChain(ctx, dbsqlc.ScopeChainParams{ScopeType: string(s.Type), ScopeID: s.ID})
	if err != nil {
		return nil, err
	}
	out := make([]Scope, 0, len(rows))
	for _, r := range rows {
		out = append(out, Scope{Type: ScopeType(r.ScopeType), ID: r.ScopeID})
	}
	return out, nil
}

// Grants implements Loader.
func (l DBLoader) Grants(ctx context.Context, principalID uuid.UUID, chain []Scope) ([]Grant, error) {
	ids := make([]uuid.UUID, 0, len(chain))
	on := map[Scope]bool{}
	for _, s := range chain {
		ids = append(ids, s.ID)
		on[s] = true
	}
	rows, err := l.Q.GrantsForPrincipalOverScopes(ctx, dbsqlc.GrantsForPrincipalOverScopesParams{PrincipalID: principalID, ScopeIds: ids})
	if err != nil {
		return nil, err
	}
	var out []Grant
	for _, r := range rows {
		s := Scope{Type: ScopeType(r.ScopeType), ID: r.ScopeID}
		if on[s] { // the query filters on ids only, so compare the type here
			out = append(out, Grant{Role: Role(r.Role), Scope: s})
		}
	}
	return out, nil
}

// Environment implements Loader.
func (l DBLoader) Environment(ctx context.Context, id uuid.UUID) (EnvInfo, error) {
	e, err := l.Q.GetEnvironment(ctx, id)
	if err != nil {
		return EnvInfo{}, err
	}
	return EnvInfo{Protected: e.Protected, RequireSecondApprover: e.RequireSecondApprover}, nil
}
