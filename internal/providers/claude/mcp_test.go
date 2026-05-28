package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"claude-bridge/internal/sessions"
)

func TestPrepareMCPConfig_InlineConfig_WritesTempAndCleansUp(t *testing.T) {
	p := NewProvider(config.Config{}, sessions.NewMemoryStore[string](time.Minute, 10))

	raw := json.RawMessage(`{"mcpServers":{"a":{"command":"x"}}}`)
	path, cleanup, err := p.prepareMCPConfig(domain.ChatRequest{MCPConfig: raw})
	if err != nil {
		t.Fatalf("prepareMCPConfig: %v", err)
	}
	if path == "" {
		t.Fatalf("expected temp path")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp: %v", err)
	}
	if string(content) != string(raw) {
		t.Fatalf("content mismatch: got %s", string(content))
	}

	cleanup()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected temp file to be removed")
	}
}

func TestPrepareMCPConfig_MCPServersFiltering(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "mcp_registry.json")
	if err := os.WriteFile(registryPath, []byte(`{"mcpServers":{"a":{"command":"x"},"b":{"command":"y"}}}`), 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	p := NewProvider(config.Config{MCPConfigPath: registryPath}, sessions.NewMemoryStore[string](time.Minute, 10))

	path, cleanup, err := p.prepareMCPConfig(domain.ChatRequest{MCPServers: []string{"b"}})
	if err != nil {
		t.Fatalf("prepareMCPConfig: %v", err)
	}
	t.Cleanup(cleanup)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read filtered: %v", err)
	}

	var got struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal filtered: %v", err)
	}
	if len(got.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(got.MCPServers))
	}
	if _, ok := got.MCPServers["b"]; !ok {
		t.Fatalf("expected server b to be present")
	}
	if _, ok := got.MCPServers["a"]; ok {
		t.Fatalf("did not expect server a to be present")
	}
}

func TestPrepareMCPConfig_MissingServerReturnsError(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "mcp_registry.json")
	if err := os.WriteFile(registryPath, []byte(`{"mcpServers":{"a":{"command":"x"}}}`), 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	p := NewProvider(config.Config{MCPConfigPath: registryPath}, sessions.NewMemoryStore[string](time.Minute, 10))

	_, _, err := p.prepareMCPConfig(domain.ChatRequest{MCPServers: []string{"missing"}})
	if err == nil {
		t.Fatalf("expected error for missing server")
	}
}

func TestPrepareMCPConfig_MCPAlwaysUsesRegistryPath(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "mcp_registry.json")
	if err := os.WriteFile(registryPath, []byte(`{"mcpServers":{"a":{"command":"x"}}}`), 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	p := NewProvider(
		config.Config{MCPConfigPath: registryPath, MCPAlways: true},
		sessions.NewMemoryStore[string](time.Minute, 10),
	)

	path, cleanup, err := p.prepareMCPConfig(domain.ChatRequest{})
	if err != nil {
		t.Fatalf("prepareMCPConfig: %v", err)
	}
	if path != registryPath {
		t.Fatalf("path: got %q want %q", path, registryPath)
	}

	cleanup()
	if _, err := os.Stat(registryPath); err != nil {
		t.Fatalf("expected registry file to remain: %v", err)
	}
}
