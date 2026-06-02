package services

import (
	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"claude-bridge/internal/providers/claude"
	"claude-bridge/internal/providers/external"
	"claude-bridge/internal/providers/registry"
	"claude-bridge/internal/sessions"
	"context"
	"io"
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
	cfg             config.Config
	claude          ChatProvider
	registry        *registry.Registry
	extProviders    map[string]*external.Provider
	backedProviders map[string]*claude.Provider
	claudeSessions  *sessions.MemoryStore[string]
	usageService    *UsageService
}

func NewChatService(
	cfg config.Config,
	claudeProvider ChatProvider,
	reg *registry.Registry,
	claudeSessions *sessions.MemoryStore[string],
	usageService *UsageService,
) *ChatService {
	return &ChatService{
		cfg:             cfg,
		claude:          claudeProvider,
		registry:        reg,
		extProviders:    make(map[string]*external.Provider),
		backedProviders: make(map[string]*claude.Provider),
		claudeSessions:  claudeSessions,
		usageService:    usageService,
	}
}

func (s *ChatService) Chat(
	ctx context.Context,
	req domain.ChatRequest,
	clientSessionID string,
) (*domain.ChatResponse, error) {
	req.Model = s.resolveModel(req.Model)

	provider, targetModel, providerName := s.resolveExecutor(req.Model)
	req.Model = targetModel

	start := time.Now()

	resp, usage, err := provider.Chat(ctx, req, clientSessionID)

	durationMs := int(time.Since(start).Milliseconds())
	s.logUsage(targetModel, providerName, usage, durationMs, err != nil)

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

	provider, targetModel, providerName := s.resolveExecutor(req.Model)
	req.Model = targetModel

	responseID := "chatcmpl-" + uuid.NewString()

	start := time.Now()

	usage, err := provider.Stream(ctx, writer, req, clientSessionID, responseID)

	durationMs := int(time.Since(start).Milliseconds())
	s.logUsage(targetModel, providerName, usage, durationMs, err != nil)

	return err
}

// resolveExecutor returns the provider, the target model name (with provider prefix
// stripped), and a provider label for usage logging.
// Priority: registry match → Claude Code fallback.
func (s *ChatService) resolveExecutor(model string) (ChatProvider, string, string) {
	if s.registry != nil && !s.registry.Empty() {
		if match := s.registry.Resolve(model); match != nil {
			// "claude" driver: run Claude Code (body) with this provider as the brain.
			if match.Provider.Driver == "claude" {
				p := s.getOrCreateBackedProvider(match.Provider)
				return p, match.TargetModel, match.Provider.Name
			}
			// default: generic OpenAI-compatible HTTP passthrough.
			p := s.getOrCreateExtProvider(match.Provider)
			return p, match.TargetModel, match.Provider.Name
		}
	}

	return s.claude, model, string(domain.ProviderClaude)
}

func (s *ChatService) getOrCreateExtProvider(ep *registry.ExternalProvider) *external.Provider {
	if p, ok := s.extProviders[ep.Name]; ok {
		return p
	}
	p := external.New(ep)
	s.extProviders[ep.Name] = p
	return p
}

func (s *ChatService) getOrCreateBackedProvider(ep *registry.ExternalProvider) *claude.Provider {
	if p, ok := s.backedProviders[ep.Name]; ok {
		return p
	}
	p := claude.NewBackedProvider(s.cfg, s.claudeSessions, ep.BaseURL, ep.APIKey)
	s.backedProviders[ep.Name] = p
	return p
}

func (s *ChatService) resolveModel(model string) string {
	if model != "" {
		return model
	}

	if s.cfg.ClaudeDefaultModel != "" {
		return s.cfg.ClaudeDefaultModel
	}

	return domain.DefaultClaudeModel
}

func (s *ChatService) logUsage(
	model string,
	providerName string,
	usage *domain.Usage,
	durationMs int,
	isError bool,
) {
	input := domain.UsageLogInput{
		Model:      model,
		Provider:   domain.ProviderName(providerName),
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
