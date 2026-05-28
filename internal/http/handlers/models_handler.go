package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/domain"
)

type ModelsHandler struct {
	models []domain.ModelInfo
}

func NewModelsHandler() *ModelsHandler {
	now := time.Now().Unix()

	return &ModelsHandler{
		models: []domain.ModelInfo{
			{
				ID:      "claude-code",
				Object:  "model",
				Created: now,
				OwnedBy: "anthropic",
			},
			{
				ID:      "claude-sonnet-4-6",
				Object:  "model",
				Created: now,
				OwnedBy: "anthropic",
			},
			{
				ID:      "claude-sonnet-4-5",
				Object:  "model",
				Created: now,
				OwnedBy: "anthropic",
			},
			{
				ID:      "claude-opus-4-5",
				Object:  "model",
				Created: now,
				OwnedBy: "anthropic",
			},
			{
				ID:      "claude-haiku-3-5",
				Object:  "model",
				Created: now,
				OwnedBy: "anthropic",
			},
		},
	}
}

func (h *ModelsHandler) Index(c *gin.Context) {
	c.JSON(http.StatusOK, domain.ModelsResponse{
		Object: "list",
		Data:   h.models,
	})
}
