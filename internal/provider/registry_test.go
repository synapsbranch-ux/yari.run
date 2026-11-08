package provider_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
	"github.com/synapsbranch-ux/yari.run/internal/provider/all"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
	"github.com/synapsbranch-ux/yari.run/internal/provider/railway"
	"github.com/synapsbranch-ux/yari.run/internal/provider/s3compat"
	"github.com/synapsbranch-ux/yari.run/internal/provider/supabase"
	"github.com/synapsbranch-ux/yari.run/internal/provider/vercel"
)

// Every catalog entry other than the three alpha ones answers ErrNotSupported with its docs link.
func TestEveryPlannedEntryIsNotSupportedWithItsDocsLink(t *testing.T) {
	cat := catalog.MustLoad()
	reg, err := all.Default(cat)
	require.NoError(t, err)

	planned := 0
	for _, e := range cat.Entries() {
		p, err := reg.Get(e.ID)
		if e.Status == catalog.StatusAlpha {
			require.NoError(t, err, e.ID)
			require.NotNil(t, p)
			continue
		}
		planned++
		require.Nil(t, p, e.ID)
		require.ErrorIs(t, err, provider.ErrNotSupported, e.ID)
		require.Contains(t, err.Error(), e.Docs, "the error must carry the docs link for %s", e.ID)
	}
	require.Greater(t, planned, 0)

	_, err = reg.Get("does-not-exist")
	require.Error(t, err)
	require.NotErrorIs(t, err, provider.ErrNotSupported)
}

func TestRegisterRejectsUnknownAndDuplicateIDs(t *testing.T) {
	cat := catalog.MustLoad()
	reg := provider.NewRegistry(cat)
	require.Error(t, reg.Register("not-in-catalog", railway.New()))
	require.NoError(t, reg.Register("railway", railway.New()))
	require.Error(t, reg.Register("railway", railway.New()))
}

// Stub capabilities must match the catalog, and stubs must not pretend to work.
func TestStubsMatchTheCatalogAndReturnNotImplemented(t *testing.T) {
	cat := catalog.MustLoad()
	for id, p := range map[string]provider.Provider{railway.ID: railway.New(), vercel.ID: vercel.New(), supabase.ID: supabase.New()} {
		entry, ok := cat.Get(id)
		require.True(t, ok, id)
		require.Equal(t, id, p.Name())
		require.Len(t, p.Capabilities(), len(entry.Kinds), id)
		for _, k := range entry.Kinds {
			require.True(t, p.Capabilities()[provider.Kind(k)], "%s should support %s", id, k)
		}
		_, _, err := p.Apply(nil, nil, provider.Change{}) //nolint:staticcheck // stubs ignore their arguments
		require.ErrorIs(t, err, provider.ErrNotImplemented)
		require.ErrorIs(t, p.Validate(nil, nil), provider.ErrNotImplemented)                //nolint:staticcheck
		require.ErrorIs(t, p.Destroy(nil, nil, provider.Ref{}), provider.ErrNotImplemented) //nolint:staticcheck
	}
}

func TestS3CompatKnowsAnEndpointPerCatalogEntry(t *testing.T) {
	cat := catalog.MustLoad()
	for _, id := range s3compat.IDs() {
		entry, ok := cat.Get(id)
		require.True(t, ok, id)
		require.Equal(t, "s3compat", entry.Adapter, id)
		p, err := s3compat.New(id)
		require.NoError(t, err)
		require.Equal(t, id, p.Name())
		require.True(t, p.Capabilities()[provider.KindBucket])
		require.Contains(t, p.Endpoint(), "https://")
	}
	// Every catalog entry that claims the s3compat family has an endpoint.
	for _, e := range cat.Entries() {
		if e.Adapter == "s3compat" {
			_, err := s3compat.New(e.ID)
			require.NoError(t, err, e.ID)
		}
	}
	_, err := s3compat.New("nope")
	require.Error(t, err)
}
