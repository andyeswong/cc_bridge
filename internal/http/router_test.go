package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"claude-bridge/internal/config"
	"claude-bridge/internal/domain"
	"claude-bridge/internal/http/handlers"
)

func TestHealth_Unprotected(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Host:         "127.0.0.1",
		Port:         "8080",
		OllamaURL:    "http://localhost:11434",
		UsageDBPath:  "usage.db",
		LocalAuthKey: "secret",
	}

	router := NewRouter(RouterDeps{
		Config:        cfg,
		HealthHandler: handlers.NewHealthHandler(cfg),
		ModelsHandler: handlers.NewModelsHandler(),
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field: %#v", body["status"])
	}
}

func TestModels_ReturnsOpenAIList(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	router := NewRouter(RouterDeps{
		Config:        cfg,
		HealthHandler: handlers.NewHealthHandler(cfg),
		ModelsHandler: handlers.NewModelsHandler(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp domain.ModelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Object != "list" {
		t.Fatalf("object: got %q", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected models")
	}
}

func TestLocalAuthMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authKey    string
		authHeader string
		wantStatus int
	}{
		{name: "NoAuthKeyPasses", authKey: "", authHeader: "", wantStatus: http.StatusOK},
		{name: "AuthKeyMissingHeader401", authKey: "k", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "AuthKeyValidBearerPasses", authKey: "k", authHeader: "Bearer k", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{LocalAuthKey: tt.authKey}
			router := NewRouter(RouterDeps{
				Config:        cfg,
				HealthHandler: handlers.NewHealthHandler(cfg),
				ModelsHandler: handlers.NewModelsHandler(),
			})

			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d want %d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
