package claude

import (
	"testing"

	"claude-bridge/internal/config"
)

func TestBuildArgs_DoesNotIncludePrompt(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	input := cliArgs{
		Prompt: "SECRET PROMPT",
		Model:  "claude-code",
	}

	args := buildArgs(cfg, input)
	for _, a := range args {
		if a == input.Prompt {
			t.Fatalf("prompt should not be added to args")
		}
	}
}

func TestBuildArgs_StreamAndNonStream(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}

	streamArgs := buildArgs(cfg, cliArgs{Stream: true, Model: "claude-code"})
	if !hasArg(streamArgs, "stream-json") || !hasArg(streamArgs, "--verbose") {
		t.Fatalf("expected stream-json and --verbose, got %#v", streamArgs)
	}

	nonStreamArgs := buildArgs(cfg, cliArgs{Stream: false, Model: "claude-code"})
	if !hasArg(nonStreamArgs, "json") || hasArg(nonStreamArgs, "--verbose") || hasArg(nonStreamArgs, "stream-json") {
		t.Fatalf("expected json without streaming flags, got %#v", nonStreamArgs)
	}
}

func TestBuildArgs_ModelFlag(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}

	args := buildArgs(cfg, cliArgs{Model: "claude-code"})
	if hasArg(args, "--model") {
		t.Fatalf("did not expect --model for claude-code: %#v", args)
	}

	args = buildArgs(cfg, cliArgs{Model: "claude-sonnet-4-5"})
	if !hasArg(args, "--model") || !hasArg(args, "claude-sonnet-4-5") {
		t.Fatalf("expected --model for non-claude-code: %#v", args)
	}
}

func TestBuildArgs_SessionFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}

	args := buildArgs(cfg, cliArgs{SessionID: "sess"})
	if !hasArg(args, "--resume") || !hasArg(args, "sess") {
		t.Fatalf("expected --resume sess: %#v", args)
	}
	if hasArg(args, "--session-id") {
		t.Fatalf("did not expect --session-id when --resume is set: %#v", args)
	}

	args = buildArgs(cfg, cliArgs{NewSessionID: "new"})
	if !hasArg(args, "--session-id") || !hasArg(args, "new") {
		t.Fatalf("expected --session-id new: %#v", args)
	}
	if hasArg(args, "--resume") {
		t.Fatalf("did not expect --resume when --session-id is set: %#v", args)
	}
}

func TestBuildArgs_SkipPermsAndMCPAndAllowedTools(t *testing.T) {
	t.Parallel()

	cfg := config.Config{ClaudeSkipPerms: true}
	args := buildArgs(cfg, cliArgs{
		Model:         "claude-code",
		MCPConfigPath: "/tmp/mcp.json",
		AllowedTools:  []string{"read_file", "run_command"},
	})

	if !hasArg(args, "--dangerously-skip-permissions") {
		t.Fatalf("expected skip perms flag: %#v", args)
	}
	if !hasArg(args, "--mcp-config") || !hasArg(args, "/tmp/mcp.json") || !hasArg(args, "--strict-mcp-config") {
		t.Fatalf("expected mcp flags: %#v", args)
	}
	if !hasArg(args, "--allowedTools") || !hasArg(args, "read_file,run_command") {
		t.Fatalf("expected allowed tools: %#v", args)
	}

	cfg = config.Config{ClaudeSkipPerms: false}
	args = buildArgs(cfg, cliArgs{Model: "claude-code"})
	if hasArg(args, "--dangerously-skip-permissions") {
		t.Fatalf("did not expect skip perms flag: %#v", args)
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
