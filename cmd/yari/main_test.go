package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	yariv1 "github.com/synapsbranch-ux/yari.run/gen/yari/v1"
	"github.com/synapsbranch-ux/yari.run/gen/yari/v1/yariv1connect"
)

type fakeCatalog struct {
	yariv1connect.UnimplementedCatalogServiceHandler
	gotAuth string
	gotCat  string
}

func (f *fakeCatalog) ListProviders(_ context.Context, r *connect.Request[yariv1.ListProvidersRequest]) (*connect.Response[yariv1.ListProvidersResponse], error) {
	f.gotAuth, f.gotCat = r.Header().Get("Authorization"), r.Msg.Category
	return connect.NewResponse(&yariv1.ListProvidersResponse{Providers: []*yariv1.ProviderInfo{{Id: "railway", Category: "paas", Status: "alpha", Kinds: []string{"web", "postgres"}}}}), nil
}

func TestProvidersListSendsTheTokenAndFilter(t *testing.T) {
	f := &fakeCatalog{}
	mux := http.NewServeMux()
	mux.Handle(yariv1connect.NewCatalogServiceHandler(f))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("YARI_API_URL", srv.URL)
	t.Setenv("YARI_TOKEN", "yari_faketoken")
	t.Setenv("YARI_DEV_SUBJECT", "")

	var out bytes.Buffer
	root := newRoot(&out)
	root.SetArgs([]string{"providers", "list", "--category", "paas"})
	require.NoError(t, root.ExecuteContext(context.Background()))
	require.Equal(t, "Bearer yari_faketoken", f.gotAuth)
	require.Equal(t, "paas", f.gotCat)
	require.Contains(t, out.String(), "railway")
	require.Contains(t, out.String(), "alpha")
}

func TestRequiresCredentials(t *testing.T) {
	t.Setenv("YARI_TOKEN", "")
	t.Setenv("YARI_DEV_SUBJECT", "")
	root := newRoot(&bytes.Buffer{})
	root.SetArgs([]string{"whoami"})
	require.ErrorContains(t, root.ExecuteContext(context.Background()), "YARI_TOKEN")
}
