package provider

import (
	"fmt"
	"sync"
)

// DocsResolver tells the registry where a catalog entry documents its API.
// catalog.Catalog implements it; the interface keeps this package free of a catalog import.
type DocsResolver interface {
	// DocsURL returns the official API docs URL of a catalog entry, and whether the id exists.
	DocsURL(id string) (string, bool)
}

// Registry maps Provider Catalog ids to adapter implementations.
// An id that is in the catalog but has no implementation yet is never a silent no-op:
// Get returns ErrNotSupported wrapped with the docs URL.
type Registry struct {
	docs DocsResolver

	mu    sync.RWMutex
	impls map[string]Provider
}

// NewRegistry returns an empty registry over a catalog.
func NewRegistry(docs DocsResolver) *Registry {
	return &Registry{docs: docs, impls: map[string]Provider{}}
}

// Register installs an implementation for a catalog id. The id must exist in the catalog.
func (r *Registry) Register(id string, p Provider) error {
	if _, ok := r.docs.DocsURL(id); !ok {
		return fmt.Errorf("provider: %q is not in the Provider Catalog", id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.impls[id]; dup {
		return fmt.Errorf("provider: %q is already registered", id)
	}
	r.impls[id] = p
	return nil
}

// Get returns the implementation for a catalog id.
//
// For an id that is in the catalog without an adapter it returns an error that wraps
// ErrNotSupported and names the provider's docs URL. For an unknown id it returns a plain error.
func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	p, ok := r.impls[id]
	r.mu.RUnlock()
	if ok {
		return p, nil
	}
	docs, known := r.docs.DocsURL(id)
	if !known {
		return nil, fmt.Errorf("provider: unknown provider %q", id)
	}
	return nil, fmt.Errorf("%w: %s has no adapter yet, see %s", ErrNotSupported, id, docs)
}

// Registered lists the ids that have an implementation.
func (r *Registry) Registered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.impls))
	for id := range r.impls {
		out = append(out, id)
	}
	return out
}
