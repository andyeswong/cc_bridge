package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ProviderConfig struct {
	Name       string   `json:"name"`
	APIBaseURL string   `json:"api_base_url"`
	APIKey     string   `json:"api_key"`
	Models     []string `json:"models"`

	// Driver selects how requests to this provider are executed:
	//   "" / "openai" — generic OpenAI-compatible HTTP passthrough (the model is the brain)
	//   "claude"      — run the Claude Code CLI (full agentic harness: bash, MCP, tools)
	//                   with ANTHROPIC_BASE_URL pointed at api_base_url, so Claude Code
	//                   acts as the body and the configured model is the brain.
	Driver string `json:"driver,omitempty"`
}

type RouterConfig struct {
	Default    string `json:"default"`
	Background string `json:"background"`
	Fallback   string `json:"fallback"`
}

type BridgeConfig struct {
	Providers []ProviderConfig `json:"providers"`
	Router    RouterConfig     `json:"router"`
}

func (c *BridgeConfig) CountProviders() int {
	if c == nil {
		return 0
	}
	return len(c.Providers)
}

func LoadBridgeConfig(path string) (*BridgeConfig, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil
		}
		path = filepath.Join(home, ".cc_bridge", "config.json")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg BridgeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
