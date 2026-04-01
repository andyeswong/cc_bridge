package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── OpenAI-compatible types ──────────────────────────────────────────────────

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	// optional: max_tokens, temperature, etc. (ignored by bridge)
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	Delta        *Delta  `json:"delta,omitempty"`
	FinishReason *string `json:"finish_reason"`
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
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

// ─── Claude CLI output types ──────────────────────────────────────────────────

// Non-streaming: --output-format json
type ClaudeResult struct {
	Type     string       `json:"type"`    // "result"
	Subtype  string       `json:"subtype"` // "success" | "error"
	IsError  bool         `json:"is_error"`
	Result   string       `json:"result"`
	Usage    *ClaudeUsage `json:"usage,omitempty"`
}

// Streaming: --output-format stream-json (one JSON object per line)
type ClaudeStreamEvent struct {
	Type    string              `json:"type"`
	Subtype string              `json:"subtype,omitempty"`
	IsError bool                `json:"is_error,omitempty"`
	Result  string              `json:"result,omitempty"`
	Message *ClaudeStreamMsg    `json:"message,omitempty"`
}

type ClaudeStreamMsg struct {
	Content []ClaudeStreamBlock `json:"content"`
}

type ClaudeStreamBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ClaudeDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ─── Session store (stateful mode) ───────────────────────────────────────────

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]string // clientSessionID -> claudeSessionID
}

var store = &SessionStore{sessions: make(map[string]string)}

func (s *SessionStore) Get(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.sessions[id]
	return v, ok
}

