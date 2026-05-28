package claude

import (
	"encoding/json"
	"fmt"
	"os"

	"claude-bridge/internal/domain"
)

func (p *Provider) prepareMCPConfig(req domain.ChatRequest) (string, func(), error) {
	noop := func() {}

	if len(req.MCPConfig) > 0 {
		return writeTempMCP(req.MCPConfig)
	}

	if len(req.MCPServers) > 0 {
		if p.cfg.MCPConfigPath == "" {
			return "", noop, fmt.Errorf("mcp_servers requested but MCP_CONFIG_PATH is not set")
		}

		raw, err := os.ReadFile(p.cfg.MCPConfigPath)
		if err != nil {
			return "", noop, fmt.Errorf("read mcp registry: %w", err)
		}

		var registry struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		}

		if err := json.Unmarshal(raw, &registry); err != nil {
			return "", noop, fmt.Errorf("parse mcp registry: %w", err)
		}

		filtered := map[string]json.RawMessage{}

		for _, serverName := range req.MCPServers {
			serverConfig, ok := registry.MCPServers[serverName]
			if !ok {
				return "", noop, fmt.Errorf("mcp server %q not found in registry", serverName)
			}

			filtered[serverName] = serverConfig
		}

		output, err := json.Marshal(map[string]any{
			"mcpServers": filtered,
		})
		if err != nil {
			return "", noop, fmt.Errorf("marshal filtered mcp config: %w", err)
		}

		return writeTempMCP(output)
	}

	if p.cfg.MCPAlways && p.cfg.MCPConfigPath != "" {
		return p.cfg.MCPConfigPath, noop, nil
	}

	return "", noop, nil
}

func writeTempMCP(content []byte) (string, func(), error) {
	file, err := os.CreateTemp("", "claude_bridge_mcp_*.json")
	if err != nil {
		return "", func() {}, err
	}

	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}

	cleanup := func() {
		_ = os.Remove(file.Name())
	}

	return file.Name(), cleanup, nil
}
