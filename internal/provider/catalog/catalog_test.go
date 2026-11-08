package catalog_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

func TestEmbeddedCatalogIsValid(t *testing.T) {
	c, err := catalog.Load()
	require.NoError(t, err)
	entries := c.Entries()
	require.NotEmpty(t, entries)

	alpha := map[string]bool{}
	for _, e := range entries {
		if e.Status == catalog.StatusAlpha {
			alpha[e.ID] = true
		}
		require.True(t, strings.HasPrefix(e.Docs, "https://"), e.ID)
	}
	require.Equal(t, map[string]bool{"railway": true, "vercel": true, "supabase": true}, alpha)
}

func valid() []catalog.Entry {
	mk := func(id, status string) catalog.Entry {
		return catalog.Entry{ID: id, Name: id, Category: "paas", Kinds: []string{"web"}, Auth: []string{"api_token"}, Adapter: "native", Status: status, Docs: "https://example.com/docs"}
	}
	return []catalog.Entry{mk("railway", "alpha"), mk("vercel", "alpha"), mk("supabase", "alpha"), mk("other", "planned")}
}

func TestValidateRules(t *testing.T) {
	require.NoError(t, catalog.Validate(valid()))

	cases := map[string]func([]catalog.Entry) []catalog.Entry{
		"duplicate id":     func(e []catalog.Entry) []catalog.Entry { e[3].ID = "railway"; return e },
		"uppercase id":     func(e []catalog.Entry) []catalog.Entry { e[3].ID = "Other"; return e },
		"unknown category": func(e []catalog.Entry) []catalog.Entry { e[3].Category = "magic"; return e },
		"unknown kind":     func(e []catalog.Entry) []catalog.Entry { e[3].Kinds = []string{"teleporter"}; return e },
		"no kinds":         func(e []catalog.Entry) []catalog.Entry { e[3].Kinds = nil; return e },
		"unknown auth":     func(e []catalog.Entry) []catalog.Entry { e[3].Auth = []string{"vibes"}; return e },
		"unknown adapter":  func(e []catalog.Entry) []catalog.Entry { e[3].Adapter = "wasm"; return e },
		"unknown status":   func(e []catalog.Entry) []catalog.Entry { e[3].Status = "beta"; return e },
		"http docs":        func(e []catalog.Entry) []catalog.Entry { e[3].Docs = "http://example.com"; return e },
		"empty docs":       func(e []catalog.Entry) []catalog.Entry { e[3].Docs = ""; return e },
		"fourth alpha":     func(e []catalog.Entry) []catalog.Entry { e[3].Status = "alpha"; return e },
		"only two alpha":   func(e []catalog.Entry) []catalog.Entry { e[2].Status = "planned"; return e },
		"missing name":     func(e []catalog.Entry) []catalog.Entry { e[3].Name = " "; return e },
		"duplicate kind":   func(e []catalog.Entry) []catalog.Entry { e[3].Kinds = []string{"web", "web"}; return e },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, catalog.Validate(mutate(valid())))
		})
	}
}

func TestFilterAndGet(t *testing.T) {
	c := catalog.MustLoad()
	pg := c.Filter("", provider.KindPostgres)
	require.NotEmpty(t, pg)
	for _, e := range pg {
		require.True(t, e.Supports(provider.KindPostgres))
	}
	for _, e := range c.Filter("storage", "") {
		require.Equal(t, "storage", e.Category)
	}
	e, ok := c.Get("railway")
	require.True(t, ok)
	require.Equal(t, catalog.StatusAlpha, e.Status)
	_, ok = c.Get("nope")
	require.False(t, ok)
}
