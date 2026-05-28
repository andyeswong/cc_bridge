package ollama

import (
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

func TestResolveOllamaModel_StripsPrefix(t *testing.T) {
	t.Parallel()

	if got := resolveOllamaModel("ollama/llama3"); got != "llama3" {
		t.Fatalf("got %q", got)
	}
	if got := resolveOllamaModel("  ollama/phi3  "); got != "phi3" {
		t.Fatalf("got %q", got)
	}
	if got := resolveOllamaModel(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestChatEndpoint_ConstructsV1Path(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.Config{OllamaURL: "http://example.com/"}, sessions.NewMemoryStore[[]domain.Message](time.Minute, 10))
	if got := p.chatEndpoint(); got != "http://example.com/v1/chat/completions" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildMessages_SessionHistory(t *testing.T) {
	t.Parallel()

	store := sessions.NewMemoryStore[[]domain.Message](time.Minute, 10)
	p := NewProvider(config.Config{}, store)
	p.maxHistory = 10

	history := []domain.Message{
		{Role: "user", Content: "h1"},
		{Role: "assistant", Content: "h2"},
	}
	store.Set("sid", history)

	incoming := []domain.Message{
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	}

	got := p.buildMessages(incoming, "sid")
	if len(got) != 3 {
		t.Fatalf("len: got %d", len(got))
	}
	if got[0].Content != "h1" || got[1].Content != "h2" {
		t.Fatalf("history mismatch: %#v", got)
	}
	if got[2].Role != "user" || got[2].Content != "u2" {
		t.Fatalf("expected last user only appended: %#v", got[2])
	}

	noUser := []domain.Message{{Role: "assistant", Content: "x"}}
	got = p.buildMessages(noUser, "sid")
	if len(got) != len(history) || got[0].Content != "h1" || got[1].Content != "h2" {
		t.Fatalf("expected history unchanged when no user in incoming: %#v", got)
	}
}

func TestRunSync_ParsesResponseAndUsage(t *testing.T) {
	stop := "stop"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(domain.ChatResponse{
			ID:      "id",
			Object:  "chat.completion",
			Created: 1,
			Model:   "llama3",
			Choices: []domain.Choice{
				{
					Index: 0,
					Message: &domain.Message{
						Role:    "assistant",
						Content: "hello",
					},
					FinishReason: &stop,
				},
			},
			Usage: &domain.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
		})
	}))
	defer srv.Close()

	store := sessions.NewMemoryStore[[]domain.Message](time.Minute, 10)
	p := NewProvider(config.Config{OllamaURL: srv.URL}, store)
	p.httpClient = srv.Client()

	text, usage, err := p.runSync(context.Background(), "llama3", []domain.Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("runSync: %v", err)
	}
	if text != "hello" {
		t.Fatalf("text: got %q", text)
	}
	if usage == nil || usage.TotalTokens != 3 {
		t.Fatalf("usage: %#v", usage)
	}
}

func TestRunSync_Non2xxReturnsErrorWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	p := NewProvider(config.Config{OllamaURL: srv.URL}, sessions.NewMemoryStore[[]domain.Message](time.Minute, 10))
	p.httpClient = srv.Client()

	_, _, err := p.runSync(context.Background(), "llama3", []domain.Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "ollama status 500: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderChat_PersistsSessionHistory(t *testing.T) {
	stop := "stop"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(domain.ChatResponse{
			ID:      "id",
			Object:  "chat.completion",
			Created: 1,
			Model:   "llama3",
			Choices: []domain.Choice{
				{
					Index: 0,
					Message: &domain.Message{
						Role:    "assistant",
						Content: "hello",
					},
					FinishReason: &stop,
				},
			},
		})
	}))
	defer srv.Close()

	store := sessions.NewMemoryStore[[]domain.Message](time.Minute, 10)
	p := NewProvider(config.Config{OllamaURL: srv.URL}, store)
	p.httpClient = srv.Client()
	p.maxHistory = 10

	req := domain.ChatRequest{
		Model:    "ollama/llama3",
		Messages: []domain.Message{{Role: "user", Content: "hi"}},
	}

	_, _, err := p.Chat(context.Background(), req, "sid")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	history, ok := store.Get("sid")
	if !ok {
		t.Fatalf("expected session history")
	}
	if len(history) != 2 {
		t.Fatalf("history len: got %d", len(history))
	}
	if history[1].Role != "assistant" || history[1].Content != "hello" {
		t.Fatalf("history: %#v", history)
	}
}
