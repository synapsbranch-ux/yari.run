// Package supabase is the yari.run adapter stub for Supabase.
//
// TODO: implement the adapter against the official API: https://supabase.com/docs/reference/api/introduction
// Until then every method except Name and Capabilities returns provider.ErrNotImplemented.
// This stub makes no network calls.
package supabase

import "github.com/synapsbranch-ux/yari.run/internal/provider"

// ID is the Provider Catalog id.
const ID = "supabase"

// Provider is the Supabase adapter stub.
type Provider struct{ provider.Unimplemented }

// New returns the stub.
func New() *Provider { return &Provider{} }

// Name implements provider.Provider.
func (*Provider) Name() string { return ID }

// Capabilities implements provider.Provider. It must match the catalog entry.
func (*Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{provider.KindPostgres: true, provider.KindBucket: true, provider.KindFunction: true}
}
