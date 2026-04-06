package main

import (
	"bufio"
	"bytes"
	"database/sql"
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
	_ "modernc.org/sqlite"
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

type ClaudeResult struct {
	Type    string       `json:"type"`
	Subtype string       `json:"subtype"`
	IsError bool         `json:"is_error"`
	Result  string       `json:"result"`
	Usage   *ClaudeUsage `json:"usage,omitempty"`
}

type ClaudeStreamEvent struct {
	Type    string           `json:"type"`
	Subtype string           `json:"subtype,omitempty"`
	IsError bool             `json:"is_error,omitempty"`
	Result  string           `json:"result,omitempty"`
	Message *ClaudeStreamMsg `json:"message,omitempty"`
	Usage   *ClaudeUsage     `json:"usage,omitempty"`
}

type ClaudeStreamMsg struct {
	Content []ClaudeStreamBlock `json:"content"`
}

type ClaudeStreamBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ─── Session stores ───────────────────────────────────────────────────────────

// Claude stateful sessions: clientSessionID -> claudeSessionID
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]string
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

// Ollama stateful sessions: clientSessionID -> []Message history
type OllamaSessionStore struct {
	mu       sync.RWMutex
	sessions map[string][]Message
}

var ollamaStore = &OllamaSessionStore{sessions: make(map[string][]Message)}

func (s *OllamaSessionStore) Get(id string) ([]Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.sessions[id]
	return v, ok
}

func (s *OllamaSessionStore) Set(id string, msgs []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = msgs
}

// ─── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	Port         string // PORT
	ClaudeBin    string // CLAUDE_BIN
	SkipPerms    bool   // CLAUDE_SKIP_PERMS
	DefaultModel string // CLAUDE_DEFAULT_MODEL
	AuthKey      string // CLAUDE_AUTH_KEY
	Workdir      string // CLAUDE_WORKDIR
	OllamaURL    string // OLLAMA_URL
	UsageDBPath  string // USAGE_DB_PATH
}

var cfg Config

func loadConfig() {
	cfg.Port = envOr("PORT", "8080")
	cfg.ClaudeBin = os.Getenv("CLAUDE_BIN")
	cfg.SkipPerms = os.Getenv("CLAUDE_SKIP_PERMS") == "true" || os.Getenv("CLAUDE_SKIP_PERMS") == "1"
	cfg.DefaultModel = envOr("CLAUDE_DEFAULT_MODEL", "claude-code")
	cfg.AuthKey = os.Getenv("CLAUDE_AUTH_KEY")
	cfg.Workdir = os.Getenv("CLAUDE_WORKDIR")
	cfg.OllamaURL = envOr("OLLAMA_URL", "https://ollama.andres-wong.com")
	cfg.UsageDBPath = envOr("USAGE_DB_PATH", "./usage.db")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ─── SQLite usage tracking ────────────────────────────────────────────────────

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", cfg.UsageDBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS requests (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		ts                DATETIME DEFAULT CURRENT_TIMESTAMP,
		model             TEXT,
		provider          TEXT,
		prompt_tokens     INTEGER,
		completion_tokens INTEGER,
		total_tokens      INTEGER,
		duration_ms       INTEGER,
		is_error          INTEGER DEFAULT 0
	)`)
	return err
}

func logRequest(model, provider string, promptTokens, completionTokens, totalTokens, durationMs int, isError bool) {
	if db == nil {
		return
	}
	errInt := 0
	if isError {
		errInt = 1
	}
	_, err := db.Exec(
		`INSERT INTO requests (model, provider, prompt_tokens, completion_tokens, total_tokens, duration_ms, is_error)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		model, provider, promptTokens, completionTokens, totalTokens, durationMs, errInt,
	)
	if err != nil {
		log.Printf("usage log error: %v", err)
	}
}

// ─── Usage query ──────────────────────────────────────────────────────────────

