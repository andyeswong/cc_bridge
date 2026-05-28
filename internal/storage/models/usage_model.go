package models

import "time"

type Usage struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Model    string `gorm:"index;not null" json:"model"`
	Provider string `gorm:"index;not null" json:"provider"`

	PromptTokens     int `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int `gorm:"not null;default:0" json:"completion_tokens"`
	TotalTokens      int `gorm:"not null;default:0" json:"total_tokens"`

	DurationMs int  `gorm:"not null;default:0" json:"duration_ms"`
	IsError    bool `gorm:"not null;default:false" json:"is_error"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
