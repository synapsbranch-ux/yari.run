// Package all builds the registries the services use.
package all

import (
	"github.com/synapsbranch-ux/yari.run/internal/provider"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
	"github.com/synapsbranch-ux/yari.run/internal/provider/fake"
	"github.com/synapsbranch-ux/yari.run/internal/provider/railway"
	"github.com/synapsbranch-ux/yari.run/internal/provider/supabase"
	"github.com/synapsbranch-ux/yari.run/internal/provider/vercel"
)

// Default registers the MVP adapters (stubs for now). Every other catalog entry answers
// provider.ErrNotSupported with its docs URL.
func Default(cat *catalog.Catalog) (*provider.Registry, error) {
	reg := provider.NewRegistry(cat)
	for id, p := range map[string]provider.Provider{
		railway.ID:  railway.New(),
		vercel.ID:   vercel.New(),
		supabase.ID: supabase.New(),
	} {
		if err := reg.Register(id, p); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// Fake registers an in-memory fake for every catalog id. The plan worker uses it in this scaffold
// so the whole flow runs without any provider credentials.
func Fake(cat *catalog.Catalog) (*provider.Registry, error) {
	var ids []string
	for _, e := range cat.Entries() {
		ids = append(ids, e.ID)
	}
	return fake.Registry(cat, ids)
}
