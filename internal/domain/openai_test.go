package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageUnmarshalJSON_ContentVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantRole    string
		wantContent string
	}{
		{
			name:        "PlainString",
			input:       `{"role":"user","content":"hello"}`,
			wantRole:    "user",
			wantContent: "hello",
		},
		{
			name:        "TextBlocksJoinedWithNewlines",
			input:       `{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`,
			wantRole:    "user",
			wantContent: "a\nb",
		},
		{
			name:        "UnsupportedBlockTypesIgnored",
			input:       `{"role":"user","content":[{"type":"image","text":"nope"},{"type":"text","text":"ok"}]}`,
			wantRole:    "user",
			wantContent: "ok",
		},
		{
			name:        "NullContentBecomesEmpty",
			input:       `{"role":"user","content":null}`,
			wantRole:    "user",
			wantContent: "",
		},
		{
			name:        "UnsupportedContentBecomesEmpty",
			input:       `{"role":"user","content":123}`,
			wantRole:    "user",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg Message
			if err := json.Unmarshal([]byte(tt.input), &msg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if msg.Role != tt.wantRole {
				t.Fatalf("role: got %q want %q", msg.Role, tt.wantRole)
			}
			if msg.Content != tt.wantContent {
				t.Fatalf("content: got %q want %q", msg.Content, tt.wantContent)
			}
		})
	}
}

func TestNewChatResponse_SetsOpenAIFields(t *testing.T) {
	t.Parallel()

	usage := &Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}
	resp := NewChatResponse("claude-code", "hi", usage)

	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Fatalf("id: got %q", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Fatalf("object: got %q", resp.Object)
	}
	if resp.Created <= 0 {
		t.Fatalf("created: got %d", resp.Created)
	}
	if resp.Model != "claude-code" {
		t.Fatalf("model: got %q", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices: got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message == nil {
		t.Fatalf("message: nil")
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Fatalf("role: got %q", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].Message.Content != "hi" {
		t.Fatalf("content: got %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason: got %#v", resp.Choices[0].FinishReason)
	}
	if resp.Usage != usage {
		t.Fatalf("usage: expected same pointer")
	}
}

func TestNewStreamChunk_DeltaFields(t *testing.T) {
	t.Parallel()

	t.Run("RoleAndContent", func(t *testing.T) {
		chunk := NewStreamChunk("id", "m", "assistant", "hello", nil)
		if chunk.Object != "chat.completion.chunk" {
			t.Fatalf("object: got %q", chunk.Object)
		}
		if chunk.ID != "id" || chunk.Model != "m" {
			t.Fatalf("id/model: got %q/%q", chunk.ID, chunk.Model)
		}
		if len(chunk.Choices) != 1 || chunk.Choices[0].Delta == nil {
			t.Fatalf("delta missing")
		}
		if chunk.Choices[0].Delta.Role != "assistant" {
			t.Fatalf("role: got %q", chunk.Choices[0].Delta.Role)
		}
		if chunk.Choices[0].Delta.Content != "hello" {
			t.Fatalf("content: got %q", chunk.Choices[0].Delta.Content)
		}
	})

	t.Run("RoleOnly", func(t *testing.T) {
		chunk := NewStreamChunk("id", "m", "assistant", "", nil)
		if chunk.Choices[0].Delta.Role != "assistant" {
			t.Fatalf("role: got %q", chunk.Choices[0].Delta.Role)
		}
		if chunk.Choices[0].Delta.Content != "" {
			t.Fatalf("content: got %q", chunk.Choices[0].Delta.Content)
		}
	})

	t.Run("ContentOnly", func(t *testing.T) {
		chunk := NewStreamChunk("id", "m", "", "x", nil)
		if chunk.Choices[0].Delta.Role != "" {
			t.Fatalf("role: got %q", chunk.Choices[0].Delta.Role)
		}
		if chunk.Choices[0].Delta.Content != "x" {
			t.Fatalf("content: got %q", chunk.Choices[0].Delta.Content)
		}
	})

	t.Run("Neither", func(t *testing.T) {
		chunk := NewStreamChunk("id", "m", "", "", nil)
		if chunk.Choices[0].Delta.Role != "" || chunk.Choices[0].Delta.Content != "" {
			t.Fatalf("delta: got %#v", chunk.Choices[0].Delta)
		}
	})
}

func TestProviderRouting_OllamaPrefix(t *testing.T) {
	t.Parallel()

	if got := ResolveProvider("ollama/llama3"); got != ProviderOllama {
		t.Fatalf("ResolveProvider: got %q want %q", got, ProviderOllama)
	}
	if got := ResolveProvider("claude-code"); got != ProviderClaude {
		t.Fatalf("ResolveProvider: got %q want %q", got, ProviderClaude)
	}
	if got := StripOllamaPrefix("ollama/llama3"); got != "llama3" {
		t.Fatalf("StripOllamaPrefix: got %q", got)
	}
}
