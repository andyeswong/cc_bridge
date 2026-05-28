package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/services"
	"claude-bridge/internal/storage/models"
	"claude-bridge/internal/storage/repository"
)

type UsageHandler struct {
	usageService *services.UsageService
}

func NewUsageHandler(usageService *services.UsageService) *UsageHandler {
	return &UsageHandler{
		usageService: usageService,
	}
}

func (h *UsageHandler) Summary(c *gin.Context) {
	rows, err := h.usageService.Summary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if rows == nil {
		rows = []repository.UsageSummaryRow{}
	}

	c.JSON(http.StatusOK, rows)
}

func (h *UsageHandler) Recent(c *gin.Context) {
	limit := 50

	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}

	records, err := h.usageService.Recent(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if records == nil {
		records = []models.Usage{}
	}

	c.JSON(http.StatusOK, records)
}
