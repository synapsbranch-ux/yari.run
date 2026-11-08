// Package fake is an in-memory provider for tests and demos. It supports every Kind and returns
// deterministic, obviously fake outputs. It makes no network calls.
package fake

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

// Lease is a fake lease: a fixed token that is obviously not real.
type Lease struct{ ScopeName string }

// Token implements provider.Lease.
func (Lease) Token(context.Context) (string, error) { return "fake-lease-token", nil }

// Scope implements provider.Lease.
func (l Lease) Scope() string { return l.ScopeName }

// Provider is the in-memory provider. The zero value is not usable; call New.
type Provider struct {
	name string

	mu      sync.Mutex
	state   map[string]provider.ObservedService // keyed by service name
	applied []provider.Change                   // every change applied, in order
	// FailOn makes Apply fail for the named services. The error text may contain a secret,
	// which lets tests check that the orchestrator redacts it.
	FailOn map[string]error
}

// New returns a fake provider reporting the given name.
func New(name string) *Provider {
	return &Provider{name: name, state: map[string]provider.ObservedService{}, FailOn: map[string]error{}}
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return p.name }

// Capabilities implements provider.Provider: every kind.
func (p *Provider) Capabilities() provider.CapabilitySet {
	out := provider.CapabilitySet{}
	for _, k := range provider.AllKinds {
		out[k] = true
	}
	return out
}

// Validate implements provider.Provider.
func (p *Provider) Validate(context.Context, provider.Lease) error { return nil }

// Plan implements provider.Provider by diffing desired against observed.
func (p *Provider) Plan(_ context.Context, _ provider.Lease, desired []provider.ServiceSpec, observed []provider.ObservedService) ([]provider.Change, error) {
	obs := map[string]provider.ObservedService{}
	for _, o := range observed {
		if o.Spec != nil {
			obs[o.Spec.Name] = o
		}
	}
	var out []provider.Change
	seen := map[string]bool{}
	for i := range desired {
		d := desired[i]
		seen[d.Name] = true
		o, exists := obs[d.Name]
		switch {
		case !exists:
			out = append(out, provider.Change{Service: d.Name, Action: provider.ActionCreate, After: &d, Risk: provider.RiskLow, Description: fmt.Sprintf("create %s %s", d.Kind, d.Name)})
		case o.Spec.Kind != d.Kind || !sameConfig(o.Spec.Config, d.Config):
			out = append(out, provider.Change{Service: d.Name, Action: provider.ActionUpdate, After: &d, Risk: provider.RiskMedium, Description: fmt.Sprintf("update %s", d.Name)})
		default:
			out = append(out, provider.Change{Service: d.Name, Action: provider.ActionNoop, After: &d, Risk: provider.RiskLow})
		}
	}
	names := make([]string, 0, len(obs))
	for n := range obs {
		if !seen[n] {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, provider.Change{Service: n, Action: provider.ActionDelete, Risk: provider.RiskHigh, Description: "delete " + n})
	}
	return out, nil
}

func sameConfig(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Apply implements provider.Provider.
func (p *Provider) Apply(_ context.Context, _ provider.Lease, c provider.Change) (provider.Ref, provider.Outputs, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.FailOn[c.Service]; err != nil {
		return provider.Ref{}, nil, err
	}
	p.applied = append(p.applied, c)
	ref := provider.Ref{Provider: p.name, ExternalID: "fake-" + c.Service, Tags: map[string]string{"yari:service": c.Service}}
	switch c.Action {
	case provider.ActionDelete:
		delete(p.state, c.Service)
		return ref, provider.Outputs{}, nil
	case provider.ActionNoop:
		return ref, provider.Outputs{}, nil
	}
	if c.After == nil {
		return provider.Ref{}, nil, fmt.Errorf("fake: %s change for %q has no spec", c.Action, c.Service)
	}
	spec := *c.After
	p.state[c.Service] = provider.ObservedService{Ref: ref, Status: "online", Spec: &spec}
	return ref, outputsFor(spec), nil
}

// outputsFor returns deterministic fake outputs by kind.
func outputsFor(s provider.ServiceSpec) provider.Outputs {
	out := provider.Outputs{"ID": {Value: "fake-" + s.Name}}
	switch s.Kind {
	case provider.KindWeb, provider.KindStatic, provider.KindFunction:
		out["URL"] = provider.Output{Value: "https://" + s.Name + ".fake.invalid"}
	case provider.KindPostgres:
		out["DATABASE_URL"] = provider.Output{Value: "postgres://fake:fake@fake.invalid/" + s.Name, Secret: true}
	case provider.KindMySQL:
		out["DATABASE_URL"] = provider.Output{Value: "mysql://fake:fake@fake.invalid/" + s.Name, Secret: true}
	case provider.KindRedis, provider.KindKV:
		out["REDIS_URL"] = provider.Output{Value: "redis://fake:fake@fake.invalid:6379", Secret: true}
	case provider.KindBucket:
		out["BUCKET_ENDPOINT"] = provider.Output{Value: "https://" + s.Name + ".s3.fake.invalid"}
	case provider.KindQueue:
		out["QUEUE_URL"] = provider.Output{Value: "https://queue.fake.invalid/" + s.Name}
	}
	return out
}

// Observe implements provider.Provider.
func (p *Provider) Observe(_ context.Context, _ provider.Lease, ref provider.Ref) (provider.ObservedService, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, o := range p.state {
		if o.Ref.ExternalID == ref.ExternalID {
			return o, nil
		}
	}
	return provider.ObservedService{Ref: ref, Status: "not_found"}, nil
}

// Logs implements provider.Provider with a few fake lines.
func (p *Provider) Logs(_ context.Context, _ provider.Lease, ref provider.Ref, _ provider.LogQuery) (<-chan provider.LogLine, error) {
	ch := make(chan provider.LogLine, 3)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, m := range []string{"starting", "listening", "healthy"} {
		ch <- provider.LogLine{At: base.Add(time.Duration(i) * time.Second), Stream: "stdout", Message: ref.ExternalID + ": " + m}
	}
	close(ch)
	return ch, nil
}

// Metrics implements provider.Provider with a flat fake series.
func (p *Provider) Metrics(_ context.Context, _ provider.Lease, _ provider.Ref, q provider.MetricQuery) ([]provider.Point, error) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []provider.Point{{At: base, Value: 0.1}, {At: base.Add(time.Minute), Value: 0.2}}, nil
}

// Destroy implements provider.Provider.
func (p *Provider) Destroy(_ context.Context, _ provider.Lease, ref provider.Ref) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, o := range p.state {
		if o.Ref.ExternalID == ref.ExternalID || name == ref.Tags["yari:service"] {
			delete(p.state, name)
		}
	}
	return nil
}

// Applied returns a copy of the changes applied so far.
func (p *Provider) Applied() []provider.Change {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]provider.Change(nil), p.applied...)
}

// Names returns the names of the services currently held, sorted.
func (p *Provider) Names() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.state))
	for n := range p.state {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Registry returns a registry that serves every catalog id with its own fake provider.
// It is what the plan worker uses in this scaffold.
func Registry(docs provider.DocsResolver, ids []string) (*provider.Registry, error) {
	reg := provider.NewRegistry(docs)
	for _, id := range ids {
		if err := reg.Register(id, New(id)); err != nil {
			return nil, err
		}
	}
	return reg, nil
}
