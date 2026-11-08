package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/synapsbranch-ux/yari.run/internal/orchestrator"
	"github.com/synapsbranch-ux/yari.run/internal/provider"
	"github.com/synapsbranch-ux/yari.run/internal/provider/all"
	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
	"github.com/synapsbranch-ux/yari.run/internal/provider/fake"
)

func spec(name string, kind provider.Kind, env map[string]provider.EnvValue) provider.ServiceSpec {
	return provider.ServiceSpec{Name: name, Kind: kind, Provider: "railway", Env: env}
}

func ref(service, key string) provider.EnvValue {
	return provider.EnvValue{FromOutput: &provider.OutputRef{Service: service, Key: key}}
}

func names(layers [][]provider.ServiceSpec) [][]string {
	var out [][]string
	for _, l := range layers {
		var n []string
		for _, s := range l {
			n = append(n, s.Name)
		}
		out = append(out, n)
	}
	return out
}

func TestOrderLayersByDependency(t *testing.T) {
	layers, err := orchestrator.Order([]provider.ServiceSpec{
		spec("web", provider.KindWeb, map[string]provider.EnvValue{"API": ref("api", "URL")}),
		spec("api", provider.KindWeb, map[string]provider.EnvValue{"DB": ref("db", "DATABASE_URL"), "FILES": ref("media", "BUCKET_ENDPOINT")}),
		spec("media", provider.KindBucket, nil),
		spec("db", provider.KindPostgres, nil),
		spec("mail", provider.KindEmail, nil),
	})
	require.NoError(t, err)
	require.Equal(t, [][]string{{"db", "mail", "media"}, {"api"}, {"web"}}, names(layers))
}

func TestOrderIsDeterministic(t *testing.T) {
	in := []provider.ServiceSpec{spec("c", provider.KindWeb, nil), spec("a", provider.KindWeb, nil), spec("b", provider.KindWeb, nil)}
	for i := 0; i < 20; i++ {
		layers, err := orchestrator.Order(in)
		require.NoError(t, err)
		require.Equal(t, [][]string{{"a", "b", "c"}}, names(layers))
	}
}

func TestOrderRejectsCyclesUnknownRefsAndDuplicates(t *testing.T) {
	_, err := orchestrator.Order([]provider.ServiceSpec{
		spec("a", provider.KindWeb, map[string]provider.EnvValue{"X": ref("b", "URL")}),
		spec("b", provider.KindWeb, map[string]provider.EnvValue{"X": ref("c", "URL")}),
		spec("c", provider.KindWeb, map[string]provider.EnvValue{"X": ref("a", "URL")}),
		spec("free", provider.KindWeb, nil),
	})
	require.ErrorContains(t, err, "cycle")
	require.ErrorContains(t, err, "a, b, c")
	require.NotContains(t, err.Error(), "free")

	_, err = orchestrator.Order([]provider.ServiceSpec{spec("a", provider.KindWeb, map[string]provider.EnvValue{"X": ref("a", "URL")})})
	require.ErrorContains(t, err, "cycle", "a self reference is a cycle")

	_, err = orchestrator.Order([]provider.ServiceSpec{spec("a", provider.KindWeb, map[string]provider.EnvValue{"X": ref("ghost", "URL")})})
	require.ErrorContains(t, err, `unknown service "ghost"`)

	_, err = orchestrator.Order([]provider.ServiceSpec{spec("a", provider.KindWeb, nil), spec("a", provider.KindWeb, nil)})
	require.ErrorContains(t, err, "duplicate")
}

type testEnv struct {
	reg    *provider.Registry
	cat    *catalog.Catalog
	leaser orchestrator.Leaser
}

func newEnv(t *testing.T) testEnv {
	t.Helper()
	cat := catalog.MustLoad()
	reg, err := all.Fake(cat)
	require.NoError(t, err)
	return testEnv{reg: reg, cat: cat, leaser: orchestrator.LeaserFunc(func(context.Context, string) (provider.Lease, error) {
		return fake.Lease{ScopeName: "test"}, nil
	})}
}

func create(s provider.ServiceSpec) provider.Change {
	return provider.Change{Service: s.Name, Action: provider.ActionCreate, After: &s}
}

func fakeOf(t *testing.T, e testEnv, id string) *fake.Provider {
	t.Helper()
	p, err := e.reg.Get(id)
	require.NoError(t, err)
	return p.(*fake.Provider)
}

