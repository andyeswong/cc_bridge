package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"claude-bridge/internal/sessions"
)

type flushBuffer struct {
	bytes.Buffer
}

func (f *flushBuffer) Flush() {}

func TestRunStream_RewritesIDAndFillsModelAndCapturesUsage(t *testing.T) {
	stop := "stop"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		first := domain.ChatResponse{
			ID:      "orig",
			Object:  "chat.completion.chunk",
			Created: 1,
			Model:   "",
			Choices: []domain.Choice{
				{
					Index: 0,
					Delta: &domain.Delta{Content: "hi"},
				},
			},
		}
		second := domain.ChatResponse{
			ID:      "orig",
			Object:  "chat.completion.chunk",
			Created: 2,
			Model:   "",
			Choices: []domain.Choice{
				{
					Index: 0,
					Delta: &domain.Delta{Content: " there"},
				},
			},
			Usage: &domain.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
		}
		third := domain.ChatResponse{
			ID:      "orig",
			Object:  "chat.completion.chunk",
			Created: 3,
			Model:   "",
			Choices: []domain.Choice{
				{
					Index:        0,
					Delta:        &domain.Delta{},
					FinishReason: &stop,
				},
			},
		}

		for _, c := range []domain.ChatResponse{first, second, third} {
			raw, _ := json.Marshal(c)
			_, _ = w.Write([]byte("data: " + string(raw) + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}

		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	store := sessions.NewMemoryStore[[]domain.Message](time.Minute, 10)
	p := NewProvider(config.Config{OllamaURL: srv.URL}, store)
	p.httpClient = srv.Client()
	p.maxHistory = 10

	var out flushBuffer

	assistantText, usage, err := p.runStream(
		context.Background(),
		&out,
		"llama3",
		[]domain.Message{{Role: "user", Content: "hi"}},
		"chatcmpl-test",
		"ollama/llama3",
	)
	if err != nil {
		t.Fatalf("runStream: %v", err)
	}
	if assistantText != "hi there" {
		t.Fatalf("assistantText: got %q", assistantText)
	}
	if usage == nil || usage.TotalTokens != 3 {
		t.Fatalf("usage: %#v", usage)
	}

	lines := strings.Split(out.String(), "\n")
	var seenRewritten bool
	var seenDone bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "data: [DONE]" {
			seenDone = true
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		raw := strings.TrimPrefix(line, "data: ")
		var chunk domain.ChatResponse
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			continue
		}
		if chunk.ID == "chatcmpl-test" && chunk.Model == "ollama/llama3" {
			seenRewritten = true
			continue
		}
	}

	if !seenRewritten {
		t.Fatalf("expected output chunks to have rewritten id and filled model, got:\n%s", out.String())
	}
	if !seenDone {
		t.Fatalf("expected [DONE], got:\n%s", out.String())
	}
}