type UsageSummaryRow struct {
	Model         string  `json:"model"`
	Provider      string  `json:"provider"`
	TotalRequests int     `json:"total_requests"`
	TotalTokens   int     `json:"total_tokens"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

func queryUsageSummary() ([]UsageSummaryRow, error) {
	rows, err := db.Query(`
		SELECT model, provider,
		       COUNT(*) AS total_requests,
		       COALESCE(SUM(total_tokens), 0) AS total_tokens,
		       COALESCE(AVG(duration_ms), 0) AS avg_duration_ms
		FROM requests
		GROUP BY model, provider
		ORDER BY total_requests DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []UsageSummaryRow
	for rows.Next() {
		var r UsageSummaryRow
		if err := rows.Scan(&r.Model, &r.Provider, &r.TotalRequests, &r.TotalTokens, &r.AvgDurationMs); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>cc_bridge Dashboard</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{background:#0f0f1a;color:#c8d0e0;font-family:ui-monospace,monospace;padding:2rem;line-height:1.5}
h1{color:#7eb8f7;font-size:1.4rem;margin-bottom:.25rem;letter-spacing:.02em}
.meta{color:#445;font-size:.8rem;margin-bottom:2rem}
h2{color:#8899bb;font-size:.95rem;margin:1.5rem 0 .6rem;text-transform:uppercase;letter-spacing:.08em}
table{width:100%;border-collapse:collapse}
th{background:#151525;color:#7eb8f7;padding:.5rem 1rem;text-align:left;border-bottom:2px solid #252540;font-size:.8rem;text-transform:uppercase;letter-spacing:.06em}
td{padding:.5rem 1rem;border-bottom:1px solid #1a1a2e;font-size:.9rem}
tr:hover td{background:#14142a}
.p-claude{color:#80c8ff}
.p-ollama{color:#80ffb0}
.empty{color:#445;padding:1rem;font-style:italic;font-size:.9rem}
.num{text-align:right}
</style>
<script>setTimeout(function(){location.reload()},30000)</script>
</head>
<body>
<h1>cc_bridge Usage Dashboard</h1>
<p class="meta">Auto-refreshes every 30 seconds &mdash; Last updated: %s</p>
<h2>Summary by Model</h2>
%s
</body>
</html>`

func htmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}

func renderDashboard(rows []UsageSummaryRow) string {
	ts := time.Now().Format("2006-01-02 15:04:05 UTC")
	var body string
	if len(rows) == 0 {
		body = `<p class="empty">No requests recorded yet.</p>`
	} else {
		var sb strings.Builder
		sb.WriteString(`<table>`)
		sb.WriteString(`<tr><th>Model</th><th>Provider</th><th class="num">Requests</th><th class="num">Total Tokens</th><th class="num">Avg Duration (ms)</th></tr>`)
		for _, r := range rows {
			cls := "p-claude"
			if r.Provider == "ollama" {
				cls = "p-ollama"
			}
			fmt.Fprintf(&sb,
				`<tr><td>%s</td><td class="%s">%s</td><td class="num">%d</td><td class="num">%d</td><td class="num">%.0f</td></tr>`,
				htmlEsc(r.Model), cls, htmlEsc(r.Provider), r.TotalRequests, r.TotalTokens, r.AvgDurationMs,
			)
		}
		sb.WriteString(`</table>`)
		body = sb.String()
	}
	return fmt.Sprintf(dashboardTemplate, ts, body)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func claudePath() string {
	if cfg.ClaudeBin != "" {
		return cfg.ClaudeBin
	}
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	return "claude"
}

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

func lastUserMessage(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// ─── Claude invocation ────────────────────────────────────────────────────────

type claudeArgs struct {
	prompt       string
	model        string
	sessionID    string
	newSessionID string
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
	if cfg.SkipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}
	args = append(args, a.prompt)
	return args
}

func runClaudeSync(a claudeArgs) (string, string, *ClaudeUsage, error) {
	args := buildArgs(a)
	cmd := exec.Command(claudePath(), args...)
	cmd.Env = os.Environ()
	if cfg.Workdir != "" {
		cmd.Dir = cfg.Workdir
	}

	out, err := cmd.Output()
	if err != nil {
		var errResult ClaudeResult
		if jsonErr := json.Unmarshal(bytes.TrimSpace(out), &errResult); jsonErr == nil && errResult.IsError {
			return "", "", nil, fmt.Errorf("claude error: %s", errResult.Result)
		}
		return "", "", nil, fmt.Errorf("claude exited: %w", err)
	}

	var result ClaudeResult
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		return strings.TrimSpace(string(out)), "", nil, nil
	}
	return result.Result, "", result.Usage, nil
}

// runClaudeStream runs claude in stream-json mode, writes SSE to w, and returns captured usage.
func runClaudeStream(w http.ResponseWriter, a claudeArgs, respID, model string) *ClaudeUsage {
	args := buildArgs(a)
	cmd := exec.Command(claudePath(), args...)
	cmd.Env = os.Environ()
	if cfg.Workdir != "" {
		cmd.Dir = cfg.Workdir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "pipe error", 500)
		return nil
	}
	cmd.Start()

	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(stdout)

	var capturedUsage *ClaudeUsage

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
			if ev.Usage != nil {
				capturedUsage = ev.Usage
			}
			stop := "stop"
			sendSSEDelta(w, respID, model, "", "", &stop)
			if canFlush {
				flusher.Flush()
			}
		}
	}

	cmd.Wait()

	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}

	return capturedUsage
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
			{Index: 0, Delta: delta, FinishReason: finishReason},
		},
	}
	b, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", b)
}

// ─── Ollama invocation ────────────────────────────────────────────────────────

type ollamaOptions struct {
	NumCtx int `json:"num_ctx"`
}

type ollamaReqBody struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  ollamaOptions `json:"options"`
}

func ollamaEndpoint() string {
	return cfg.OllamaURL + "/v1/chat/completions"
}

func runOllamaSync(model string, messages []Message) (string, *Usage, error) {
	body, _ := json.Marshal(ollamaReqBody{Model: model, Messages: messages, Stream: false, Options: ollamaOptions{NumCtx: 48000}})
	resp, err := http.Post(ollamaEndpoint(), "application/json", bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("ollama read: %w", err)
	}

	var cr ChatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", nil, fmt.Errorf("ollama parse: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", nil, fmt.Errorf("ollama: empty choices")
	}
	return cr.Choices[0].Message.Content, cr.Usage, nil
}

// runOllamaStream proxies SSE from Ollama to w, returns (assistantText, usage).
func runOllamaStream(w http.ResponseWriter, model string, messages []Message, respID string) (string, *Usage) {
	body, _ := json.Marshal(ollamaReqBody{Model: model, Messages: messages, Stream: true, Options: ollamaOptions{NumCtx: 48000}})
	resp, err := http.Post(ollamaEndpoint(), "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("ollama stream error: %v", err)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		return "", nil
	}
	defer resp.Body.Close()

	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(resp.Body)

	var assistantText strings.Builder
	var capturedUsage *Usage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			if canFlush {
				flusher.Flush()
			}
			break
		}

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			capturedUsage = chunk.Usage
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			assistantText.WriteString(chunk.Choices[0].Delta.Content)
		}

		// Re-emit with our respID
		chunk.ID = respID
		out, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", out)
		if canFlush {
			flusher.Flush()
		}
	}

	return assistantText.String(), capturedUsage
}

// ─── HTTP Handlers ────────────────────────────────────────────────────────────

var knownModels = []string{
	"claude-code",
	"claude-sonnet-4-6",
	"claude-sonnet-4-5",
	"claude-opus-4-5",
	"claude-haiku-3-5",
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	ts := time.Now().Unix()
	data := make([]ModelInfo, 0, len(knownModels))
	for _, id := range knownModels {
		data = append(data, ModelInfo{ID: id, Object: "model", Created: ts, OwnedBy: "anthropic"})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ModelsResponse{Object: "list", Data: data})
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthKey == "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+cfg.AuthKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "read error", 400)
		return
	}
	var req ChatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
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
		model = cfg.DefaultModel
	}

	// ── Ollama routing ────────────────────────────────────────────────────────
	if strings.HasPrefix(model, "ollama/") {
		ollamaModel := strings.TrimPrefix(model, "ollama/")
		handleOllamaChat(w, r, req, respID, model, ollamaModel)
		return
	}

	// ── Claude routing ────────────────────────────────────────────────────────
	start := time.Now()
	clientSessionID := r.Header.Get("X-Session-ID")

	var args claudeArgs
	args.stream = req.Stream
	args.model = model

	if clientSessionID != "" {
		if claudeID, ok := store.Get(clientSessionID); ok {
			args.sessionID = claudeID
			args.prompt = lastUserMessage(req.Messages)
		} else {
			newID := uuid.New().String()
			store.Set(clientSessionID, newID)
			args.newSessionID = newID
			args.prompt = formatMessages(req.Messages)
		}
	} else {
		args.prompt = formatMessages(req.Messages)
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		usage := runClaudeStream(w, args, respID, model)
		ms := int(time.Since(start).Milliseconds())
		if usage != nil {
			logRequest(model, "claude", usage.InputTokens, usage.OutputTokens, usage.InputTokens+usage.OutputTokens, ms, false)
		} else {
			logRequest(model, "claude", 0, 0, 0, ms, false)
		}
		return
	}

	text, _, usage, err := runClaudeSync(args)
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		log.Printf("claude error: %v", err)
		logRequest(model, "claude", 0, 0, 0, ms, true)
		http.Error(w, "claude error: "+err.Error(), 500)
		return
	}

	pt, ct, tt := 0, 0, 0
	if usage != nil {
		pt, ct, tt = usage.InputTokens, usage.OutputTokens, usage.InputTokens+usage.OutputTokens
	}
	logRequest(model, "claude", pt, ct, tt, ms, false)

	stop := "stop"
	resp := ChatResponse{
		ID:      respID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{Index: 0, Message: Message{Role: "assistant", Content: text}, FinishReason: &stop},
		},
	}
	if usage != nil {
		resp.Usage = &Usage{PromptTokens: pt, CompletionTokens: ct, TotalTokens: tt}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleOllamaChat(w http.ResponseWriter, r *http.Request, req ChatRequest, respID, displayModel, ollamaModel string) {
	start := time.Now()
	clientSessionID := r.Header.Get("X-Session-ID")

	// Build messages: for stateful sessions, append new user message to stored history
	var messages []Message
	if clientSessionID != "" {
		if history, ok := ollamaStore.Get(clientSessionID); ok {
			// Continuation: append only the last user message
			lastMsg := req.Messages[len(req.Messages)-1]
			messages = append(history, lastMsg)
		} else {
			// First call: use full messages array
			messages = req.Messages
		}
	} else {
		messages = req.Messages
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		assistantText, usage := runOllamaStream(w, ollamaModel, messages, respID)
		ms := int(time.Since(start).Milliseconds())

		if clientSessionID != "" && assistantText != "" {
			updated := append(messages, Message{Role: "assistant", Content: assistantText})
			ollamaStore.Set(clientSessionID, updated)
		}

		if usage != nil {
			logRequest(displayModel, "ollama", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, ms, false)
		} else {
			logRequest(displayModel, "ollama", 0, 0, 0, ms, false)
		}
		return
	}

	text, usage, err := runOllamaSync(ollamaModel, messages)
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		log.Printf("ollama error: %v", err)
		logRequest(displayModel, "ollama", 0, 0, 0, ms, true)
		http.Error(w, "ollama error: "+err.Error(), 500)
		return
	}

	if clientSessionID != "" {
		updated := append(messages, Message{Role: "assistant", Content: text})
		ollamaStore.Set(clientSessionID, updated)
	}

	pt, ct, tt := 0, 0, 0
	if usage != nil {
		pt, ct, tt = usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens
	}
	logRequest(displayModel, "ollama", pt, ct, tt, ms, false)

	stop := "stop"
	resp := ChatResponse{
		ID:      respID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   displayModel,
		Choices: []Choice{
			{Index: 0, Message: Message{Role: "assistant", Content: text}, FinishReason: &stop},
		},
		Usage: usage,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	rows, err := queryUsageSummary()
	if err != nil {
		http.Error(w, "db error: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderDashboard(rows))
}

func handleUsageJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := queryUsageSummary()
	if err != nil {
		http.Error(w, "db error: "+err.Error(), 500)
		return
	}
	if rows == nil {
		rows = []UsageSummaryRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	loadConfig()

	if err := initDB(); err != nil {
		log.Fatalf("db init: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", authMiddleware(handleModels))
	mux.HandleFunc("/v1/chat/completions", authMiddleware(handleChat))
	mux.HandleFunc("/v1/usage", authMiddleware(handleUsageJSON))
	mux.HandleFunc("/dashboard", handleDashboard)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","claude":"%s","workdir":"%s","skip_perms":%v}`,
			claudePath(), cfg.Workdir, cfg.SkipPerms)
	})

	log.Printf("cc_bridge starting")
	log.Printf("  port:          :%s", cfg.Port)
	log.Printf("  claude binary: %s", claudePath())
	log.Printf("  workdir:       %s", cfg.Workdir)
	log.Printf("  skip_perms:    %v", cfg.SkipPerms)
	log.Printf("  default model: %s", cfg.DefaultModel)
	log.Printf("  auth:          %v", cfg.AuthKey != "")
	log.Printf("  ollama url:    %s", cfg.OllamaURL)
	log.Printf("  ollama num_ctx: 48000")
	log.Printf("  usage db:      %s", cfg.UsageDBPath)

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
