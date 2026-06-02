package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/uuid"

	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"claude-bridge/internal/sessions"
)

type Provider struct {
	cfg      config.Config
	sessions *sessions.MemoryStore[string]

	// Optional Anthropic backend override. When set, the Claude Code subprocess
	// is pointed at this base URL (e.g. an Ollama instance serving the Anthropic
	// Messages API) instead of the real Anthropic API — running the Claude Code
	// harness (bash, MCP, tool loop) on a different brain. Empty = real Anthropic.
	anthropicBaseURL   string
	anthropicAuthToken string
}

type Result struct {
	Type    string       `json:"type"`
	Subtype string       `json:"subtype"`
	IsError bool         `json:"is_error"`
	Result  string       `json:"result"`
	Usage   *UsageResult `json:"usage,omitempty"`
}

type UsageResult struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func NewProvider(cfg config.Config, sessionStore *sessions.MemoryStore[string]) *Provider {
	return &Provider{
		cfg:      cfg,
		sessions: sessionStore,
	}
}

// NewBackedProvider builds a Claude Code provider whose subprocess is pointed at
// an alternate Anthropic-compatible backend (base URL + auth token). This lets
// the Claude Code agentic harness (bash, MCP, tools) run on any brain that speaks
// the Anthropic Messages API — e.g. Ollama v0.20+ serving /v1/messages.
func NewBackedProvider(cfg config.Config, sessionStore *sessions.MemoryStore[string], baseURL, authToken string) *Provider {
	return &Provider{
		cfg:                cfg,
		sessions:           sessionStore,
		anthropicBaseURL:   baseURL,
		anthropicAuthToken: authToken,
	}
}

// processEnv returns the subprocess environment, injecting the Anthropic backend
// override when this provider is backed by a custom brain.
func (p *Provider) processEnv() []string {
	env := os.Environ()
	if p.anthropicBaseURL != "" {
		token := p.anthropicAuthToken
		if token == "" {
			token = "ollama"
		}
		env = append(env,
			"ANTHROPIC_BASE_URL="+p.anthropicBaseURL,
			"ANTHROPIC_AUTH_TOKEN="+token,
		)
	}
	return env
}

func (p *Provider) Chat(
	ctx context.Context,
	req domain.ChatRequest,
	clientSessionID string,
) (*domain.ChatResponse, *domain.Usage, error) {
	input, cleanup, err := p.prepareCLIInput(req, clientSessionID, false)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	text, usage, err := p.runSync(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	displayModel := req.Model
	if displayModel == "" {
		displayModel = domain.DefaultClaudeModel
	}

	openAIUsage := convertUsage(usage)

	resp := domain.NewChatResponse(
		displayModel,
		text,
		openAIUsage,
	)

	return &resp, openAIUsage, nil
}

func (p *Provider) prepareCLIInput(
	req domain.ChatRequest,
	clientSessionID string,
	stream bool,
) (cliArgs, func(), error) {
	cleanup := func() {}

	model := req.Model
	if model == "" {
		model = domain.DefaultClaudeModel
	}

	input := cliArgs{
		Model:        model,
		Stream:       stream,
		Workdir:      req.Workdir,
		AllowedTools: req.AllowedTools,
	}

	mcpPath, cleanupMCP, err := p.prepareMCPConfig(req)
	if err != nil {
		return cliArgs{}, cleanup, err
	}

	cleanup = cleanupMCP
	input.MCPConfigPath = mcpPath

	if clientSessionID == "" {
		input.Prompt = formatMessages(req.Messages)
		return input, cleanup, nil
	}

	if claudeSessionID, ok := p.sessions.Get(clientSessionID); ok {
		input.SessionID = claudeSessionID
		input.Prompt = lastUserMessage(req.Messages)
		return input, cleanup, nil
	}

	newClaudeSessionID := uuid.NewString()

	p.sessions.Set(clientSessionID, newClaudeSessionID)

	input.NewSessionID = newClaudeSessionID
	input.Prompt = formatMessages(req.Messages)

	return input, cleanup, nil
}

func (p *Provider) runSync(ctx context.Context, input cliArgs) (string, *UsageResult, error) {
	args := buildArgs(p.cfg, input)

	cmd := exec.CommandContext(ctx, binaryPath(p.cfg), args...)
	cmd.Env = p.processEnv()
	cmd.Stdin = strings.NewReader(input.Prompt)

	workdir := p.cfg.ClaudeWorkdir
	if input.Workdir != "" {
		workdir = input.Workdir
	}

	if workdir != "" {
		cmd.Dir = workdir
	}

	output, err := cmd.Output()
	if err != nil {
		var result Result

		if jsonErr := json.Unmarshal(bytes.TrimSpace(output), &result); jsonErr == nil && result.IsError {
			return "", nil, fmt.Errorf("claude error: %s", result.Result)
		}

		return "", nil, fmt.Errorf("claude exited: %w", err)
	}

	var result Result
	if err := json.Unmarshal(bytes.TrimSpace(output), &result); err != nil {
		return cleanText(string(output)), nil, nil
	}

	if result.IsError {
		return "", result.Usage, fmt.Errorf("claude error: %s", result.Result)
	}

	return result.Result, result.Usage, nil
}

func formatMessages(messages []domain.Message) string {
	var sb strings.Builder

	for _, message := range messages {
		switch message.Role {
		case "system":
			sb.WriteString("<system>\n")
			sb.WriteString(message.Content)
			sb.WriteString("\n</system>\n\n")

		case "user":
			sb.WriteString("Human: ")
			sb.WriteString(message.Content)
			sb.WriteString("\n\n")

		case "assistant":
			sb.WriteString("Assistant: ")
			sb.WriteString(message.Content)
			sb.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

func lastUserMessage(messages []domain.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			return messages[index].Content
		}
	}

	return ""
}

func convertUsage(usage *UsageResult) *domain.Usage {
	if usage == nil {
		return nil
	}

	return domain.NewUsageFromClaude(
		usage.InputTokens,
		usage.OutputTokens,
	)
}
