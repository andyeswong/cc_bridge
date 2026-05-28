package http

import (
	"github.com/gin-gonic/gin"

	"claude-bridge/internal/config"
	"claude-bridge/internal/http/handlers"
	"claude-bridge/internal/http/middleware"
)

type RouterDeps struct {
	Config config.Config

	ChatHandler      *handlers.ChatHandler
	ModelsHandler    *handlers.ModelsHandler
	UsageHandler     *handlers.UsageHandler
	HealthHandler    *handlers.HealthHandler
	DashboardHandler *handlers.DashboardHandler
}

func NewRouter(deps RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", deps.HealthHandler.Show)

	protected := router.Group("")
	protected.Use(middleware.LocalAuth(deps.Config))

	v1 := protected.Group("/v1")
	{
		v1.GET("/models", deps.ModelsHandler.Index)
		v1.POST("/chat/completions", deps.ChatHandler.Create)

		v1.GET("/usage", deps.UsageHandler.Summary)
		v1.GET("/usage/recent", deps.UsageHandler.Recent)
	}

	if deps.DashboardHandler != nil {
		protected.GET("/dashboard", deps.DashboardHandler.Index)
	}

	return router
}
