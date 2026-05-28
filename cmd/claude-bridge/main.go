package main

import (
	"log"
	"net/http"
	"time"

	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	bridgehttp "claude-bridge/internal/http"
	"claude-bridge/internal/http/handlers"
	"claude-bridge/internal/providers/claude"
	"claude-bridge/internal/providers/ollama"
	"claude-bridge/internal/services"
	"claude-bridge/internal/sessions"
	"claude-bridge/internal/storage"
	"claude-bridge/internal/storage/repository"
)

func main() {
	cfg := config.Load()

	db, err := storage.OpenDatabase(cfg.UsageDBPath)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	usageRepository := repository.NewUsageRepository(db)
	usageService := services.NewUsageService(usageRepository)

	claudeSessions := sessions.NewMemoryStore[string](
		2*time.Hour,
		1000,
	)

	ollamaSessions := sessions.NewMemoryStore[[]domain.Message](
		2*time.Hour,
		1000,
	)

	claudeProvider := claude.NewProvider(
		cfg,
		claudeSessions,
	)

	ollamaProvider := ollama.NewProvider(
		cfg,
		ollamaSessions,
	)

	chatService := services.NewChatService(
		cfg,
		claudeProvider,
		ollamaProvider,
		usageService,
	)

	router := bridgehttp.NewRouter(bridgehttp.RouterDeps{
		Config: cfg,

		ChatHandler:      handlers.NewChatHandler(chatService),
		ModelsHandler:    handlers.NewModelsHandler(),
		UsageHandler:     handlers.NewUsageHandler(usageService),
		HealthHandler:    handlers.NewHealthHandler(cfg),
		DashboardHandler: handlers.NewDashboardHandler(usageService),
	})

	address := cfg.Host + ":" + cfg.Port

	server := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("claude-bridge starting")
	log.Printf("address:       http://%s", address)
	log.Printf("dashboard:     http://%s/dashboard", address)
	log.Printf("health:        http://%s/health", address)
	log.Printf("usage db:      %s", cfg.UsageDBPath)
	log.Printf("ollama url:    %s", cfg.OllamaURL)
	log.Printf("auth enabled:  %v", cfg.LocalAuthKey != "")
	log.Printf("mcp config:    %q", cfg.MCPConfigPath)
	log.Printf("mcp always:    %v", cfg.MCPAlways)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
