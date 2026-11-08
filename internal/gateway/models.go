// Package gateway talks to the LiteLLM Proxy that fronts every model call of an Agent Session.
package gateway

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// Model is a model alias the gateway allows.
type Model struct {
	Alias       string
	DisplayName string
	Provider    string
}

// ConfigModels lists the model aliases allowed by the gateway from agent/litellm/config.yaml,
// the file the gateway itself runs on. The result is cached until the file changes or the
// TTL passes, so ListModels needs no running gateway.
type ConfigModels struct {
	Path string
	TTL  time.Duration

	mu      sync.Mutex
	cached  []Model
	mtime   time.Time
	fetched time.Time
}

type litellmConfig struct {
	ModelList []struct {
		ModelName     string `yaml:"model_name"`
		LiteLLMParams struct {
			Model string `yaml:"model"`
		} `yaml:"litellm_params"`
		ModelInfo struct {
			DisplayName string `yaml:"display_name"`
		} `yaml:"model_info"`
	} `yaml:"model_list"`
}

// Models implements ModelLister.
func (c *ConfigModels) Models(_ context.Context) ([]Model, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, err := os.Stat(c.Path)
	if err != nil {
		return nil, fmt.Errorf("gateway: model config: %w", err)
	}
	ttl := c.TTL
	if ttl == 0 {
		ttl = time.Minute
	}
	if c.cached != nil && st.ModTime().Equal(c.mtime) && time.Since(c.fetched) < ttl {
		return append([]Model(nil), c.cached...), nil
	}
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return nil, fmt.Errorf("gateway: model config: %w", err)
	}
	models, err := ParseModels(data)
	if err != nil {
		return nil, err
	}
	c.cached, c.mtime, c.fetched = models, st.ModTime(), time.Now()
	return append([]Model(nil), models...), nil
}

// ParseModels extracts the allowed model aliases from a LiteLLM proxy config.
func ParseModels(data []byte) ([]Model, error) {
	var cfg litellmConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("gateway: parse model config: %w", err)
	}
	seen := map[string]bool{}
	var out []Model
	for _, m := range cfg.ModelList {
		if m.ModelName == "" || seen[m.ModelName] {
			continue
		}
		seen[m.ModelName] = true
		provider, _, _ := strings.Cut(m.LiteLLMParams.Model, "/")
		name := m.ModelInfo.DisplayName
		if name == "" {
			name = m.ModelName
		}
		out = append(out, Model{Alias: m.ModelName, DisplayName: name, Provider: provider})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out, nil
}
