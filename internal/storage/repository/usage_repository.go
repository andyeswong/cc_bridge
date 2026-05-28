package repository

import (
	"claude-bridge/internal/storage/models"

	"gorm.io/gorm"
)

type UsageSummaryRow struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	TotalRequests    int64   `json:"total_requests"`
	Errors           int64   `json:"errors"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AvgDurationMs    float64 `json:"avg_duration_ms"`
}

type UsageRepository struct {
	db *gorm.DB
}

func NewUsageRepository(db *gorm.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

func (r *UsageRepository) Create(record *models.Usage) error {
	return r.db.Create(record).Error
}

func (r *UsageRepository) Summary() ([]UsageSummaryRow, error) {
	var rows []UsageSummaryRow

	err := r.db.
		Model(&models.Usage{}).
		Select(`
			model,
			provider,
			COUNT(*) AS total_requests,
			SUM(CASE WHEN is_error THEN 1 ELSE 0 END) AS errors,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(AVG(duration_ms), 0) AS avg_duration_ms
		`).
		Group("model, provider").
		Order("total_requests DESC").
		Scan(&rows).Error
	return rows, err
}

func (r *UsageRepository) FindRecent(limit int) ([]models.Usage, error) {
	if limit <= 0 {
		limit = 50
	}
	var records []models.Usage
	err := r.db.
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}
