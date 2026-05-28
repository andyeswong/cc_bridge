package services

import (
	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"context"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ChatProvider interface {
	Chat(
		ctx context.Context,
		req domain.ChatRequest,
		clientSessionID string,
	) (*domain.ChatResponse, *domain.Usage, error)

	Stream(
		ctx context.Context,
		writer io.Writer,
		req domain.ChatRequest,
		clientSessionID string,
		responseID string,
	) (*domain.Usage, error)
}

type ChatService struct {
	cfg          config.Config
	claude       ChatProvider
	ollama       ChatProvider
	usageService *UsageService
}

func NewChatService(
	cfg config.Config,
	claude ChatProvider,
	ollama ChatProvider,
	usageService *UsageService,
) *ChatService {
	return &ChatService{
		cfg:          cfg,
		claude:       claude,
		ollama:       ollama,
		usageService: usageService,
	}
}

func (s *ChatService) Chat(
	ctx context.Context,
	req domain.ChatRequest,
	clientSessionID string,
) (*domain.ChatResponse, error) {
	req.Model = s.resolveModel(req.Model)

	providerName := domain.ResolveProvider(req.Model)
	provider := s.resolveProvider(providerName)

	start := time.Now()

	resp, usage, err := provider.Chat(ctx, req, clientSessionID)

	durationMs := int(time.Since(start).Milliseconds())

	s.logUsage(req.Model, providerName, usage, durationMs, err != nil)

	if err != nil {
		return nil, err
	}

	if resp.ID == "" {
		resp.ID = "chatcmpl-" + uuid.NewString()
	}

	if resp.Model == "" {
		resp.Model = req.Model
	}

	return resp, nil
}

func (s *ChatService) Stream(
	ctx context.Context,
	writer io.Writer,
	req domain.ChatRequest,
	clientSessionID string,
) error {
	req.Model = s.resolveModel(req.Model)

	providerName := domain.ResolveProvider(req.Model)
	provider := s.resolveProvider(providerName)

	responseID := "chatcmpl-" + uuid.NewString()

	start := time.Now()

	usage, err := provider.Stream(
		ctx,
		writer,
		req,
		clientSessionID,
		responseID,
	)

	durationMs := int(time.Since(start).Milliseconds())

	s.logUsage(req.Model, providerName, usage, durationMs, err != nil)

	return err
}

func (s *ChatService) resolveModel(model string) string {
	model = strings.TrimSpace(model)

	if model != "" {
		return model
	}

	if s.cfg.ClaudeDefaultModel != "" {
		return s.cfg.ClaudeDefaultModel
	}

	return domain.DefaultClaudeModel
}

func (s *ChatService) resolveProvider(providerName domain.ProviderName) ChatProvider {
	if providerName == domain.ProviderOllama {
		return s.ollama
	}

	return s.claude
}

func (s *ChatService) logUsage(
	model string,
	provider domain.ProviderName,
	usage *domain.Usage,
	durationMs int,
	isError bool,
) {
	input := domain.UsageLogInput{
		Model:      model,
		Provider:   provider,
		DurationMs: durationMs,
		IsError:    isError,
	}

	if usage != nil {
		input.PromptTokens = usage.PromptTokens
		input.CompletionTokens = usage.CompletionTokens
		input.TotalTokens = usage.TotalTokens
	}

	_ = s.usageService.Log(input)
}
