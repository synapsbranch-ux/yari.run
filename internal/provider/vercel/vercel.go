// Package vercel is the yari.run adapter stub for Vercel.
//
// TODO: implement the adapter against the official API: https://vercel.com/docs/rest-api
// Until then every method except Name and Capabilities returns provider.ErrNotImplemented.
// This stub makes no network calls.
package vercel

import "github.com/synapsbranch-ux/yari.run/internal/provider"

// ID is the Provider Catalog id.
const ID = "vercel"

// Provider is the Vercel adapter stub.
type Provider struct{ provider.Unimplemented }

// New returns the stub.
func New() *Provider { return &Provider{} }

// Name implements provider.Provider.
func (*Provider) Name() string { return ID }

// Capabilities implements provider.Provider. It must match the catalog entry.
func (*Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{provider.KindWeb: true, provider.KindFunction: true, provider.KindStatic: true, provider.KindCron: true}
}
