package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-bridge/internal/domain"
	"claude-bridge/internal/providers/registry"
)

type Provider struct {
	ep         *registry.ExternalProvider
	httpClient *http.Client
}

func New(ep *registry.ExternalProvider) *Provider {
	return &Provider{
		ep: ep,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

type requestBody struct {
	Model      string          `json:"model"`
	Messages   []domain.Message `json:"messages"`
	Stream     bool            `json:"stream"`
	Tools      []domain.Tool   `json:"tools,omitempty"`
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
}

func (p *Provider) Chat(
	ctx context.Context,
	req domain.ChatRequest,
	_ string,
) (*domain.ChatResponse, *domain.Usage, error) {
	body := requestBody{
		Model:      req.Model,
		Messages:   req.Messages,
		Stream:     false,
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("external marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(), bytes.NewReader(rawBody))
	if err != nil {
		return nil, nil, fmt.Errorf("external create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.ep.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.ep.APIKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("external request: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("external read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("external status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawResp)))
	}

	var chatResp domain.ChatResponse
	if err := json.Unmarshal(rawResp, &chatResp); err != nil {
		return nil, nil, fmt.Errorf("external parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, chatResp.Usage, fmt.Errorf("external empty choices")
	}

	if chatResp.Model == "" {
		chatResp.Model = req.Model
	}

	return &chatResp, chatResp.Usage, nil
}

func (p *Provider) endpoint() string {
	return strings.TrimRight(p.ep.BaseURL, "/") + "/chat/completions"
}
