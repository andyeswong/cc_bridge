package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/config"
)

type HealthHandler struct {
	cfg config.Config
}

func NewHealthHandler(cfg config.Config) *HealthHandler {
	return &HealthHandler{
		cfg: cfg,
	}
}

func (h *HealthHandler) Show(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"local":      h.cfg.Host == "127.0.0.1" || h.cfg.Host == "localhost",
		"host":       h.cfg.Host,
		"port":       h.cfg.Port,
		"auth":       h.cfg.LocalAuthKey != "",
		"ollama_url": h.cfg.OllamaURL,
		"usage_db":   h.cfg.UsageDBPath,
		"skip_perms": h.cfg.ClaudeSkipPerms,
	})
}
