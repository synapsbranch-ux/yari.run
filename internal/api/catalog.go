package api

import (
	"context"

	"connectrpc.com/connect"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

// ListProviders lists the Provider Catalog, optionally filtered by category and kind.
// The catalog is global reference data: any authenticated caller may read it.
func (s *Server) ListProviders(_ context.Context, req *connect.Request[yariv1.ListProvidersRequest]) (*connect.Response[yariv1.ListProvidersResponse], error) {
	if k := provider.Kind(req.Msg.Kind); k != "" && !k.Valid() {
		return nil, invalid("unknown kind %q", req.Msg.Kind)
	}
	resp := &yariv1.ListProvidersResponse{}
	for _, e := range s.Catalog.Filter(req.Msg.Category, provider.Kind(req.Msg.Kind)) {
		resp.Providers = append(resp.Providers, &yariv1.ProviderInfo{
			Id: e.ID, Name: e.Name, Category: e.Category, Kinds: e.Kinds, AuthMethods: e.Auth,
			AdapterFamily: e.Adapter, Status: e.Status, DocsUrl: e.Docs,
		})
	}
	return connect.NewResponse(resp), nil
}
