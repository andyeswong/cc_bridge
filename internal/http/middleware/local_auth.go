package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"claude-bridge/internal/config"
)

func LocalAuth(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.LocalAuthKey == "" {
			c.Next()
			return
		}

		expected := "Bearer " + cfg.LocalAuthKey
		authHeader := c.GetHeader("Authorization")

		if authHeader != expected {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
