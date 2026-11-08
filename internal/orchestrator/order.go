// Package orchestrator orders and applies the changes of a Plan across providers.
package orchestrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/synapsbranch-ux/yari.run/internal/provider"
)

// Order groups specs into dependency layers using the FromOutput references of their env values.
// Every spec in a layer depends only on specs in earlier layers, so a layer can be applied
// together. Layers and the specs inside them are sorted by name, so the result is deterministic.
// It returns an error for duplicate names, references to unknown services and cycles.
func Order(specs []provider.ServiceSpec) ([][]provider.ServiceSpec, error) {
	byName := make(map[string]provider.ServiceSpec, len(specs))
	for _, s := range specs {
		if s.Name == "" {
			return nil, fmt.Errorf("orchestrator: a service has no name")
		}
		if _, dup := byName[s.Name]; dup {
			return nil, fmt.Errorf("orchestrator: duplicate service %q", s.Name)
		}
		byName[s.Name] = s
	}

	deps := make(map[string]map[string]bool, len(specs))
	for _, s := range specs {
		deps[s.Name] = map[string]bool{}
		for key, v := range s.Env {
			if v.FromOutput == nil {
				continue
			}
			ref := v.FromOutput.Service
			if _, ok := byName[ref]; !ok {
				return nil, fmt.Errorf("orchestrator: service %q env %q references unknown service %q", s.Name, key, ref)
			}
			deps[s.Name][ref] = true
		}
	}

	var layers [][]provider.ServiceSpec
	placed := map[string]bool{}
	for len(placed) < len(specs) {
		var layer []string
		for name := range byName {
			if placed[name] {
				continue
			}
			ready := true
			for d := range deps[name] {
				if !placed[d] {
					ready = false
					break
				}
			}
			if ready {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			var stuck []string
			for name := range byName {
				if !placed[name] {
					stuck = append(stuck, name)
				}
			}
			sort.Strings(stuck)
			return nil, fmt.Errorf("orchestrator: dependency cycle among services: %s", strings.Join(stuck, ", "))
		}
		sort.Strings(layer)
		out := make([]provider.ServiceSpec, 0, len(layer))
		for _, name := range layer {
			placed[name] = true
			out = append(out, byName[name])
		}
		layers = append(layers, out)
	}
	return layers, nil
}
