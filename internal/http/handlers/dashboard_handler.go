package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/services"
	dashboardTemplate "claude-bridge/web/dashboard"
)

type DashboardHandler struct {
	usageService *services.UsageService
}

func NewDashboardHandler(usageService *services.UsageService) *DashboardHandler {
	return &DashboardHandler{
		usageService: usageService,
	}
}

func (h *DashboardHandler) Index(c *gin.Context) {
	rows, err := h.usageService.Summary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	html, err := dashboardTemplate.RenderUsageDashboard(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