func (s *SessionStore) Set(clientID, claudeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[clientID] = claudeID
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func claudePath() string {
	// try PATH first, then npm global
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	candidates := []string{
		"/home/entdev/.npm-global/bin/claude",
		"/usr/local/bin/claude",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "claude"
}

// formatMessages converts the OpenAI messages slice into a single text prompt.
// Used in stateless mode or as the initial turn in stateful mode.
func formatMessages(messages []Message) string {
	var sb strings.Builder
	for _, m := range messages {
		switch m.Role {
		case "system":
			sb.WriteString(fmt.Sprintf("<system>\n%s\n</system>\n\n", m.Content))
		case "user":
			sb.WriteString(fmt.Sprintf("Human: %s\n\n", m.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", m.Content))
		}
	}
	return strings.TrimSpace(sb.String())
}

// lastUserMessage returns only the last user message (used in stateful continuation).
func lastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// ─── Claude invocation ───────────────────────────────────────────────────────

type claudeArgs struct {
	prompt       string
	model        string
	sessionID    string // if set, use --resume
	newSessionID string // if set, use --session-id (first call)
	stream       bool
}

func buildArgs(a claudeArgs) []string {
	args := []string{"-p", "--output-format"}
	if a.stream {
		args = append(args, "stream-json", "--verbose")
	} else {
		args = append(args, "json")
	}
	if a.model != "" && a.model != "claude-code" {
		args = append(args, "--model", a.model)
	}
	if a.sessionID != "" {
		args = append(args, "--resume", a.sessionID)
	} else if a.newSessionID != "" {
		args = append(args, "--session-id", a.newSessionID)
	}
	args = append(args, "--dangerously-skip-permissions")
	args = append(args, a.prompt)
	return args
}

// runClaudeSync runs claude in non-streaming mode and returns the full text.
func runClaudeSync(a claudeArgs) (string, string, *ClaudeUsage, error) {
	args := buildArgs(a)
	cmd := exec.Command(claudePath(), args...)
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		// claude may write the error result to stdout as JSON
		var errResult ClaudeResult
		if jsonErr := json.Unmarshal(bytes.TrimSpace(out), &errResult); jsonErr == nil && errResult.IsError {
			return "", "", nil, fmt.Errorf("claude error: %s", errResult.Result)
		}
		return "", "", nil, fmt.Errorf("claude exited: %w", err)
	}

	// CLI --output-format json returns a ClaudeResult object
	var result ClaudeResult
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		// fallback: treat raw output as text
		return strings.TrimSpace(string(out)), "", nil, nil
	}
	return result.Result, "", result.Usage, nil
}

// runClaudeStream runs claude in stream-json mode and writes SSE chunks to w.
func runClaudeStream(w http.ResponseWriter, a claudeArgs, respID, model string) {
	args := buildArgs(a)
	cmd := exec.Command(claudePath(), args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "pipe error", 500)
		return
	}
	cmd.Start()

	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(stdout)

	// send role delta first
	sendSSEDelta(w, respID, model, "assistant", "", nil)
	if canFlush {
		flusher.Flush()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var ev ClaudeStreamEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			// stream-json emits full assistant messages incrementally
			if ev.Message != nil {
				for _, block := range ev.Message.Content {
					if block.Type == "text" && block.Text != "" {
						sendSSEDelta(w, respID, model, "", block.Text, nil)
						if canFlush {
							flusher.Flush()
						}
					}
				}
			}
		case "result":
			stop := "stop"
			sendSSEDelta(w, respID, model, "", "", &stop)
			if canFlush {
				flusher.Flush()
			}
		}
	}

	cmd.Wait()

	// send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}

func sendSSEDelta(w http.ResponseWriter, id, model, role, content string, finishReason *string) {
	delta := &Delta{}
	if role != "" {
		delta.Role = role
	}
	if content != "" {
		delta.Content = content
	}
	chunk := ChatResponse{
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
	b, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", b)
}

// ─── HTTP Handlers ────────────────────────────────────────────────────────────

func handleModels(w http.ResponseWriter, r *http.Request) {
	models := ModelsResponse{
		Object: "list",
		Data: []ModelInfo{
			{ID: "claude-code", Object: "model", Created: 1700000000, OwnedBy: "anthropic"},
			{ID: "claude-sonnet-4-5", Object: "model", Created: 1700000000, OwnedBy: "anthropic"},
			{ID: "claude-opus-4-5", Object: "model", Created: 1700000000, OwnedBy: "anthropic"},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "read error", 400)
		return
	}
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, "messages required", 400)
		return
	}

	respID := "chatcmpl-" + uuid.New().String()
	model := req.Model
	if model == "" {
		model = "claude-code"
	}

	// ── Determine stateful vs stateless ──────────────────────────────────────
	clientSessionID := r.Header.Get("X-Session-ID")
	var args claudeArgs
	args.stream = req.Stream
	args.model = model

	if clientSessionID != "" {
		// Stateful mode
		if claudeID, ok := store.Get(clientSessionID); ok {
			// continuation: send only the last user message
			args.sessionID = claudeID
			args.prompt = lastUserMessage(req.Messages)
		} else {
			// first call for this client session
			newID := uuid.New().String()
			store.Set(clientSessionID, newID)
			args.newSessionID = newID
			args.prompt = formatMessages(req.Messages)
		}
	} else {
		// Stateless mode: format full history as prompt
		args.prompt = formatMessages(req.Messages)
	}

	// ── Streaming ─────────────────────────────────────────────────────────────
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		runClaudeStream(w, args, respID, model)
		return
	}

	// ── Non-streaming ─────────────────────────────────────────────────────────
	text, _, usage, err := runClaudeSync(args)
	if err != nil {
		log.Printf("claude error: %v", err)
		http.Error(w, "claude error: "+err.Error(), 500)
		return
	}

	stop := "stop"
	resp := ChatResponse{
		ID:      respID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: text},
				FinishReason: &stop,
			},
		},
	}
	if usage != nil {
		resp.Usage = &Usage{
			PromptTokens:     usage.InputTokens,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      usage.InputTokens + usage.OutputTokens,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/v1/chat/completions", handleChat)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("claude-bridge listening on :%s", port)
	log.Printf("claude binary: %s", claudePath())
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
