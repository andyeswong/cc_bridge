package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Host                string
	Port                string
	ClaudeBin           string
	ClaudeWorkdir       string
	ClaudeSkipPerms     bool
	ClaudeDefaultModel  string
	OllamaURL           string
	UsageDBPath         string
	LocalAuthKey        string
	MCPConfigPath       string
	MCPAlways           bool
	ProvidersConfigPath string
}

func Load() Config {
	godotenv.Load()
	return Config{
		Host:                envOr("HOST", "127.0.0.1"),
		Port:                envOr("PORT", "8080"),
		ClaudeBin:           envOr("CLAUDE_BIN", ""),
		ClaudeWorkdir:       envOr("CLAUDE_WORKDIR", ""),
		ClaudeSkipPerms:     boolEnv("CLAUDE_SKIP_PERMS"),
		ClaudeDefaultModel:  envOr("CLAUDE_DEFAULT_MODEL", "llama2"),
		OllamaURL:           envOr("OLLAMA_URL", "http://localhost:11434"),
		UsageDBPath:         envOr("USAGE_DB_PATH", "usage.db"),
		LocalAuthKey:        envOr("LOCAL_AUTH_KEY", ""),
		MCPConfigPath:       envOr("MCP_CONFIG_PATH", ""),
		MCPAlways:           boolEnv("MCP_ALWAYS"),
		ProvidersConfigPath: envOr("CCB_CONFIG_PATH", ""),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1"
}
