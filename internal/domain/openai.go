package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	var raw struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  []ToolCall      `json:"tool_calls"`
		ToolCallID string          `json:"tool_call_id"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	m.Role = raw.Role
	m.Content = flattenContent(raw.Content)
	m.ToolCalls = raw.ToolCalls
	m.ToolCallID = raw.ToolCallID

	return nil
}

func flattenContent(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}

	var parts []contentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var sb strings.Builder

		for _, part := range parts {
			if part.Type != "" && part.Type != "text" {
				continue
			}

			if part.Text == "" {
				continue
			}

			if sb.Len() > 0 {
				sb.WriteString("\n")
			}

			sb.WriteString(part.Text)
		}

		return sb.String()
	}

	return ""
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`

	// OpenAI-compatible tool use
	Tools      []Tool          `json:"tools,omitempty"`
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`

	// Claude Code extensions
	AllowedTools []string        `json:"allowed_tools,omitempty"`
	MCPServers   []string        `json:"mcp_servers,omitempty"`
	MCPConfig    json.RawMessage `json:"mcp_config,omitempty"`
	Workdir      string          `json:"workdir,omitempty"`
}

type DeltaToolCall struct {
	Index    int              `json:"index,omitempty"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function,omitempty"`
}

type Delta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []DeltaToolCall `json:"tool_calls,omitempty"`
}

type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	Delta        *Delta   `json:"delta,omitempty"`
	FinishReason *string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

func NewChatResponse(model string, content string, usage *Usage) ChatResponse {
	stop := "stop"

	return ChatResponse{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: &Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: &stop,
			},
		},
		Usage: usage,
	}
}

func NewStreamChunk(id string, model string, role string, content string, finishReason *string) ChatResponse {
	delta := &Delta{}

	if role != "" {
		delta.Role = role
	}

	if content != "" {
		delta.Content = content
	}

	return ChatResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	}
}

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}
