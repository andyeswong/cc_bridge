package claude

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"claude-bridge/internal/domain"
)

// execTools are the side-effecting tools blocked in no-exec (plan-only) mode.
var execTools = []string{"Bash", "Edit", "Write", "NotebookEdit", "WebFetch", "WebSearch", "Task"}

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
	avail := `If the task needs a shell command, use {"name":"bash","arguments":{"command":"<cmd>"}}.`
	if len(names) > 0 {
		avail = "Available tools you may call (choose exactly one): " + strings.Join(names, ", ") + "."
	}
	return "PLAN-ONLY MODE. You must NOT execute any tool, command, or file operation yourself — tools are disabled. " +
		"Decide the SINGLE next tool call the PARENT system should run for the user's request. " + avail +
		` Respond with ONLY one compact JSON object on a single line, no markdown fences, no prose: {"name":"<tool>","arguments":{...}} . ` +
		`If no tool is needed, respond exactly {"name":"","answer":"<your text answer>"}.`
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
