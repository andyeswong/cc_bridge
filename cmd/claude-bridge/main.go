package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"claude-bridge/internal/config"
	bridgehttp "claude-bridge/internal/http"
	"claude-bridge/internal/http/handlers"
	"claude-bridge/internal/providers/claude"
	"claude-bridge/internal/providers/registry"
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

	claudeProvider := claude.NewProvider(cfg, claudeSessions)

	bridgeCfg, err := config.LoadBridgeConfig(cfg.ProvidersConfigPath)
	if err != nil {
		log.Printf("warning: failed to load providers config: %v", err)
	}

	// Auto-register legacy OLLAMA_URL as an "ollama" provider when no config file
	// is present, for backward compatibility.
	if bridgeCfg == nil && cfg.OllamaURL != "" {
		bridgeCfg = &config.BridgeConfig{
			Providers: []config.ProviderConfig{
				{
					Name:       "ollama",
					APIBaseURL: strings.TrimRight(cfg.OllamaURL, "/") + "/v1",
					APIKey:     "ollama",
				},
			},
		}
		log.Printf("ollama auto-registered from OLLAMA_URL=%s (migrate to CCB_CONFIG_PATH for full control)", cfg.OllamaURL)
	}

	reg := registry.New(bridgeCfg)

	chatService := services.NewChatService(cfg, claudeProvider, reg, usageService)

	router := bridgehttp.NewRouter(bridgehttp.RouterDeps{
		Config:           cfg,
		ChatHandler:      handlers.NewChatHandler(chatService),
		ModelsHandler:    handlers.NewModelsHandler(reg),
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
	log.Printf("providers:     %d registered", bridgeCfg.CountProviders())
	log.Printf("auth enabled:  %v", cfg.LocalAuthKey != "")
	log.Printf("mcp config:    %q", cfg.MCPConfigPath)
	log.Printf("mcp always:    %v", cfg.MCPAlways)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
