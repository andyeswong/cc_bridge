package registry

import (
	"testing"

	"claude-bridge/internal/config"
)

func testRegistry() *Registry {
	return New(&config.BridgeConfig{
		Providers: []config.ProviderConfig{
			{
				Name:       "ollama",
				APIBaseURL: "http://localhost:11434/v1",
				APIKey:     "ollama",
				Models:     []string{"qwen3-coder:latest"},
			},
			{
				Name:       "claude-minimax",
				APIBaseURL: "http://localhost:11434",
				APIKey:     "ollama",
				Models:     []string{"minimax-m2.7:cloud"},
				Driver:     "claude",
			},
		},
	})
}

func TestResolve_ProviderCommaModel(t *testing.T) {
	t.Parallel()
	r := testRegistry()

	m := r.Resolve("ollama,qwen3-coder:latest")
	if m == nil {
		t.Fatal("expected match for ollama,qwen3-coder:latest")
	}
	if m.Provider.Name != "ollama" || m.TargetModel != "qwen3-coder:latest" {
		t.Fatalf("got provider=%q target=%q", m.Provider.Name, m.TargetModel)
	}
	if m.Provider.Driver != "" {
		t.Fatalf("ollama driver: got %q want empty", m.Provider.Driver)
	}
}

func TestResolve_ClaudeDriver(t *testing.T) {
	t.Parallel()
	r := testRegistry()

	m := r.Resolve("claude-minimax,minimax-m2.7:cloud")
	if m == nil {
		t.Fatal("expected match for claude-minimax,minimax-m2.7:cloud")
	}
	if m.Provider.Driver != "claude" {
		t.Fatalf("driver: got %q want claude", m.Provider.Driver)
	}
	if m.TargetModel != "minimax-m2.7:cloud" {
		t.Fatalf("target: got %q", m.TargetModel)
	}
	if m.Provider.BaseURL != "http://localhost:11434" {
		t.Fatalf("base url: got %q", m.Provider.BaseURL)
	}
}

func TestResolve_SlashSeparator(t *testing.T) {
	t.Parallel()
	r := testRegistry()

	m := r.Resolve("ollama/qwen3-coder:latest")
	if m == nil || m.Provider.Name != "ollama" || m.TargetModel != "qwen3-coder:latest" {
		t.Fatalf("slash separator resolve failed: %#v", m)
	}
}

func TestResolve_BareModelName(t *testing.T) {
	t.Parallel()
	r := testRegistry()

	m := r.Resolve("minimax-m2.7:cloud")
	if m == nil || m.Provider.Name != "claude-minimax" {
		t.Fatalf("bare model resolve failed: %#v", m)
	}
}

func TestResolve_NoMatchFallsThrough(t *testing.T) {
	t.Parallel()
	r := testRegistry()

	if m := r.Resolve("claude-code"); m != nil {
		t.Fatalf("expected nil (Claude fallback) for claude-code, got %#v", m)
	}
	if m := r.Resolve("unknown,model"); m != nil {
		t.Fatalf("expected nil for unknown provider, got %#v", m)
	}
}

func TestEmpty(t *testing.T) {
	t.Parallel()

	if !New(nil).Empty() {
		t.Fatal("nil config should produce empty registry")
	}
	if testRegistry().Empty() {
		t.Fatal("populated registry should not be empty")
	}
}
