package claude

import (
	"os/exec"
	"strings"

	"claude-bridge/internal/config"
)

type cliArgs struct {
	Prompt        string
	Model         string
	SessionID     string
	NewSessionID  string
	Stream        bool
	MCPConfigPath string
	AllowedTools  []string
	Workdir       string
	NoExec        bool // plan-only: block tool execution at the CLI level
}

func binaryPath(cfg config.Config) string {
	if cfg.ClaudeBin != "" {
		return cfg.ClaudeBin
	}

	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}

	return "claude"
}

func buildArgs(cfg config.Config, input cliArgs) []string {
	args := []string{"-p", "--output-format"}

	if input.Stream {
		args = append(args, "stream-json", "--verbose")
	} else {
		args = append(args, "json")
	}

	if input.Model != "" && input.Model != "claude-code" {
		args = append(args, "--model", input.Model)
	}

	if input.SessionID != "" {
		args = append(args, "--resume", input.SessionID)
	} else if input.NewSessionID != "" {
		args = append(args, "--session-id", input.NewSessionID)
	}

	if cfg.ClaudeSkipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}

	if input.MCPConfigPath != "" {
		args = append(args, "--mcp-config", input.MCPConfigPath, "--strict-mcp-config")
	}

	if len(input.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(input.AllowedTools, ","))
	}

	if input.NoExec {
		// Hard block: Claude physically cannot run these, so even a non-compliant
		// response cannot cause side effects.
		args = append(args, "--disallowedTools", strings.Join(execTools, ","))
	}

	return args
}

func cleanText(value string) string {
	return strings.TrimSpace(value)
}
