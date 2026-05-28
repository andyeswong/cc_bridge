package ollama

import (
	"bytes"
	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"claude-bridge/internal/sessions"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultNumCtx     = 48000
	defaultMaxHistory = 40
)

type Provider struct {
	cfg        config.Config
	sessions   *sessions.MemoryStore[[]domain.Message]
	httpClient *http.Client
	maxHistory int
}

type Options struct {
	NumCtx int `json:"num_ctx"`
}

type requestBody struct {
	Model    string           `json:"model"`
	Messages []domain.Message `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  Options          `json:"options"`
}

func NewProvider(cfg config.Config, sessionStore *sessions.MemoryStore[[]domain.Message]) *Provider {
	return &Provider{
		cfg:      cfg,
		sessions: sessionStore,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		maxHistory: defaultMaxHistory,
	}
}

func (p *Provider) Chat(
	ctx context.Context,
	req domain.ChatRequest,
	clientSessionID string,
) (*domain.ChatResponse, *domain.Usage, error) {
	displayModel := req.Model
	ollamaModel := resolveOllamaModel(req.Model)

	messages := p.buildMessages(req.Messages, clientSessionID)

	text, usage, err := p.runSync(ctx, ollamaModel, messages)
	if err != nil {
		return nil, nil, err
	}

	if clientSessionID != "" {
		updated := append(messages, domain.Message{
			Role:    "assistant",
			Content: text,
		})

		p.sessions.Set(clientSessionID, trimHistory(updated, p.maxHistory))
	}

	resp := domain.NewChatResponse(displayModel, text, usage)

	return &resp, usage, nil
}

func (p *Provider) runSync(
	ctx context.Context,
	model string,
	messages []domain.Message,
) (string, *domain.Usage, error) {
	payload := requestBody{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options: Options{
			NumCtx: defaultNumCtx,
		},
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("ollama marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.chatEndpoint(),
		bytes.NewReader(rawPayload),
	)
	if err != nil {
		return "", nil, fmt.Errorf("ollama create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("ollama read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("ollama status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var chatResp domain.ChatResponse
	if err := json.Unmarshal(rawBody, &chatResp); err != nil {
		return "", nil, fmt.Errorf("ollama parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", chatResp.Usage, fmt.Errorf("ollama empty choices")
	}

	message := chatResp.Choices[0].Message
	if message == nil {
		return "", chatResp.Usage, fmt.Errorf("ollama empty message")
	}

	return message.Content, chatResp.Usage, nil
}

func (p *Provider) buildMessages(
	incoming []domain.Message,
	clientSessionID string,
) []domain.Message {
	if clientSessionID == "" {
		return incoming
	}

	history, ok := p.sessions.Get(clientSessionID)
	if !ok {
		return incoming
	}

	lastUser := lastUserMessage(incoming)
	if lastUser == "" {
		return history
	}

	messages := append([]domain.Message{}, history...)
	messages = append(messages, domain.Message{
		Role:    "user",
		Content: lastUser,
	})

	return trimHistory(messages, p.maxHistory)
}

func (p *Provider) chatEndpoint() string {
	base := strings.TrimRight(p.cfg.OllamaURL, "/")
	return base + "/v1/chat/completions"
}

func resolveOllamaModel(model string) string {
	model = strings.TrimSpace(model)

	if model == "" {
		return ""
	}

	return domain.StripOllamaPrefix(model)
}

func lastUserMessage(messages []domain.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			return messages[index].Content
		}
	}

	return ""
}

func trimHistory(messages []domain.Message, max int) []domain.Message {
	if max <= 0 {
		return messages
	}

	if len(messages) <= max {
		return messages
	}

	return messages[len(messages)-max:]
}
