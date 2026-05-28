package services

import (
	"claude-bridge/internal/domain"
	"claude-bridge/internal/storage/models"
	"claude-bridge/internal/storage/repository"
)

type UsageService struct {
	repository *repository.UsageRepository
}

func NewUsageService(repository *repository.UsageRepository) *UsageService {
	return &UsageService{
		repository: repository,
	}
}

func (s *UsageService) Log(input domain.UsageLogInput) error {
	record := &models.Usage{
		Model:            input.Model,
		Provider:         string(input.Provider),
		PromptTokens:     input.PromptTokens,
		CompletionTokens: input.CompletionTokens,
		TotalTokens:      input.TotalTokens,
		DurationMs:       input.DurationMs,
		IsError:          input.IsError,
	}

	return s.repository.Create(record)
}

func (s *UsageService) Summary() ([]repository.UsageSummaryRow, error) {
	return s.repository.Summary()
}

func (s *UsageService) Recent(limit int) ([]models.Usage, error) {
	return s.repository.FindRecent(limit)
}
