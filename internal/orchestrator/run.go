package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

// Leaser issues short-lived, operation-scoped credentials for a provider. The credential broker
// will implement it. Until it exists the worker passes a fake leaser.
type Leaser interface {
	Lease(ctx context.Context, providerID string) (provider.Lease, error)
}

// LeaserFunc adapts a function to Leaser.
type LeaserFunc func(ctx context.Context, providerID string) (provider.Lease, error)

// Lease implements Leaser.
func (f LeaserFunc) Lease(ctx context.Context, id string) (provider.Lease, error) { return f(ctx, id) }

// Result is what a successful Run produced.
type Result struct {
	// Refs by service name, for every created or updated service.
	Refs map[string]provider.Ref
	// Outputs by service name. Secret outputs redact themselves when printed.
	Outputs map[string]provider.Outputs
	// Deleted lists the services that were destroyed.
	Deleted []string
}

// ApplyError reports a failed change. Its message never contains a secret value.
type ApplyError struct {
	Service string
	Message string
	cause   error // only a sentinel, never the original error
}

func (e *ApplyError) Error() string { return fmt.Sprintf("apply %s: %s", e.Service, e.Message) }
func (e *ApplyError) Unwrap() error { return e.cause }

// Run applies the changes of a plan: create and update changes layer by layer in dependency
// order (changes in one layer run concurrently), resolving FromOutput references from the
// outputs of earlier layers, then deletes. It stops at the first failing layer.
//
// Secret values are never put in errors or logs: they are replaced before an error leaves Run.
func Run(ctx context.Context, reg *provider.Registry, leaser Leaser, changes []provider.Change) (*Result, error) {
	res := &Result{Refs: map[string]provider.Ref{}, Outputs: map[string]provider.Outputs{}}
	byService := map[string]provider.Change{}
	var specs []provider.ServiceSpec
	var deletes []provider.Change
	for _, c := range changes {
		switch c.Action {
		case provider.ActionCreate, provider.ActionUpdate:
			if c.After == nil {
				return nil, &ApplyError{Service: c.Service, Message: string(c.Action) + " change has no spec"}
			}
			byService[c.After.Name] = c
			specs = append(specs, *c.After)
		case provider.ActionDelete:
			deletes = append(deletes, c)
		}
	}
	layers, err := Order(specs)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	secrets := newRedactor()

	for _, layer := range layers {
		var wg sync.WaitGroup
		errs := make([]error, len(layer))
		for i, spec := range layer {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c := byService[spec.Name]
				mu.Lock()
				resolved, rerr := resolve(*c.After, res.Outputs)
				mu.Unlock()
				if rerr != nil {
					errs[i] = &ApplyError{Service: spec.Name, Message: rerr.Error()}
					return
				}
				c.After = &resolved
				ref, out, aerr := applyOne(ctx, reg, leaser, c)
				mu.Lock()
				defer mu.Unlock()
				for _, v := range out {
					if v.Secret {
						secrets.add(v.Value)
					}
				}
				for _, v := range resolved.Env {
					if v.Secret {
						secrets.add(v.Value)
					}
				}
				if aerr != nil {
					errs[i] = &ApplyError{Service: spec.Name, Message: secrets.clean(aerr.Error()), cause: sentinel(aerr)}
					return
				}
				res.Refs[spec.Name] = ref
				res.Outputs[spec.Name] = out
			}()
		}
		wg.Wait()
		if err := firstError(layer, errs); err != nil {
			return res, err
		}
	}

	sort.Slice(deletes, func(i, j int) bool { return deletes[i].Service < deletes[j].Service })
	for _, c := range deletes {
		if err := destroyOne(ctx, reg, leaser, c); err != nil {
			return res, &ApplyError{Service: c.Service, Message: secrets.clean(err.Error()), cause: sentinel(err)}
		}
		res.Deleted = append(res.Deleted, c.Service)
	}
	return res, nil
}

func firstError(layer []provider.ServiceSpec, errs []error) error {
	for i := range layer {
		if errs[i] != nil {
			return errs[i]
		}
	}
	return nil
}

// sentinel keeps errors.Is working for the two provider sentinels without keeping the raw error.
func sentinel(err error) error {
	switch {
	case errors.Is(err, provider.ErrNotSupported):
		return provider.ErrNotSupported
	case errors.Is(err, provider.ErrNotImplemented):
		return provider.ErrNotImplemented
	}
	return nil
}

func applyOne(ctx context.Context, reg *provider.Registry, leaser Leaser, c provider.Change) (provider.Ref, provider.Outputs, error) {
	p, err := reg.Get(c.After.Provider)
	if err != nil {
		return provider.Ref{}, nil, err
	}
	lease, err := leaser.Lease(ctx, c.After.Provider)
	if err != nil {
		return provider.Ref{}, nil, fmt.Errorf("lease: %w", err)
	}
	slog.Info("applying change", "service", c.Service, "action", c.Action, "provider", c.After.Provider, "kind", c.After.Kind)
	return p.Apply(ctx, lease, c)
}

func destroyOne(ctx context.Context, reg *provider.Registry, leaser Leaser, c provider.Change) error {
	if c.After == nil || c.After.Provider == "" {
		return errors.New("delete change has no provider")
	}
	p, err := reg.Get(c.After.Provider)
	if err != nil {
		return err
	}
	lease, err := leaser.Lease(ctx, c.After.Provider)
	if err != nil {
		return fmt.Errorf("lease: %w", err)
	}
	slog.Info("destroying service", "service", c.Service, "provider", c.After.Provider)
	return p.Destroy(ctx, lease, provider.Ref{Provider: c.After.Provider, ExternalID: "", Tags: map[string]string{"yari:service": c.Service}})
}

// resolve returns a copy of the spec with FromOutput references replaced by output values.
func resolve(spec provider.ServiceSpec, outputs map[string]provider.Outputs) (provider.ServiceSpec, error) {
	out := spec
	out.Env = make(map[string]provider.EnvValue, len(spec.Env))
	for key, v := range spec.Env {
		if v.FromOutput != nil {
			src, ok := outputs[v.FromOutput.Service]
			if !ok {
				return out, fmt.Errorf("env %s: service %q has no outputs yet", key, v.FromOutput.Service)
			}
			o, ok := src[v.FromOutput.Key]
			if !ok {
				return out, fmt.Errorf("env %s: service %q has no output %q", key, v.FromOutput.Service, v.FromOutput.Key)
			}
			v = provider.EnvValue{Value: o.Value, Secret: v.Secret || o.Secret}
		}
		out.Env[key] = v
	}
	return out, nil
}

type redactor struct {
	mu   sync.Mutex
	vals map[string]bool
}

func newRedactor() *redactor { return &redactor{vals: map[string]bool{}} }

func (r *redactor) add(v string) {
	if v == "" {
		return
	}
	r.mu.Lock()
	r.vals[v] = true
	r.mu.Unlock()
}

// clean replaces every known secret value in s.
func (r *redactor) clean(s string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for v := range r.vals {
		s = strings.ReplaceAll(s, v, "[redacted]")
	}
	return s
}
