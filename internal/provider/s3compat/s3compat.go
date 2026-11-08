// Package s3compat is the adapter family for S3-compatible object storage: Backblaze B2, Wasabi,
// Tigris, and the object storage of Cloudflare R2, DigitalOcean Spaces, Scaleway, Hetzner, Linode
// and Vultr, which are configured from their parent catalog entries.
//
// TODO: implement bucket lifecycle with the S3 API (CreateBucket, DeleteBucket, HeadBucket) and
// each provider's key management API. Until then every method except Name and Capabilities
// returns provider.ErrNotImplemented. This stub makes no network calls.
package s3compat

import (
	"fmt"
	"sort"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

// Endpoints holds the S3 endpoint per catalog id. {region} is replaced from the Connection config.
var Endpoints = map[string]string{
	"backblaze-b2": "https://s3.{region}.backblazeb2.com",
	"wasabi":       "https://s3.{region}.wasabisys.com",
	"tigris":       "https://t3.storage.dev",
}

// Provider is one member of the family, bound to a catalog id and an endpoint.
type Provider struct {
	provider.Unimplemented
	id       string
	endpoint string
}

// New returns the stub for a catalog id that has a known endpoint.
func New(id string) (*Provider, error) {
	ep, ok := Endpoints[id]
	if !ok {
		return nil, fmt.Errorf("s3compat: no endpoint configured for %q", id)
	}
	return &Provider{id: id, endpoint: ep}, nil
}

// IDs lists the catalog ids the family knows an endpoint for, sorted.
func IDs() []string {
	out := make([]string, 0, len(Endpoints))
	for id := range Endpoints {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Endpoint returns the S3 endpoint template.
func (p *Provider) Endpoint() string { return p.endpoint }

// Name implements provider.Provider.
func (p *Provider) Name() string { return p.id }

// Capabilities implements provider.Provider.
func (*Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{provider.KindBucket: true}
}
