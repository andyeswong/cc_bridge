package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/domain"
	"claude-bridge/internal/providers/registry"
)

var claudeModels = []domain.ModelInfo{
	{ID: "claude-code", Object: "model", OwnedBy: "anthropic"},
	{ID: "claude-sonnet-4-6", Object: "model", OwnedBy: "anthropic"},
	{ID: "claude-sonnet-4-5", Object: "model", OwnedBy: "anthropic"},
	{ID: "claude-opus-4-5", Object: "model", OwnedBy: "anthropic"},
	{ID: "claude-haiku-3-5", Object: "model", OwnedBy: "anthropic"},
}

type ModelsHandler struct {
	registry *registry.Registry
}

func NewModelsHandler(reg *registry.Registry) *ModelsHandler {
	return &ModelsHandler{registry: reg}
}

func (h *ModelsHandler) Index(c *gin.Context) {
	now := time.Now().Unix()

	models := make([]domain.ModelInfo, len(claudeModels))
	for i, m := range claudeModels {
		m.Created = now
		models[i] = m
	}

	if h.registry != nil {
		for _, ep := range h.registry.Providers() {
			for _, model := range ep.Models {
				models = append(models, domain.ModelInfo{
					ID:      ep.Name + "," + model,
					Object:  "model",
					Created: now,
					OwnedBy: ep.Name,
				})
			}
		}
	}

	c.JSON(http.StatusOK, domain.ModelsResponse{
		Object: "list",
		Data:   models,
	})
}
