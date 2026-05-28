package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/domain"
	"claude-bridge/internal/services"
)

type ChatHandler struct {
	chatService *services.ChatService
}

func NewChatHandler(chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

func (h *ChatHandler) Create(c *gin.Context) {
	var req domain.ChatRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid JSON",
		})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "messages required",
		})
		return
	}

	sessionID := c.GetHeader("X-Session-ID")

	if req.Stream {
		h.stream(c, req, sessionID)
		return
	}

	resp, err := h.chatService.Chat(
		c.Request.Context(),
		req,
		sessionID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ChatHandler) stream(c *gin.Context, req domain.ChatRequest, sessionID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	err := h.chatService.Stream(
		c.Request.Context(),
		c.Writer,
		req,
		sessionID,
	)
	if err != nil {
		_, _ = c.Writer.WriteString("data: {\"error\":\"" + escapeSSEError(err.Error()) + "\"}\n\n")
		_, _ = c.Writer.WriteString("data: [DONE]\n\n")
		c.Writer.Flush()
		return
	}
}

func escapeSSEError(value string) string {
	escaped := ""

	for _, r := range value {
		switch r {
		case '\\':
			escaped += "\\\\"
		case '"':
			escaped += "\\\""
		case '\n', '\r':
			escaped += " "
		default:
			escaped += string(r)
		}
	}

	return escaped
}
