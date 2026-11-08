// Package railway is the yari.run adapter stub for Railway.
//
// TODO: implement the adapter against the official API: https://docs.railway.com/reference/public-api
// Until then every method except Name and Capabilities returns provider.ErrNotImplemented.
// This stub makes no network calls.
package railway

import "github.com/synapsbranch-ux/yari.run/internal/provider"

// ID is the Provider Catalog id.
const ID = "railway"

// Provider is the Railway adapter stub.
type Provider struct{ provider.Unimplemented }

// New returns the stub.
func New() *Provider { return &Provider{} }

// Name implements provider.Provider.
func (*Provider) Name() string { return ID }

// Capabilities implements provider.Provider. It must match the catalog entry.
func (*Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{provider.KindWeb: true, provider.KindWorker: true, provider.KindCron: true, provider.KindPostgres: true, provider.KindRedis: true}
}
