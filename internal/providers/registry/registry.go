package registry

import (
	"strings"

	"claude-bridge/internal/config"
)

type ExternalProvider struct {
	Name    string
	BaseURL string
	APIKey  string
	Models  []string
}

type RouteMatch struct {
	Provider    *ExternalProvider
	TargetModel string
}

type Registry struct {
	providers map[string]*ExternalProvider
}

func New(cfg *config.BridgeConfig) *Registry {
	r := &Registry{
		providers: make(map[string]*ExternalProvider),
	}

	if cfg == nil {
		return r
	}

	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if p.Name == "" || p.APIBaseURL == "" {
			continue
		}
		r.providers[p.Name] = &ExternalProvider{
			Name:    p.Name,
			BaseURL: p.APIBaseURL,
			APIKey:  p.APIKey,
			Models:  p.Models,
		}
	}

	return r
}

func (r *Registry) Empty() bool {
	return len(r.providers) == 0
}

func (r *Registry) Providers() []*ExternalProvider {
	list := make([]*ExternalProvider, 0, len(r.providers))
	for _, p := range r.providers {
		list = append(list, p)
	}
	return list
}

// Resolve finds an external provider for the given model string.
// Supports three formats:
//   - "provider,model"  — explicit provider selection (e.g. "ollama,qwen3-coder:latest")
//   - "provider/model"  — alternate separator (e.g. "ollama/qwen3-coder:latest")
//   - "model"           — searches all providers by declared models list
//
// Returns nil if no match; caller should fall back to the default executor.
func (r *Registry) Resolve(model string) *RouteMatch {
	// "provider,model"
	if idx := strings.Index(model, ","); idx > 0 {
		name := model[:idx]
		target := model[idx+1:]
		if p := r.providers[name]; p != nil {
			return &RouteMatch{Provider: p, TargetModel: target}
		}
	}

	// "provider/model" — only when prefix matches a registered provider name
	if idx := strings.Index(model, "/"); idx > 0 {
		name := model[:idx]
		target := model[idx+1:]
		if p := r.providers[name]; p != nil {
			return &RouteMatch{Provider: p, TargetModel: target}
		}
	}

	// bare model name — search declared models lists
	for _, p := range r.providers {
		for _, m := range p.Models {
			if m == model {
				return &RouteMatch{Provider: p, TargetModel: model}
			}
		}
	}

	return nil
}
