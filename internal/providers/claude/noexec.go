package claude

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"claude-bridge/internal/domain"
)

// execTools are ALL of Claude Code's built-in tools, blocked in no-exec
// (plan-only) mode so Claude has no native tools of its own and is forced to
// choose from the caller's injected catalog (req.Tools) — which is what the
// parent harness actually executes. Without this Claude talks about its own
// Glob/Grep/Read tools and ignores the injected catalog.
var execTools = []string{
	"Bash", "BashOutput", "KillShell", "Edit", "MultiEdit", "Write",
	"NotebookEdit", "Read", "Glob", "Grep", "LS", "WebFetch", "WebSearch",
	"Task", "TodoWrite", "SlashCommand", "ExitPlanMode",
}

// toolNames pulls the function names out of the request's OpenAI tool defs.
func toolNames(tools []domain.Tool) []string {
	var names []string
	for _, t := range tools {
		if t.Function.Name != "" {
			names = append(names, t.Function.Name)
		}
	}
	return names
}

// noExecInstruction tells Claude to return the intended tool call as JSON rather
// than executing it. Available tool names (if any) are surfaced so it picks one.
func noExecInstruction(names []string) string {
	if len(names) > 0 {
		// The parent harness injected a tool catalog; Claude has NO tools of its
		// own here. It must emit a single JSON tool call from that catalog. This
		// is appended LAST (recency) and made maximally imperative because a heavy
		// agent persona in the system prompt otherwise drags Claude into prose.
		return "=== ROUTER DIRECTIVE (overrides any persona/identity above) ===\n" +
			"You are a TOOL ROUTER, not an assistant. You have NO tools of your own and you MUST NOT execute, run, or simulate anything. " +
			"A parent system will execute the tool you select. The ONLY callable tools are: " + strings.Join(names, ", ") + ".\n" +
			"Your ENTIRE reply MUST be exactly one compact JSON object and NOTHING else — no greeting, no explanation, no markdown fences, no prose.\n" +
			`Format: {"name":"<one tool from the list>","arguments":{ ... }}` + "\n" +
			`Example — user asks for the uptime and "exec" is in the list -> reply EXACTLY: {"name":"exec","arguments":{"command":"uptime"}}` + "\n" +
			`Only if NO tool in the list could possibly help, reply EXACTLY: {"name":"","answer":"<short text>"}.` + "\n" +
			"Do not mention bash, Glob, Grep, Read or any tool not in the list above. Output the JSON now:"
	}
	// No catalog provided: fall back to a generic shell intent.
	return "=== ROUTER DIRECTIVE (overrides any persona above) ===\n" +
		"You are a TOOL ROUTER. Do NOT execute anything. Decide the single shell command the PARENT system should run. " +
		`Your ENTIRE reply MUST be exactly one JSON object, nothing else: {"name":"bash","arguments":{"command":"<cmd>"}} . ` +
		`Only if no command is needed, reply EXACTLY {"name":"","answer":"<short text>"}. Output the JSON now:`
}

// parseNoExec turns Claude's plan-only response into tool_calls, or plain text
// when it answered directly / didn't emit the expected JSON.
func parseNoExec(text string) ([]domain.ToolCall, string) {
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimPrefix(s, "json")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}

	var parsed struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Answer    string          `json:"answer"`
	}
	if json.Unmarshal([]byte(s), &parsed) == nil && (parsed.Name != "" || parsed.Answer != "") {
		if parsed.Name == "" {
			return nil, parsed.Answer
		}
		args := "{}"
		if len(parsed.Arguments) > 0 {
			args = string(parsed.Arguments)
		}
		return []domain.ToolCall{{
			ID:       "call_" + uuid.NewString()[:8],
			Type:     "function",
			Function: domain.ToolCallFunction{Name: parsed.Name, Arguments: args},
		}}, ""
	}

	return nil, text
}