func TestRunPassesOutputsFromFakePostgresIntoFakeWeb(t *testing.T) {
	e := newEnv(t)
	db := spec("db", provider.KindPostgres, nil)
	web := spec("web", provider.KindWeb, map[string]provider.EnvValue{
		"DATABASE_URL": {FromOutput: &provider.OutputRef{Service: "db", Key: "DATABASE_URL"}},
		"STATIC":       {Value: "1"},
	})
	// Submit in the wrong order: the orchestrator must still create db first.
	res, err := orchestrator.Run(context.Background(), e.reg, e.leaser, []provider.Change{create(web), create(db)})
	require.NoError(t, err)

	applied := fakeOf(t, e, "railway").Applied()
	require.Len(t, applied, 2)
	require.Equal(t, "db", applied[0].Service)
	require.Equal(t, "web", applied[1].Service)

	got := applied[1].After.Env["DATABASE_URL"]
	require.Nil(t, got.FromOutput, "the reference is resolved before it reaches the provider")
	require.Equal(t, "postgres://fake:fake@fake.invalid/db", got.Value)
	require.True(t, got.Secret, "a secret output stays secret when injected")
	require.Equal(t, "1", applied[1].After.Env["STATIC"].Value)

	require.Equal(t, "https://web.fake.invalid", res.Outputs["web"]["URL"].Value)
	require.True(t, res.Outputs["db"]["DATABASE_URL"].Secret)
}

func TestRunFailsOnMissingOutputKey(t *testing.T) {
	e := newEnv(t)
	db := spec("db", provider.KindPostgres, nil)
	web := spec("web", provider.KindWeb, map[string]provider.EnvValue{"X": ref("db", "NOPE")})
	_, err := orchestrator.Run(context.Background(), e.reg, e.leaser, []provider.Change{create(db), create(web)})
	require.ErrorContains(t, err, `no output "NOPE"`)
}

func TestRunNeverLeaksSecretsInErrors(t *testing.T) {
	e := newEnv(t)
	db := spec("db", provider.KindPostgres, nil)
	web := spec("web", provider.KindWeb, map[string]provider.EnvValue{"DATABASE_URL": ref("db", "DATABASE_URL")})
	const secret = "postgres://fake:fake@fake.invalid/db"
	fakeOf(t, e, "railway").FailOn = map[string]error{"web": fmt.Errorf("deploy rejected for connection %s (code 42)", secret)}

	_, err := orchestrator.Run(context.Background(), e.reg, e.leaser, []provider.Change{create(db), create(web)})
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), "fake:fake")
	require.Contains(t, err.Error(), "[redacted]")
	require.Contains(t, err.Error(), "web")
	var ae *orchestrator.ApplyError
	require.True(t, errors.As(err, &ae))
}

func TestOutputRedactsItself(t *testing.T) {
	o := provider.Output{Value: "postgres://fake:fake@fake.invalid/x", Secret: true}
	require.NotContains(t, fmt.Sprint(o), "fake:fake")
	require.NotContains(t, fmt.Sprintf("%v %+v %s %#v", o, o, o, o), "fake:fake")
	pub := provider.Output{Value: "https://x.fake.invalid"}
	require.Equal(t, "https://x.fake.invalid", fmt.Sprint(pub))
	out := provider.Outputs{"DATABASE_URL": o}
	require.NotContains(t, fmt.Sprintf("%v", out), "fake:fake")
}

func TestRunStopsAfterAFailingLayerAndDeletesLast(t *testing.T) {
	e := newEnv(t)
	db := spec("db", provider.KindPostgres, nil)
	web := spec("web", provider.KindWeb, map[string]provider.EnvValue{"X": ref("db", "DATABASE_URL")})
	fakeOf(t, e, "railway").FailOn = map[string]error{"db": errors.New("boom")}
	_, err := orchestrator.Run(context.Background(), e.reg, e.leaser, []provider.Change{create(db), create(web)})
	require.ErrorContains(t, err, "boom")
	require.Empty(t, fakeOf(t, e, "railway").Applied(), "the dependent layer never ran")

	e = newEnv(t)
	old := spec("old", provider.KindWorker, nil)
	_, err = orchestrator.Run(context.Background(), e.reg, e.leaser, []provider.Change{
		{Service: "old", Action: provider.ActionDelete, After: &old},
		create(spec("new", provider.KindWorker, nil)),
	})
	require.NoError(t, err)
	require.Equal(t, []string{"new"}, fakeOf(t, e, "railway").Names())
}

func TestRunReportsNotSupportedProviders(t *testing.T) {
	cat := catalog.MustLoad()
	reg, err := all.Default(cat) // real registry: stubs only
	require.NoError(t, err)
	leaser := orchestrator.LeaserFunc(func(context.Context, string) (provider.Lease, error) { return fake.Lease{}, nil })
	s := provider.ServiceSpec{Name: "x", Kind: provider.KindWeb, Provider: "render"}
	_, err = orchestrator.Run(context.Background(), reg, leaser, []provider.Change{create(s)})
	require.ErrorIs(t, err, provider.ErrNotSupported)
	require.Contains(t, err.Error(), "https://")

	s.Provider = "railway" // stub: not implemented
	_, err = orchestrator.Run(context.Background(), reg, leaser, []provider.Change{create(s)})
	require.ErrorIs(t, err, provider.ErrNotImplemented)
	require.False(t, strings.Contains(err.Error(), "panic"))
}
