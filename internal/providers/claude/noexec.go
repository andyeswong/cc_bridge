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
		// own here — it must pick exactly one of these and let the parent run it.
		return "PLAN-ONLY MODE. You have NO tools of your own and you must NOT execute anything. " +
			"The parent system will run the tool you choose. The ONLY callable tools are: " +
			strings.Join(names, ", ") + ". " +
			"For the user's request, choose EXACTLY ONE of those tools and the arguments to call it with. " +
			"Do NOT mention, invent, or refer to any tool not in that list (you do NOT have bash/Glob/Grep/Read). " +
			`Respond with ONLY one compact JSON object on a single line, no markdown fences, no prose: {"name":"<tool from the list>","arguments":{...}} . ` +
			`If genuinely no tool applies, respond exactly {"name":"","answer":"<your text answer>"}.`
	}
	// No catalog provided: fall back to a generic shell intent.
	return "PLAN-ONLY MODE. You must NOT execute any tool or command yourself. " +
		"Decide the SINGLE shell command the PARENT system should run for the user's request. " +
		`Respond with ONLY one compact JSON object, no markdown, no prose: {"name":"bash","arguments":{"command":"<cmd>"}} . ` +
		`If no command is needed, respond exactly {"name":"","answer":"<your text answer>"}.`
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
